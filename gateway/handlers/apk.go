package handlers

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gorilla/mux"

	"github.com/emulador/gateway/adb"
	"github.com/emulador/gateway/auth"
	"github.com/emulador/gateway/db"
)

type apkInfo struct {
	Filename    string `json:"filename"`
	SizeBytes   int64  `json:"size_bytes"`
	UploadedAt  string `json:"uploaded_at"`
	SHA256      string `json:"sha256,omitempty"`
	Exists      bool   `json:"exists"`
	HumanSize   string `json:"human_size"`
	PackageName string `json:"package_name,omitempty"`
	VersionName string `json:"version_name,omitempty"`
	VersionCode int32  `json:"version_code,omitempty"`
	AppLabel    string `json:"app_label,omitempty"`
	MinSDK      int32  `json:"min_sdk,omitempty"`
	TargetSDK   int32  `json:"target_sdk,omitempty"`
}

func humanSize(b int64) string {
	const u = 1024
	if b < u {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(u), 0
	for n := b / u; n >= u; n /= u {
		div *= u
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGT"[exp])
}

// GET /api/apk
func GetAPKInfoHandler(apkPath string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		st, err := os.Stat(apkPath)
		if err != nil {
			writeJSON(w, http.StatusOK, apkInfo{Exists: false, Filename: filepath.Base(apkPath)})
			return
		}
		info := apkInfo{
			Filename:   filepath.Base(apkPath),
			SizeBytes:  st.Size(),
			UploadedAt: st.ModTime().UTC().Format(time.RFC3339),
			HumanSize:  humanSize(st.Size()),
			Exists:     true,
		}
		// compute sha256 — small enough APKs (~70 MB) to do synchronously
		if f, err := os.Open(apkPath); err == nil {
			defer f.Close()
			h := sha256.New()
			if _, err := io.Copy(h, f); err == nil {
				info.SHA256 = hex.EncodeToString(h.Sum(nil))
			}
		}
		// parse manifest (best-effort)
		if meta, err := ParseAPKMeta(apkPath); err == nil {
			info.PackageName = meta.PackageName
			info.VersionName = meta.VersionName
			info.VersionCode = meta.VersionCode
			info.AppLabel = meta.AppLabel
			info.MinSDK = meta.MinSDK
			info.TargetSDK = meta.TargetSDK
		}
		writeJSON(w, http.StatusOK, info)
	}
}

// POST /api/apk (multipart, field name "apk")
// Replaces current APK in-place. 200 MB max.
func UploadAPKHandler(apkPath string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := auth.GetUserID(r.Context())
		if userID == "" {
			writeError(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		const maxBytes = 200 << 20
		r.Body = http.MaxBytesReader(w, r.Body, maxBytes)

		if err := r.ParseMultipartForm(32 << 20); err != nil {
			writeError(w, "invalid multipart form: "+err.Error(), http.StatusBadRequest)
			return
		}
		file, hdr, err := r.FormFile("apk")
		if err != nil {
			writeError(w, "missing 'apk' file field", http.StatusBadRequest)
			return
		}
		defer file.Close()

		if !strings.HasSuffix(strings.ToLower(hdr.Filename), ".apk") {
			writeError(w, "filename must end with .apk", http.StatusBadRequest)
			return
		}

		// quick magic check: APK is a ZIP, header "PK\x03\x04"
		head := make([]byte, 4)
		if _, err := file.Read(head); err != nil {
			writeError(w, "failed reading uploaded file", http.StatusBadRequest)
			return
		}
		if !(head[0] == 'P' && head[1] == 'K' && head[2] == 0x03 && head[3] == 0x04) {
			writeError(w, "uploaded file is not a valid APK (zip header missing)", http.StatusBadRequest)
			return
		}
		// rewind
		if _, err := file.Seek(0, 0); err != nil {
			writeError(w, "failed seeking uploaded file", http.StatusInternalServerError)
			return
		}

		// write to a tmp path then atomic rename
		dir := filepath.Dir(apkPath)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			writeError(w, "cannot create apk dir: "+err.Error(), http.StatusInternalServerError)
			return
		}
		tmp := apkPath + ".tmp"
		out, err := os.Create(tmp)
		if err != nil {
			writeError(w, "cannot open tmp file: "+err.Error(), http.StatusInternalServerError)
			return
		}
		written, err := io.Copy(out, file)
		out.Close()
		if err != nil {
			os.Remove(tmp)
			writeError(w, "write failed: "+err.Error(), http.StatusInternalServerError)
			return
		}
		if err := os.Rename(tmp, apkPath); err != nil {
			os.Remove(tmp)
			writeError(w, "rename failed: "+err.Error(), http.StatusInternalServerError)
			return
		}

		log.Printf("apk uploaded by user %s: %s (%d bytes, original=%s)", userID, apkPath, written, hdr.Filename)
		writeJSON(w, http.StatusOK, map[string]any{
			"ok":         true,
			"size_bytes": written,
			"human_size": humanSize(written),
			"original":   hdr.Filename,
		})
	}
}

// DELETE /api/apk
func DeleteAPKHandler(apkPath string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := auth.GetUserID(r.Context())
		if userID == "" {
			writeError(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if err := os.Remove(apkPath); err != nil && !os.IsNotExist(err) {
			writeError(w, "delete failed: "+err.Error(), http.StatusInternalServerError)
			return
		}
		log.Printf("apk deleted by user %s: %s", userID, apkPath)
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	}
}

// POST /api/devices/:id/update-app
// Reinstalls the current APK on a single running device (-r flag = replace).
func UpdateDeviceAppHandler(database *sql.DB, _ any, adbClient *adb.ADBClient, apkPath string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := auth.GetUserID(r.Context())
		if userID == "" {
			writeError(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		deviceID := mux.Vars(r)["id"]
		device, err := db.GetDeviceByID(database, deviceID)
		if err != nil || device == nil {
			writeError(w, "device not found", http.StatusNotFound)
			return
		}
		if device.UserID != userID {
			writeError(w, "forbidden", http.StatusForbidden)
			return
		}
		if device.Status != "ready" {
			writeError(w, "device must be 'ready' to update app (current: "+device.Status+")", http.StatusBadRequest)
			return
		}

		st, err := os.Stat(apkPath)
		if err != nil {
			writeError(w, "no APK uploaded yet — upload via /api/apk first", http.StatusPreconditionFailed)
			return
		}
		if st.Size() == 0 {
			writeError(w, "uploaded APK is empty", http.StatusPreconditionFailed)
			return
		}

		log.Printf("update-app requested for device %s by user %s", deviceID, userID)
		if err := adbClient.InstallAPK(device.ADBPort, apkPath); err != nil {
			log.Printf("update-app failed for device %s: %v", deviceID, err)
			writeError(w, "install failed: "+err.Error(), http.StatusInternalServerError)
			return
		}
		hash := FileSHA256(apkPath)
		if hash != "" {
			_ = db.UpdateDeviceAPKHash(database, deviceID, hash)
		}
		log.Printf("update-app succeeded for device %s (hash=%s)", deviceID, hash[:min(12, len(hash))])
		writeJSON(w, http.StatusOK, map[string]any{
			"ok":                 true,
			"installed":          true,
			"size_bytes":         st.Size(),
			"installed_apk_hash": hash,
		})
	}
}
