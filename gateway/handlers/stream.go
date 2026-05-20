package handlers

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"

	"github.com/emulador/gateway/adb"
	"github.com/emulador/gateway/auth"
	"github.com/emulador/gateway/db"
	"github.com/emulador/gateway/streaming"
)

func StreamHandler(database *sql.DB, streamMgr *streaming.StreamManager, adbClient *adb.ADBClient, jwtSecret string) http.HandlerFunc {
	upgrader := websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 65536,
		CheckOrigin: func(r *http.Request) bool {
			return true // allow all origins for dev
		},
	}

	return func(w http.ResponseWriter, r *http.Request) {
		deviceID := mux.Vars(r)["id"]

		// Validate JWT from query param (WebSocket can't send headers easily)
		tokenStr := r.URL.Query().Get("token")
		if tokenStr == "" {
			writeError(w, "missing token", http.StatusUnauthorized)
			return
		}

		userID, err := auth.ValidateToken(tokenStr, jwtSecret)
		if err != nil {
			writeError(w, "invalid or expired token", http.StatusUnauthorized)
			return
		}

		// Verify device belongs to user
		device, err := db.GetDeviceByID(database, deviceID)
		if err != nil {
			writeError(w, "device not found", http.StatusNotFound)
			return
		}

		if device.UserID != userID {
			writeError(w, "device not found", http.StatusNotFound)
			return
		}

		if device.Status != "ready" && device.Status != "booting" && device.Status != "installing" {
			writeError(w, "device is not running", http.StatusBadRequest)
			return
		}

		if device.ADBPort == 0 {
			writeError(w, "device has no ADB port assigned", http.StatusBadRequest)
			return
		}

		log.Printf("websocket stream requested for device %s by user %s", deviceID, userID)

		containerName := device.ContainerName
		if containerName == "" {
			containerName = fmt.Sprintf("emulador_redroid_%s", deviceID[:8])
		}
		handler := streaming.HandleStream(&upgrader, streamMgr, adbClient, device.ADBPort, deviceID, containerName)
		handler(w, r)
	}
}
