package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/docker/docker/client"
	"github.com/gorilla/mux"

	"github.com/emulador/gateway/adb"
	"github.com/emulador/gateway/auth"
	"github.com/emulador/gateway/db"
	dkr "github.com/emulador/gateway/docker"
	"github.com/emulador/gateway/models"
	"github.com/emulador/gateway/streaming"
)

type createDeviceRequest struct {
	Name           string `json:"name"`
	AndroidVersion string `json:"android_version"`
	APKID          string `json:"apk_id"` // optional; falls back to default
}

func ListDevicesHandler(database *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := auth.GetUserID(r.Context())
		if userID == "" {
			writeError(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		devices, err := db.GetDevicesByUserID(database, userID)
		if err != nil {
			log.Printf("error listing devices: %v", err)
			writeError(w, "failed to list devices", http.StatusInternalServerError)
			return
		}

		if devices == nil {
			devices = []models.Device{}
		}

		writeJSON(w, http.StatusOK, map[string]interface{}{
			"data": devices,
		})
	}
}

func CreateDeviceHandler(database *sql.DB, dockerCli *client.Client, adbClient *adb.ADBClient, whatsappAPK string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := auth.GetUserID(r.Context())
		if userID == "" {
			writeError(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		var req createDeviceRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, "invalid request body", http.StatusBadRequest)
			return
		}

		req.Name = strings.TrimSpace(req.Name)
		if req.Name == "" {
			writeError(w, "name is required", http.StatusBadRequest)
			return
		}

		if req.AndroidVersion == "" {
			req.AndroidVersion = "11"
		}

		// Resolve which APK to install: explicit apk_id wins, else current default,
		// else legacy single-file path.
		apkPathToInstall := whatsappAPK
		var resolvedAPKID string
		if req.APKID != "" {
			rec, err := db.GetAPKByID(database, req.APKID)
			if err != nil {
				writeError(w, "apk_id not found", http.StatusBadRequest)
				return
			}
			apkPathToInstall = APKFilePath(APKDir(whatsappAPK), rec)
			resolvedAPKID = rec.ID
		} else if def, err := db.GetDefaultAPK(database); err == nil && def != nil {
			apkPathToInstall = APKFilePath(APKDir(whatsappAPK), def)
			resolvedAPKID = def.ID
		}

		// Allocate ADB port
		adbPort, err := db.AllocateADBPort(database)
		if err != nil {
			log.Printf("error allocating ADB port: %v", err)
			writeError(w, "no available ports", http.StatusServiceUnavailable)
			return
		}

		// Create device record
		device, err := db.CreateDevice(database, userID, req.Name, req.AndroidVersion)
		if err != nil {
			log.Printf("error creating device record: %v", err)
			writeError(w, "failed to create device", http.StatusInternalServerError)
			return
		}

		// Create Docker container
		containerID, err := dkr.CreateRedroidContainer(dockerCli, device.ID, adbPort, 512*1024*1024)
		if err != nil {
			log.Printf("error creating container for device %s: %v", device.ID, err)
			db.UpdateDeviceStatus(database, device.ID, models.StatusError, err.Error())
			db.CreateDeviceLog(database, device.ID, "create_container", "failed: "+err.Error())
			writeError(w, "failed to create container", http.StatusInternalServerError)
			return
		}

		containerName := "emulador_redroid_" + device.ID[:8]
		if err := db.UpdateDeviceContainer(database, device.ID, containerID, containerName, adbPort); err != nil {
			log.Printf("error updating device container info: %v", err)
		}

		// Start container
		if err := dkr.StartContainer(dockerCli, containerID); err != nil {
			log.Printf("error starting container for device %s: %v", device.ID, err)
			db.UpdateDeviceStatus(database, device.ID, models.StatusError, err.Error())
			db.CreateDeviceLog(database, device.ID, "start_container", "failed: "+err.Error())
			writeError(w, "failed to start container", http.StatusInternalServerError)
			return
		}

		db.UpdateDeviceStatus(database, device.ID, models.StatusBooting, "")
		db.CreateDeviceLog(database, device.ID, "create", "container created and started")

		// Persist apk_id reference if resolved
		if resolvedAPKID != "" {
			_ = db.UpdateDeviceAPKID(database, device.ID, resolvedAPKID)
		}

		// Background provisioning goroutine
		go provisionDevice(database, adbClient, device.ID, adbPort, apkPathToInstall)

		// Refresh device from DB to get updated fields
		device, _ = db.GetDeviceByID(database, device.ID)

		writeJSON(w, http.StatusCreated, map[string]interface{}{
			"data": device,
		})
	}
}

func provisionDevice(database *sql.DB, adbClient *adb.ADBClient, deviceID string, adbPort int, whatsappAPK string) {
	log.Printf("starting provisioning for device %s on port %d", deviceID, adbPort)

	bootCtx, bootCancel := timeoutContext(5 * time.Minute)
	defer bootCancel()

	if err := adbClient.WaitForBoot(bootCtx, adbPort, 5*time.Minute); err != nil {
		log.Printf("device %s boot failed: %v", deviceID, err)
		db.UpdateDeviceStatus(database, deviceID, models.StatusError, "boot timeout: "+err.Error())
		db.CreateDeviceLog(database, deviceID, "boot", "failed: "+err.Error())
		return
	}

	db.CreateDeviceLog(database, deviceID, "boot", "device booted successfully")

	// Install WhatsApp if APK file exists
	if whatsappAPK != "" {
		if _, err := os.Stat(whatsappAPK); err == nil {
			db.UpdateDeviceStatus(database, deviceID, models.StatusInstalling, "")
			db.CreateDeviceLog(database, deviceID, "install", "installing WhatsApp APK")

			if err := adbClient.InstallAPK(adbPort, whatsappAPK); err != nil {
				log.Printf("device %s WhatsApp install failed (continuing): %v", deviceID, err)
				db.CreateDeviceLog(database, deviceID, "install", "WhatsApp install failed: "+err.Error())
			} else {
				db.CreateDeviceLog(database, deviceID, "install", "WhatsApp installed successfully")
				if hash := FileSHA256(whatsappAPK); hash != "" {
					_ = db.UpdateDeviceAPKHash(database, deviceID, hash)
				}
			}
		} else {
			log.Printf("device %s WhatsApp APK not found at %s, skipping install", deviceID, whatsappAPK)
			db.CreateDeviceLog(database, deviceID, "install", "WhatsApp APK not found, skipped")
		}
	}

	db.UpdateDeviceStatus(database, deviceID, models.StatusReady, "")
	db.CreateDeviceLog(database, deviceID, "ready", "device is ready")
	log.Printf("device %s provisioning complete", deviceID)
}

func timeoutContext(d time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), d)
}

func GetDeviceHandler(database *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := auth.GetUserID(r.Context())
		if userID == "" {
			writeError(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		deviceID := mux.Vars(r)["id"]
		device, err := db.GetDeviceByID(database, deviceID)
		if err != nil {
			writeError(w, "device not found", http.StatusNotFound)
			return
		}

		if device.UserID != userID {
			writeError(w, "device not found", http.StatusNotFound)
			return
		}

		writeJSON(w, http.StatusOK, map[string]interface{}{
			"data": device,
		})
	}
}

func UpdateDeviceHandler(database *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := auth.GetUserID(r.Context())
		if userID == "" {
			writeError(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		deviceID := mux.Vars(r)["id"]
		device, err := db.GetDeviceByID(database, deviceID)
		if err != nil || device.UserID != userID {
			writeError(w, "device not found", http.StatusNotFound)
			return
		}

		var body struct {
			Name string `json:"name"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, "invalid body", http.StatusBadRequest)
			return
		}
		body.Name = strings.TrimSpace(body.Name)
		if body.Name == "" {
			writeError(w, "name is required", http.StatusBadRequest)
			return
		}
		if len(body.Name) > 255 {
			writeError(w, "name too long (max 255)", http.StatusBadRequest)
			return
		}

		if err := db.UpdateDeviceName(database, deviceID, body.Name); err != nil {
			writeError(w, "update failed: "+err.Error(), http.StatusInternalServerError)
			return
		}
		updated, _ := db.GetDeviceByID(database, deviceID)
		writeJSON(w, http.StatusOK, map[string]any{"data": updated})
	}
}

func DeleteDeviceHandler(database *sql.DB, dockerCli *client.Client, streamMgr *streaming.StreamManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := auth.GetUserID(r.Context())
		if userID == "" {
			writeError(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		deviceID := mux.Vars(r)["id"]
		device, err := db.GetDeviceByID(database, deviceID)
		if err != nil {
			writeError(w, "device not found", http.StatusNotFound)
			return
		}

		if device.UserID != userID {
			writeError(w, "device not found", http.StatusNotFound)
			return
		}

		// Stop stream
		streamMgr.StopStream(deviceID)

		// Stop and remove container if it exists
		if device.ContainerID != "" {
			exists, _ := dkr.ContainerExists(dockerCli, device.ContainerID)
			if exists {
				if err := dkr.StopContainer(dockerCli, device.ContainerID); err != nil {
					log.Printf("error stopping container %s: %v", device.ContainerID, err)
				}
				if err := dkr.RemoveContainer(dockerCli, device.ContainerID); err != nil {
					log.Printf("error removing container %s: %v", device.ContainerID, err)
				}
			}
		}

		// Delete from DB
		if err := db.DeleteDevice(database, deviceID); err != nil {
			log.Printf("error deleting device %s: %v", deviceID, err)
			writeError(w, "failed to delete device", http.StatusInternalServerError)
			return
		}

		writeJSON(w, http.StatusOK, map[string]interface{}{
			"data": map[string]string{"message": "device deleted"},
		})
	}
}

func StartDeviceHandler(database *sql.DB, dockerCli *client.Client, adbClient *adb.ADBClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := auth.GetUserID(r.Context())
		if userID == "" {
			writeError(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		deviceID := mux.Vars(r)["id"]
		device, err := db.GetDeviceByID(database, deviceID)
		if err != nil {
			writeError(w, "device not found", http.StatusNotFound)
			return
		}

		if device.UserID != userID {
			writeError(w, "device not found", http.StatusNotFound)
			return
		}

		if device.ContainerID == "" {
			writeError(w, "device has no container", http.StatusBadRequest)
			return
		}

		if err := dkr.StartContainer(dockerCli, device.ContainerID); err != nil {
			log.Printf("error starting container %s: %v", device.ContainerID, err)
			writeError(w, "failed to start device", http.StatusInternalServerError)
			return
		}

		db.UpdateDeviceStatus(database, deviceID, models.StatusBooting, "")
		db.CreateDeviceLog(database, deviceID, "start", "container started")

		// Background: wait for Android boot and mark ready. Without this the
		// device stays in 'booting' forever after a stop→start cycle.
		go waitForBootAndMarkReady(database, adbClient, deviceID, device.ADBPort)

		writeJSON(w, http.StatusOK, map[string]interface{}{
			"data": map[string]string{"message": "device starting"},
		})
	}
}

// waitForBootAndMarkReady is the lite version of provisionDevice used after
// stop→start. It does NOT reinstall the APK (already there from first
// provisioning). Just waits for Android boot and flips status to ready.
func waitForBootAndMarkReady(database *sql.DB, adbClient *adb.ADBClient, deviceID string, adbPort int) {
	log.Printf("waitForBoot: device %s on port %d", deviceID, adbPort)
	bootCtx, bootCancel := timeoutContext(5 * time.Minute)
	defer bootCancel()

	if err := adbClient.WaitForBoot(bootCtx, adbPort, 5*time.Minute); err != nil {
		log.Printf("waitForBoot: device %s boot failed: %v", deviceID, err)
		db.UpdateDeviceStatus(database, deviceID, models.StatusError, "boot timeout: "+err.Error())
		db.CreateDeviceLog(database, deviceID, "start", "boot failed: "+err.Error())
		return
	}
	db.UpdateDeviceStatus(database, deviceID, models.StatusReady, "")
	db.CreateDeviceLog(database, deviceID, "start", "device ready after boot")
	log.Printf("waitForBoot: device %s ready", deviceID)
}

func StopDeviceHandler(database *sql.DB, dockerCli *client.Client, streamMgr *streaming.StreamManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := auth.GetUserID(r.Context())
		if userID == "" {
			writeError(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		deviceID := mux.Vars(r)["id"]
		device, err := db.GetDeviceByID(database, deviceID)
		if err != nil {
			writeError(w, "device not found", http.StatusNotFound)
			return
		}

		if device.UserID != userID {
			writeError(w, "device not found", http.StatusNotFound)
			return
		}

		if device.ContainerID == "" {
			writeError(w, "device has no container", http.StatusBadRequest)
			return
		}

		streamMgr.StopStream(deviceID)

		if err := dkr.StopContainer(dockerCli, device.ContainerID); err != nil {
			log.Printf("error stopping container %s: %v", device.ContainerID, err)
			writeError(w, "failed to stop device", http.StatusInternalServerError)
			return
		}

		db.UpdateDeviceStatus(database, deviceID, models.StatusStopped, "")
		db.CreateDeviceLog(database, deviceID, "stop", "container stopped")

		writeJSON(w, http.StatusOK, map[string]interface{}{
			"data": map[string]string{"message": "device stopped"},
		})
	}
}

func DeviceStatsHandler(database *sql.DB, dockerCli *client.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := auth.GetUserID(r.Context())
		if userID == "" {
			writeError(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		deviceID := mux.Vars(r)["id"]
		device, err := db.GetDeviceByID(database, deviceID)
		if err != nil {
			writeError(w, "device not found", http.StatusNotFound)
			return
		}

		if device.UserID != userID {
			writeError(w, "device not found", http.StatusNotFound)
			return
		}

		if device.ContainerID == "" {
			writeError(w, "device has no container", http.StatusBadRequest)
			return
		}

		stats, err := dkr.GetContainerStats(dockerCli, device.ContainerID)
		if err != nil {
			log.Printf("error getting stats for container %s: %v", device.ContainerID, err)
			writeError(w, "failed to get device stats", http.StatusInternalServerError)
			return
		}

		writeJSON(w, http.StatusOK, map[string]interface{}{
			"data": stats,
		})
	}
}
