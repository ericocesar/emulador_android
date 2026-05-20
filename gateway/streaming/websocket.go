package streaming

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/emulador/gateway/adb"
	"github.com/gorilla/websocket"
)

type InputMessage struct {
	Type     string `json:"type"`     // "tap", "swipe", "text", "keyevent"
	X        int    `json:"x"`
	Y        int    `json:"y"`
	StartX   int    `json:"start_x"`
	StartY   int    `json:"start_y"`
	EndX     int    `json:"end_x"`
	EndY     int    `json:"end_y"`
	Duration int    `json:"duration"`
	Text     string `json:"text"`
	Keycode  int    `json:"keycode"`
}

func HandleStream(upgrader *websocket.Upgrader, streamMgr *StreamManager, adbClient *adb.ADBClient, adbPort int, deviceID string, containerName string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Printf("websocket upgrade error: %v", err)
			return
		}
		defer conn.Close()

		// Ensure stream is running
		streamMgr.StartStream(deviceID, adbPort, containerName)

		// Subscribe to frames
		frameCh, unsub := streamMgr.Subscribe(deviceID)
		if frameCh == nil {
			log.Printf("failed to subscribe to stream for device %s (session not found)", deviceID)
			conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"error","message":"stream not available"}`))
			return
		}
		log.Printf("client subscribed to stream for device %s", deviceID)
		defer unsub()

		done := make(chan struct{})

		// Read goroutine: handle input events
		go func() {
			defer close(done)
			for {
				_, msg, err := conn.ReadMessage()
				if err != nil {
					if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
						log.Printf("websocket read error: %v", err)
					}
					return
				}

				var input InputMessage
				if err := json.Unmarshal(msg, &input); err != nil {
					log.Printf("invalid input message: %v", err)
					continue
				}

				log.Printf("input received: type=%s x=%d y=%d keycode=%d text=%q", input.Type, input.X, input.Y, input.Keycode, input.Text)

				switch input.Type {
				case "tap":
					if err := adbClient.Tap(adbPort, input.X, input.Y); err != nil {
						log.Printf("tap error: %v", err)
					}
				case "swipe":
					dur := input.Duration
					if dur <= 0 {
						dur = 300
					}
					if err := adbClient.Swipe(adbPort, input.StartX, input.StartY, input.EndX, input.EndY, dur); err != nil {
						log.Printf("swipe error: %v", err)
					}
				case "text":
					if err := adbClient.InputText(adbPort, input.Text); err != nil {
						log.Printf("input text error: %v", err)
					}
				case "keyevent":
					if err := adbClient.KeyEvent(adbPort, input.Keycode); err != nil {
						log.Printf("keyevent error: %v", err)
					}
				default:
					log.Printf("unknown input type: %s", input.Type)
				}
			}
		}()

		// Write goroutine: send frames
		for {
			select {
			case <-done:
				return
			case frame, ok := <-frameCh:
				if !ok {
					return
				}
				if err := conn.WriteMessage(websocket.BinaryMessage, frame); err != nil {
					log.Printf("websocket write error: %v", err)
					return
				}
			}
		}
	}
}
