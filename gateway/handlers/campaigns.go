package handlers

import (
	"bufio"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/gorilla/mux"

	"github.com/emulador/gateway/auth"
	"github.com/emulador/gateway/db"
	"github.com/emulador/gateway/models"
)

// dispatcherIface is the small surface the campaign handlers need from the
// dispatcher — it is provided by the dispatch package but typed here as an
// interface so we don't need to import the dispatch package (and risk an
// import cycle, since dispatch imports handlers).
type dispatcherIface interface {
	Stop(campaignID string)
}

// ─── POST /api/campaigns ─────────────────────────────────────────────────
//
// Multipart body:
//
//	csv:        the file (column "phone" required, "name" optional)
//	config:     JSON string with name, message, delay_min, delay_max, device_ids[]
//
// Returns the created campaign in 'draft' status. Use POST /campaigns/:id/start
// to actually fire the dispatcher.
func CreateCampaignHandler(database *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := auth.GetUserID(r.Context())
		if userID == "" {
			writeError(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		const maxBytes = 10 << 20
		r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
		if err := r.ParseMultipartForm(2 << 20); err != nil {
			writeError(w, "invalid multipart: "+err.Error(), http.StatusBadRequest)
			return
		}

		// 1. parse config JSON
		cfgStr := r.FormValue("config")
		if cfgStr == "" {
			writeError(w, "missing 'config' field", http.StatusBadRequest)
			return
		}
		var cfg struct {
			Name      string   `json:"name"`
			Message   string   `json:"message"`
			DelayMin  int      `json:"delay_min_seconds"`
			DelayMax  int      `json:"delay_max_seconds"`
			DeviceIDs []string `json:"device_ids"`
		}
		if err := json.Unmarshal([]byte(cfgStr), &cfg); err != nil {
			writeError(w, "invalid config json: "+err.Error(), http.StatusBadRequest)
			return
		}
		cfg.Name = strings.TrimSpace(cfg.Name)
		cfg.Message = strings.TrimSpace(cfg.Message)
		if cfg.Name == "" {
			cfg.Name = "Campanha sem nome"
		}
		if cfg.Message == "" {
			writeError(w, "message is required", http.StatusBadRequest)
			return
		}
		if cfg.DelayMin < 1 {
			cfg.DelayMin = 30
		}
		if cfg.DelayMax < cfg.DelayMin {
			cfg.DelayMax = cfg.DelayMin
		}
		if len(cfg.DeviceIDs) == 0 {
			writeError(w, "select at least one device", http.StatusBadRequest)
			return
		}

		// 2. parse CSV file
		file, _, err := r.FormFile("csv")
		if err != nil {
			writeError(w, "missing 'csv' file: "+err.Error(), http.StatusBadRequest)
			return
		}
		defer file.Close()
		rows, err := parseCSV(file)
		if err != nil {
			writeError(w, "csv parse: "+err.Error(), http.StatusBadRequest)
			return
		}
		if len(rows) == 0 {
			writeError(w, "no valid phone numbers in CSV", http.StatusBadRequest)
			return
		}

		// 3. insert campaign + rows
		c := &models.Campaign{
			UserID:          userID,
			Name:            cfg.Name,
			Message:         cfg.Message,
			DelayMinSeconds: cfg.DelayMin,
			DelayMaxSeconds: cfg.DelayMax,
			DeviceIDs:       cfg.DeviceIDs,
			Status:          models.CampaignDraft,
			Total:           len(rows),
		}
		created, err := db.CreateCampaign(database, c)
		if err != nil {
			writeError(w, "create campaign: "+err.Error(), http.StatusInternalServerError)
			return
		}
		if err := db.InsertCampaignRows(database, created.ID, rows); err != nil {
			writeError(w, "insert rows: "+err.Error(), http.StatusInternalServerError)
			return
		}
		log.Printf("campaign created: id=%s rows=%d devices=%d user=%s",
			created.ID, len(rows), len(cfg.DeviceIDs), userID)
		writeJSON(w, http.StatusCreated, map[string]any{"data": created})
	}
}

func ListCampaignsHandler(database *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := auth.GetUserID(r.Context())
		if userID == "" {
			writeError(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		out, err := db.ListCampaignsByUser(database, userID)
		if err != nil {
			writeError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if out == nil {
			out = []models.Campaign{}
		}
		writeJSON(w, http.StatusOK, map[string]any{"data": out})
	}
}

func GetCampaignHandler(database *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := auth.GetUserID(r.Context())
		if userID == "" {
			writeError(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		c, err := db.GetCampaign(database, mux.Vars(r)["id"])
		if err != nil || c == nil || c.UserID != userID {
			writeError(w, "campaign not found", http.StatusNotFound)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"data": c})
	}
}

func ListCampaignRowsHandler(database *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := auth.GetUserID(r.Context())
		if userID == "" {
			writeError(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		id := mux.Vars(r)["id"]
		c, err := db.GetCampaign(database, id)
		if err != nil || c == nil || c.UserID != userID {
			writeError(w, "campaign not found", http.StatusNotFound)
			return
		}
		status := r.URL.Query().Get("status")
		limit := 200
		offset := 0
		if v, _ := strconv.Atoi(r.URL.Query().Get("limit")); v > 0 && v <= 1000 {
			limit = v
		}
		if v, _ := strconv.Atoi(r.URL.Query().Get("offset")); v > 0 {
			offset = v
		}
		rows, err := db.ListCampaignRows(database, id, status, limit, offset)
		if err != nil {
			writeError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if rows == nil {
			rows = []models.CampaignRow{}
		}
		writeJSON(w, http.StatusOK, map[string]any{"data": rows})
	}
}

func StartCampaignHandler(database *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := auth.GetUserID(r.Context())
		if userID == "" {
			writeError(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		id := mux.Vars(r)["id"]
		c, err := db.GetCampaign(database, id)
		if err != nil || c == nil || c.UserID != userID {
			writeError(w, "campaign not found", http.StatusNotFound)
			return
		}
		if c.Status != models.CampaignDraft && c.Status != models.CampaignPaused {
			writeError(w, "cannot start (current status: "+c.Status+")", http.StatusBadRequest)
			return
		}
		if err := db.UpdateCampaignStatus(database, id, models.CampaignRunning); err != nil {
			writeError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		// dispatcher's polling loop will pick this up within 5s
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	}
}

func PauseCampaignHandler(database *sql.DB, dis dispatcherIface) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := auth.GetUserID(r.Context())
		if userID == "" {
			writeError(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		id := mux.Vars(r)["id"]
		c, err := db.GetCampaign(database, id)
		if err != nil || c == nil || c.UserID != userID {
			writeError(w, "campaign not found", http.StatusNotFound)
			return
		}
		if err := db.UpdateCampaignStatus(database, id, models.CampaignPaused); err != nil {
			writeError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		dis.Stop(id)
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	}
}

func DeleteCampaignHandler(database *sql.DB, dis dispatcherIface) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := auth.GetUserID(r.Context())
		if userID == "" {
			writeError(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		id := mux.Vars(r)["id"]
		c, err := db.GetCampaign(database, id)
		if err != nil || c == nil || c.UserID != userID {
			writeError(w, "campaign not found", http.StatusNotFound)
			return
		}
		dis.Stop(id)
		if err := db.DeleteCampaign(database, id); err != nil {
			writeError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	}
}

// ─── csv parsing ─────────────────────────────────────────────────────────

// parseCSV accepts either:
//
//	a) a CSV with a header containing "phone" (and optionally "name"); or
//	b) one phone number per line (no header).
//
// Returns rows ready for InsertCampaignRows.
func parseCSV(rd io.Reader) ([]models.CampaignRow, error) {
	// peek so we can decide: if first line contains a digit-rich token and
	// no comma/header words, treat as plain list.
	br := bufio.NewReader(rd)
	first, _ := br.Peek(4096)
	hasHeader := looksLikeHeader(string(first))

	var out []models.CampaignRow
	if hasHeader {
		r := csv.NewReader(br)
		r.FieldsPerRecord = -1 // tolerate ragged
		records, err := r.ReadAll()
		if err != nil {
			return nil, err
		}
		if len(records) < 1 {
			return nil, nil
		}
		header := records[0]
		phoneIdx := -1
		nameIdx := -1
		for i, h := range header {
			h = strings.ToLower(strings.TrimSpace(h))
			switch h {
			case "phone", "telefone", "numero", "número", "celular":
				phoneIdx = i
			case "name", "nome":
				nameIdx = i
			}
		}
		if phoneIdx == -1 {
			phoneIdx = 0 // first column fallback
		}
		for _, rec := range records[1:] {
			if phoneIdx >= len(rec) {
				continue
			}
			phone := digitsOnly(rec[phoneIdx])
			if len(phone) < 8 {
				continue
			}
			row := models.CampaignRow{Phone: phone}
			if nameIdx >= 0 && nameIdx < len(rec) {
				row.Name = strings.TrimSpace(rec[nameIdx])
			}
			out = append(out, row)
		}
	} else {
		// plain-list mode: one phone per line
		s := bufio.NewScanner(br)
		s.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for s.Scan() {
			line := strings.TrimSpace(s.Text())
			if line == "" {
				continue
			}
			phone := digitsOnly(line)
			if len(phone) < 8 {
				continue
			}
			out = append(out, models.CampaignRow{Phone: phone})
		}
		if err := s.Err(); err != nil {
			return nil, err
		}
	}
	return out, nil
}

var digitsRe = regexp.MustCompile(`\D`)

func digitsOnly(s string) string { return digitsRe.ReplaceAllString(s, "") }

func looksLikeHeader(snippet string) bool {
	// crude heuristic — any of the header keywords in the first line?
	if i := strings.IndexByte(snippet, '\n'); i > 0 {
		snippet = snippet[:i]
	}
	low := strings.ToLower(snippet)
	for _, kw := range []string{"phone", "telefone", "numero", "número", "celular", "name", "nome"} {
		if strings.Contains(low, kw) {
			return true
		}
	}
	return false
}
