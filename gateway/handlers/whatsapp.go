package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"

	"github.com/emulador/gateway/adb"
	"github.com/emulador/gateway/auth"
	"github.com/emulador/gateway/db"
	"github.com/emulador/gateway/models"
)

const waPackage = "com.whatsapp"

// resolveDevice loads the device, verifies ownership and returns it.
// On any failure it writes the error to the response and returns nil.
func resolveDevice(database *sql.DB, w http.ResponseWriter, r *http.Request) *models.Device {
	userID := auth.GetUserID(r.Context())
	if userID == "" {
		writeError(w, "unauthorized", http.StatusUnauthorized)
		return nil
	}
	deviceID := mux.Vars(r)["id"]
	device, err := db.GetDeviceByID(database, deviceID)
	if err != nil || device == nil || device.UserID != userID {
		writeError(w, "device not found", http.StatusNotFound)
		return nil
	}
	return device
}

// ─── POST /api/devices/{id}/whatsapp/open ────────────────────────────────
//
// Opens the WhatsApp app on the device and reports its current state.
type whatsappStatus struct {
	OK              bool   `json:"ok"`
	State           string `json:"state"` // ready | needs_registration | loading | banned | not_focused | unknown
	CurrentActivity string `json:"current_activity"`
	Version         string `json:"version,omitempty"`
	Package         string `json:"package"`
	Note            string `json:"note,omitempty"`
}

func WhatsAppOpenHandler(database *sql.DB, adbClient *adb.ADBClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		device := resolveDevice(database, w, r)
		if device == nil {
			return
		}
		if device.Status != "ready" {
			writeError(w, "device not ready (status: "+device.Status+")", http.StatusBadRequest)
			return
		}

		// 1. resolve the launcher activity dynamically — works across versions
		resolveOut, _ := adbClient.Exec(device.ADBPort, "shell",
			"cmd package resolve-activity --brief --components -c android.intent.category.LAUNCHER "+waPackage)
		launcherActivity := strings.TrimSpace(parseLauncherActivity(string(resolveOut)))
		if launcherActivity == "" {
			launcherActivity = waPackage + "/com.whatsapp.Main" // fallback
		}

		// 2. wake and unlock just in case
		_, _ = adbClient.Exec(device.ADBPort, "shell", "input keyevent KEYCODE_WAKEUP")
		_, _ = adbClient.Exec(device.ADBPort, "shell", "wm dismiss-keyguard")

		// 3. launch via am start (deterministic)
		startOut, _ := adbClient.Exec(device.ADBPort, "shell",
			"am start -W -n "+launcherActivity)
		log.Printf("whatsapp/open device=%s launcher=%s start_output=%q",
			device.ID, launcherActivity, truncate(string(startOut), 200))

		// 4. wait for app to settle by checking ResumedActivity (no shell pipes)
		ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
		defer cancel()
		var focus string
		for {
			activities, _ := adbClient.Exec(device.ADBPort, "shell", "dumpsys activity activities")
			focus = findResumedActivity(string(activities))
			if strings.Contains(focus, waPackage) {
				break
			}
			select {
			case <-ctx.Done():
				goto done
			case <-time.After(500 * time.Millisecond):
			}
		}
	done:

		st := whatsappStatus{Package: waPackage, CurrentActivity: focus}
		if st.CurrentActivity == "" {
			st.CurrentActivity = extractCurrentActivity(focus)
		}
		st.State = classifyWAActivity(st.CurrentActivity, focus)
		st.OK = st.State == "ready"

		// 3. fetch installed version (parse client-side to avoid shell pipes)
		dumpBytes, _ := adbClient.Exec(device.ADBPort, "shell", "dumpsys", "package", waPackage)
		for _, line := range strings.Split(string(dumpBytes), "\n") {
			if i := strings.Index(line, "versionName="); i >= 0 {
				v := strings.TrimSpace(line[i+len("versionName="):])
				if sp := strings.IndexAny(v, " \r\n"); sp > 0 {
					v = v[:sp]
				}
				st.Version = v
				break
			}
		}

		switch st.State {
		case "ready":
			st.Note = "WhatsApp aberto e pronto"
		case "needs_registration":
			st.Note = "Conta ainda não registrada — terminar cadastro pelo painel"
		case "loading":
			st.Note = "WhatsApp ainda está carregando — tente novamente em alguns segundos"
		case "banned":
			st.Note = "Conta possivelmente banida — tela de recurso detectada"
		case "not_focused":
			st.Note = "WhatsApp não está em foco mesmo após launch"
		}

		log.Printf("whatsapp/open device=%s state=%s activity=%s", device.ID, st.State, st.CurrentActivity)
		writeJSON(w, http.StatusOK, st)
	}
}

// parseLauncherActivity reads the output of "cmd package resolve-activity" and
// returns the "package/Activity" string of the LAUNCHER component, or "" if not
// found. Output format is several lines; the activity component is on a line
// like "com.whatsapp/com.whatsapp.Main".
func parseLauncherActivity(out string) string {
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if strings.Contains(line, "/") && strings.Contains(line, ".") &&
			!strings.Contains(line, "name=") && !strings.Contains(line, "priority") {
			// looks like a component
			if strings.Count(line, " ") == 0 {
				return line
			}
		}
	}
	return ""
}

// findResumedActivity scans "dumpsys activity activities" output for the
// currently resumed activity component (e.g. "com.whatsapp/.HomeActivity").
func findResumedActivity(out string) string {
	re := regexp.MustCompile(`(?m)(?:Resumed|mResumed)Activity[^{]*\{[^ ]+ [^ ]+ ([A-Za-z0-9._]+/[A-Za-z0-9._]+)`)
	if m := re.FindStringSubmatch(out); len(m) >= 2 {
		return m[1]
	}
	return ""
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

func extractCurrentActivity(dump string) string {
	// match patterns like "Window{... com.whatsapp/com.whatsapp.Main}"
	// or "ActivityRecord{... com.whatsapp/.HomeActivity ..."
	re := regexp.MustCompile(`(com\.whatsapp[/.][A-Za-z0-9._]+)`)
	if m := re.FindString(dump); m != "" {
		return m
	}
	return ""
}

func classifyWAActivity(activity, dump string) string {
	a := strings.ToLower(activity)
	switch {
	case a == "":
		if strings.Contains(dump, waPackage) {
			return "loading"
		}
		return "not_focused"
	case strings.Contains(a, "main"), strings.Contains(a, "home"), strings.Contains(a, "convo"), strings.Contains(a, "chats"):
		return "ready"
	case strings.Contains(a, "register"), strings.Contains(a, "verify"), strings.Contains(a, "phonenumberentry"), strings.Contains(a, "smscode"):
		return "needs_registration"
	case strings.Contains(a, "ban"), strings.Contains(a, "appeals"):
		return "banned"
	case strings.Contains(a, "splash"), strings.Contains(a, "launcher"):
		return "loading"
	default:
		return "unknown"
	}
}

// ─── POST /api/devices/{id}/whatsapp/send ────────────────────────────────
//
// Body: {"phone": "5511999998888", "message": "oi"}
// Opens the chat (works for saved and unsaved contacts via wa.me deeplink),
// waits for it to load, then taps the Send button.
type sendMessageRequest struct {
	Phone   string `json:"phone"`
	Message string `json:"message"`
}

type sendMessageResponse struct {
	OK              bool   `json:"ok"`
	Phone           string `json:"phone"`
	Message         string `json:"message"`
	ChatLoadedAfter string `json:"chat_loaded_after,omitempty"`
	TappedAt        *xy    `json:"tapped_at,omitempty"`
	Note            string `json:"note,omitempty"`
}

type xy struct{ X, Y int }

func WhatsAppSendMessageHandler(database *sql.DB, adbClient *adb.ADBClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		device := resolveDevice(database, w, r)
		if device == nil {
			return
		}
		if device.Status != "ready" {
			writeError(w, "device not ready (status: "+device.Status+")", http.StatusBadRequest)
			return
		}

		var body sendMessageRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, "invalid body", http.StatusBadRequest)
			return
		}
		digits := regexp.MustCompile(`\D`).ReplaceAllString(body.Phone, "")
		if len(digits) < 8 {
			writeError(w, "invalid phone (digits only, min 8)", http.StatusBadRequest)
			return
		}
		if strings.TrimSpace(body.Message) == "" {
			writeError(w, "message is required", http.StatusBadRequest)
			return
		}

		log.Printf("whatsapp/send device=%s phone=%s len(msg)=%d", device.ID, digits, len(body.Message))
		res := InternalSendWhatsApp(r.Context(), adbClient, device.ADBPort, digits, body.Message)
		resp := sendMessageResponse{
			OK:              res.OK,
			Phone:           res.Phone,
			Message:         body.Message,
			ChatLoadedAfter: res.ChatLoadedAfter.Round(100 * time.Millisecond).String(),
			TappedAt:        res.TappedAt,
		}
		if res.OK {
			resp.Note = "mensagem enviada"
		} else {
			resp.Note = res.ErrMsg
		}
		writeJSON(w, http.StatusOK, resp)
	}
}

// SendResult is what InternalSendWhatsApp returns. Used by the campaign
// dispatcher to record per-message outcome without going through HTTP.
type SendResult struct {
	OK              bool
	Phone           string
	ChatLoadedAfter time.Duration
	TappedAt        *xy
	ErrMsg          string
}

// InternalSendWhatsApp opens a chat with the given phone (saved or unsaved
// contact via wa.me deeplink), waits for the chat UI, and taps the send
// button. Reused by both the HTTP handler and the campaign dispatcher.
func InternalSendWhatsApp(ctx context.Context, adbClient *adb.ADBClient, port int, phone, message string) SendResult {
	digits := regexp.MustCompile(`\D`).ReplaceAllString(phone, "")
	if len(digits) < 8 {
		return SendResult{OK: false, Phone: digits, ErrMsg: "invalid phone"}
	}
	if strings.TrimSpace(message) == "" {
		return SendResult{OK: false, Phone: digits, ErrMsg: "empty message"}
	}
	deeplink := fmt.Sprintf("https://wa.me/%s?text=%s", digits, url.QueryEscape(message))
	// -p com.whatsapp force-routes the intent to WhatsApp (otherwise Android
	// shows a ResolverActivity "open with…" chooser since there's no default
	// browser/app set for wa.me URLs in fresh containers).
	// FLAG_ACTIVITY_NEW_TASK (0x10000000) | FLAG_ACTIVITY_CLEAR_TOP (0x04000000) = 0x14000000
	startCmd := fmt.Sprintf(
		"am start -a android.intent.action.VIEW -d '%s' -p com.whatsapp -f 0x14000000",
		strings.ReplaceAll(deeplink, "'", `'\''`),
	)
	if out, err := adbClient.Exec(port, "shell", startCmd); err != nil {
		return SendResult{OK: false, Phone: digits, ErrMsg: "intent: " + err.Error() + " " + string(out)}
	}

	// Wait for chat to load (15s budget) — detect both success states and
	// dead-ends (device not registered, account banned).
	waitCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	start := time.Now()
	chatReady := false
	var lastFocus string
	for {
		select {
		case <-waitCtx.Done():
			return SendResult{OK: false, Phone: digits, ErrMsg: "chat não carregou em 15s · última activity: " + lastFocus}
		case <-time.After(700 * time.Millisecond):
		}
		activities, _ := adbClient.Exec(port, "shell", "dumpsys activity activities")
		focus := findResumedActivity(string(activities))
		lastFocus = focus
		lower := strings.ToLower(focus)

		// Dead-end states — fail fast with a clear message.
		if strings.Contains(lower, "eula") ||
			strings.Contains(lower, "registration") ||
			strings.Contains(lower, "verifyphonenumber") ||
			strings.Contains(lower, "phonenumberentry") ||
			strings.Contains(lower, "smscode") {
			return SendResult{OK: false, Phone: digits, ErrMsg: "WhatsApp neste device ainda não está logado · faça o cadastro pelo painel antes de disparar"}
		}
		if strings.Contains(lower, "ban") || strings.Contains(lower, "appeals") {
			return SendResult{OK: false, Phone: digits, ErrMsg: "conta possivelmente banida"}
		}

		// Success states: chat opened directly (logged-in flow).
		if strings.Contains(lower, "conversation") ||
			strings.Contains(lower, "chatactivity") ||
			strings.Contains(lower, "viewmessage") {
			chatReady = true
			break
		}
		// TextAndDirectChatDeepLink is transient — keep polling
	}
	if !chatReady {
		return SendResult{OK: false, Phone: digits, ErrMsg: "chat did not load"}
	}
	loadedAfter := time.Since(start)

	time.Sleep(700 * time.Millisecond)
	if _, err := adbClient.Exec(port, "shell", "uiautomator dump /sdcard/_ui.xml"); err != nil {
		return SendResult{OK: false, Phone: digits, ChatLoadedAfter: loadedAfter, ErrMsg: "uiautomator dump: " + err.Error()}
	}
	xmlBytes, err := adbClient.Exec(port, "shell", "cat /sdcard/_ui.xml")
	if err != nil {
		return SendResult{OK: false, Phone: digits, ChatLoadedAfter: loadedAfter, ErrMsg: "ui xml: " + err.Error()}
	}
	x, y, ok := findSendButton(string(xmlBytes))
	if !ok {
		// detect "not on whatsapp" popup
		if isNotOnWhatsAppPopup(string(xmlBytes)) {
			// dismiss and return clear error
			_, _ = adbClient.Exec(port, "shell", "input keyevent KEYCODE_BACK")
			return SendResult{OK: false, Phone: digits, ChatLoadedAfter: loadedAfter, ErrMsg: "número não está no WhatsApp"}
		}
		return SendResult{OK: false, Phone: digits, ChatLoadedAfter: loadedAfter, ErrMsg: "send button not found"}
	}
	if err := adbClient.Tap(port, x, y); err != nil {
		return SendResult{OK: false, Phone: digits, ChatLoadedAfter: loadedAfter, ErrMsg: "tap: " + err.Error()}
	}
	// small grace so the message has time to leave the queue
	time.Sleep(500 * time.Millisecond)
	return SendResult{OK: true, Phone: digits, ChatLoadedAfter: loadedAfter, TappedAt: &xy{X: x, Y: y}}
}

func isNotOnWhatsAppPopup(xmlStr string) bool {
	low := strings.ToLower(xmlStr)
	return (strings.Contains(low, "not on whatsapp") ||
		strings.Contains(low, "não está no whatsapp") ||
		strings.Contains(low, "no whatsapp"))
}

// findSendButton parses uiautomator dump XML and returns the centre of the WA
// send button. Tries multiple selectors because the resource-id and content
// description vary between WA versions and locales.
func findSendButton(xmlStr string) (int, int, bool) {
	// All shapes look like:  resource-id="X" ... bounds="[L,T][R,B]"
	// We extract the *bounds* of any node that matches one of the selectors.
	candidates := []*regexp.Regexp{
		regexp.MustCompile(`<node[^>]*resource-id="com\.whatsapp:id/send"[^>]*bounds="\[(\d+),(\d+)\]\[(\d+),(\d+)\]"`),
		regexp.MustCompile(`<node[^>]*content-desc="Send"[^>]*bounds="\[(\d+),(\d+)\]\[(\d+),(\d+)\]"`),
		regexp.MustCompile(`<node[^>]*content-desc="Enviar"[^>]*bounds="\[(\d+),(\d+)\]\[(\d+),(\d+)\]"`),
		regexp.MustCompile(`<node[^>]*bounds="\[(\d+),(\d+)\]\[(\d+),(\d+)\]"[^>]*resource-id="com\.whatsapp:id/send"`),
		regexp.MustCompile(`<node[^>]*bounds="\[(\d+),(\d+)\]\[(\d+),(\d+)\]"[^>]*content-desc="Send"`),
		regexp.MustCompile(`<node[^>]*bounds="\[(\d+),(\d+)\]\[(\d+),(\d+)\]"[^>]*content-desc="Enviar"`),
	}
	for _, re := range candidates {
		m := re.FindStringSubmatch(xmlStr)
		if len(m) == 5 {
			x1, _ := strconv.Atoi(m[1])
			y1, _ := strconv.Atoi(m[2])
			x2, _ := strconv.Atoi(m[3])
			y2, _ := strconv.Atoi(m[4])
			return (x1 + x2) / 2, (y1 + y2) / 2, true
		}
	}
	return 0, 0, false
}
