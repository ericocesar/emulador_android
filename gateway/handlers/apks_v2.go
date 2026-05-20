package handlers

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gorilla/mux"

	"github.com/emulador/gateway/auth"
	"github.com/emulador/gateway/db"
)

// ─── Multi-version APK endpoints ──────────────────────────────────────────

type apkListItem struct {
	ID          string `json:"id"`
	Filename    string `json:"filename"`
	PackageName string `json:"package_name"`
	VersionName string `json:"version_name"`
	VersionCode int64  `json:"version_code"`
	AppLabel    string `json:"app_label"`
	SizeBytes   int64  `json:"size_bytes"`
	HumanSize   string `json:"human_size"`
	SHA256      string `json:"sha256"`
	MinSDK      int    `json:"min_sdk"`
	TargetSDK   int    `json:"target_sdk"`
	IsDefault   bool   `json:"is_default"`
	Source      string `json:"source"`
	CreatedAt   string `json:"created_at"`
	DevicesLinked int  `json:"devices_linked"`
}

// GET /api/apks
func ListAPKsHandler(database *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := auth.GetUserID(r.Context())
		if userID == "" {
			writeError(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		apks, err := db.ListAPKs(database)
		if err != nil {
			writeError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		out := make([]apkListItem, 0, len(apks))
		for _, a := range apks {
			cnt, _ := db.CountDevicesByAPK(database, a.ID)
			out = append(out, apkListItem{
				ID:            a.ID,
				Filename:      a.Filename,
				PackageName:   a.PackageName,
				VersionName:   a.VersionName,
				VersionCode:   a.VersionCode,
				AppLabel:      a.AppLabel,
				SizeBytes:     a.SizeBytes,
				HumanSize:     humanSize(a.SizeBytes),
				SHA256:        a.SHA256,
				MinSDK:        a.MinSDK,
				TargetSDK:     a.TargetSDK,
				IsDefault:     a.IsDefault,
				Source:        a.Source,
				CreatedAt:     a.CreatedAt.UTC().Format(time.RFC3339),
				DevicesLinked: cnt,
			})
		}
		writeJSON(w, http.StatusOK, map[string]any{"data": out})
	}
}

// POST /api/apks  (multipart upload of one apk)
func UploadAPKv2Handler(database *sql.DB, legacyAPKPath string) http.HandlerFunc {
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
		head := make([]byte, 4)
		if _, err := file.Read(head); err != nil || !(head[0] == 'P' && head[1] == 'K' && head[2] == 0x03 && head[3] == 0x04) {
			writeError(w, "uploaded file is not a valid APK (zip header missing)", http.StatusBadRequest)
			return
		}
		if _, err := file.Seek(0, 0); err != nil {
			writeError(w, "seek failed", http.StatusInternalServerError)
			return
		}

		dir := APKDir(legacyAPKPath)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			writeError(w, "cannot create apk dir: "+err.Error(), http.StatusInternalServerError)
			return
		}
		tmp := filepath.Join(dir, ".upload-"+fmt.Sprintf("%d", time.Now().UnixNano())+".tmp")
		out, err := os.Create(tmp)
		if err != nil {
			writeError(w, "cannot open tmp file: "+err.Error(), http.StatusInternalServerError)
			return
		}
		if _, err := io.Copy(out, file); err != nil {
			out.Close()
			os.Remove(tmp)
			writeError(w, "write failed: "+err.Error(), http.StatusInternalServerError)
			return
		}
		out.Close()

		// Check if there's already a default. If not, this becomes default.
		hasDefault := false
		if def, _ := db.GetDefaultAPK(database); def != nil {
			hasDefault = true
		}
		rec, err := IngestAPKFile(database, tmp, "upload", !hasDefault)
		if err != nil {
			os.Remove(tmp)
			writeError(w, "ingest failed: "+err.Error(), http.StatusInternalServerError)
			return
		}
		log.Printf("apk uploaded by user %s: id=%s version=%s default=%t", userID, rec.ID, rec.VersionName, rec.IsDefault)
		writeJSON(w, http.StatusCreated, map[string]any{
			"data": rec,
		})
	}
}

// PATCH /api/apks/{id}/default  (set as default for new devices)
func SetDefaultAPKHandler(database *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := auth.GetUserID(r.Context())
		if userID == "" {
			writeError(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		id := mux.Vars(r)["id"]
		if _, err := db.GetAPKByID(database, id); err != nil {
			writeError(w, "apk not found", http.StatusNotFound)
			return
		}
		if err := db.SetDefaultAPK(database, id); err != nil {
			writeError(w, "set-default failed: "+err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "id": id})
	}
}

// ─── Check-update against whatsapp.com ────────────────────────────────────

// CheckWhatsAppUpdate downloads the official APK, parses it, and compares
// versionCode with what we have in DB. If newer, ingests it (NON-default).
// If equal/older, deletes the temp file and reports no-op.
func CheckWhatsAppUpdate(database *sql.DB, legacyAPKPath string) (newAPKID, msg string, err error) {
	const url = "https://www.whatsapp.com/android/current/WhatsApp.apk"
	dir := APKDir(legacyAPKPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", "", fmt.Errorf("mkdir: %w", err)
	}
	tmp := filepath.Join(dir, ".check-"+fmt.Sprintf("%d", time.Now().UnixNano())+".tmp")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Linux; Android 11) AstraDroid/1.0")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("http fetch: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", "", fmt.Errorf("unexpected status %d from whatsapp.com", resp.StatusCode)
	}
	out, err := os.Create(tmp)
	if err != nil {
		return "", "", err
	}
	if _, err := io.Copy(out, resp.Body); err != nil {
		out.Close()
		os.Remove(tmp)
		return "", "", fmt.Errorf("download: %w", err)
	}
	out.Close()

	// Parse the downloaded file
	meta, err := ParseAPKMeta(tmp)
	if err != nil {
		os.Remove(tmp)
		return "", "", fmt.Errorf("parse downloaded apk: %w", err)
	}
	// Compare against latest in DB
	latest, _ := db.GetLatestAPK(database)
	if latest != nil && int64(meta.VersionCode) <= latest.VersionCode {
		os.Remove(tmp)
		return "", fmt.Sprintf("nenhuma versão nova · whatsapp.com tem %s, já temos %s", meta.VersionName, latest.VersionName), nil
	}
	// Ingest. Don't auto-mark as default — user opt-in.
	rec, err := IngestAPKFile(database, tmp, "auto-download", false)
	if err != nil {
		// IngestAPKFile may have removed the tmp on duplicate; check anyway
		os.Remove(tmp)
		// duplicate hash -> treat as no-op
		if errors.Is(err, sql.ErrNoRows) || strings.Contains(err.Error(), "duplicate") {
			return "", fmt.Sprintf("já temos esse arquivo · v%s", meta.VersionName), nil
		}
		return "", "", err
	}
	prevVersion := ""
	if latest != nil {
		prevVersion = latest.VersionName
	}
	return rec.ID, fmt.Sprintf("nova versão baixada · v%s (anterior: v%s)", rec.VersionName, prevVersion), nil
}

// POST /api/apks/check-update
type checkUpdateResponse struct {
	OK         bool   `json:"ok"`
	Message    string `json:"message"`
	NewAPKID   string `json:"new_apk_id,omitempty"`
}

func CheckUpdateHandler(database *sql.DB, legacyAPKPath string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := auth.GetUserID(r.Context())
		if userID == "" {
			writeError(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		id, msg, err := CheckWhatsAppUpdate(database, legacyAPKPath)
		if err != nil {
			writeJSON(w, http.StatusBadGateway, checkUpdateResponse{OK: false, Message: err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, checkUpdateResponse{
			OK:       true,
			Message:  msg,
			NewAPKID: id,
		})
	}
}

// StartDailyAPKChecker runs CheckWhatsAppUpdate once per day in background.
// Logs the outcome but never crashes the gateway.
func StartDailyAPKChecker(database *sql.DB, legacyAPKPath string) {
	go func() {
		// initial 30s grace so the gateway is fully ready before first hit
		time.Sleep(30 * time.Second)
		for {
			id, msg, err := CheckWhatsAppUpdate(database, legacyAPKPath)
			if err != nil {
				log.Printf("daily-apk-check error: %v", err)
			} else {
				log.Printf("daily-apk-check: %s (new_id=%s)", msg, id)
			}
			time.Sleep(24 * time.Hour)
		}
	}()
}