package adb

import (
	"bufio"
	"fmt"
	"strings"
	"sync"
	"time"
)

// ─── auto-detect the redroid vinput device per ADB port ────────────────────
type vinputCache struct {
	mu sync.Mutex
	m  map[int]string // port → device path (e.g. /dev/input/event5)
}

var vinput = &vinputCache{m: map[int]string{}}

// detectVinput finds the redroid vinput keyboard/touch device by scanning
// `getevent -p` output. ReDroid containers may expose it at /dev/input/eventN
// or directly at /dev/eventN depending on devfs setup.
func (c *ADBClient) detectVinput(port int) (string, error) {
	vinput.mu.Lock()
	if path, ok := vinput.m[port]; ok {
		vinput.mu.Unlock()
		return path, nil
	}
	vinput.mu.Unlock()

	out, err := c.Exec(port, "shell", "getevent", "-p")
	if err != nil {
		return "", fmt.Errorf("getevent -p: %w (%s)", err, string(out))
	}

	var current string
	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		line := scanner.Text()
		// "add device 1: /dev/input/event5"
		if i := strings.Index(line, "add device"); i >= 0 {
			if j := strings.Index(line, ": "); j > 0 {
				current = strings.TrimSpace(line[j+2:])
			}
			continue
		}
		// look for the redroid vinput name marker
		if strings.Contains(line, "redroid vinput") || strings.Contains(line, `name:     "redroid vinput"`) {
			if current != "" && (strings.HasPrefix(current, "/dev/input/") || strings.HasPrefix(current, "/dev/event")) {
				vinput.mu.Lock()
				vinput.m[port] = current
				vinput.mu.Unlock()
				return current, nil
			}
		}
	}
	return "", fmt.Errorf("redroid vinput device not found in getevent -p output")
}

// sendevent batches multiple events into a single shell call to avoid round-trip cost.
func (c *ADBClient) sendeventBatch(port int, events [][3]int) error {
	dev, err := c.detectVinput(port)
	if err != nil {
		return err
	}
	var sb strings.Builder
	for _, e := range events {
		sb.WriteString(fmt.Sprintf("sendevent %s %d %d %d\n", dev, e[0], e[1], e[2]))
	}
	out, err := c.Exec(port, "shell", sb.String())
	if err != nil {
		return fmt.Errorf("sendevent failed: %w (%s)", err, string(out))
	}
	return nil
}

// ─── Touch via multi-touch protocol B ──────────────────────────────────────
const (
	evSyn        = 0
	evKey        = 1
	evAbs        = 3
	synReport    = 0
	btnTouch     = 330
	absMtSlot    = 47
	absMtTrackID = 57
	absMtPosX    = 53
	absMtPosY    = 54
	absMtTouchMaj = 48
)

func (c *ADBClient) Tap(port int, x, y int) error {
	tid := int(time.Now().UnixNano() & 0x7FFFFFFF)
	events := [][3]int{
		{evAbs, absMtSlot, 0},
		{evAbs, absMtTrackID, tid},
		{evAbs, absMtPosX, x},
		{evAbs, absMtPosY, y},
		{evAbs, absMtTouchMaj, 5},
		{evKey, btnTouch, 1},
		{evSyn, synReport, 0},
		{evAbs, absMtTrackID, -1},
		{evKey, btnTouch, 0},
		{evSyn, synReport, 0},
	}
	return c.sendeventBatch(port, events)
}

func (c *ADBClient) Swipe(port int, x1, y1, x2, y2, durationMs int) error {
	tid := int(time.Now().UnixNano() & 0x7FFFFFFF)
	steps := durationMs / 16
	if steps < 5 {
		steps = 5
	}
	if steps > 60 {
		steps = 60
	}

	events := [][3]int{
		{evAbs, absMtSlot, 0},
		{evAbs, absMtTrackID, tid},
		{evAbs, absMtPosX, x1},
		{evAbs, absMtPosY, y1},
		{evAbs, absMtTouchMaj, 5},
		{evKey, btnTouch, 1},
		{evSyn, synReport, 0},
	}
	for i := 1; i <= steps; i++ {
		x := x1 + (x2-x1)*i/steps
		y := y1 + (y2-y1)*i/steps
		events = append(events,
			[3]int{evAbs, absMtPosX, x},
			[3]int{evAbs, absMtPosY, y},
			[3]int{evSyn, synReport, 0},
		)
	}
	events = append(events,
		[3]int{evAbs, absMtTrackID, -1},
		[3]int{evKey, btnTouch, 0},
		[3]int{evSyn, synReport, 0},
	)
	return c.sendeventBatch(port, events)
}

// ─── Keyboard via uinputd ──────────────────────────────────────────────────
// Linux input event KEY_* scancodes — same device exposes touch + keyboard.
const (
	keyLeftShift = 42
	keyBackspace = 14
	keyEnter     = 28
	keySpace     = 57
	keyDot       = 52
	keyComma     = 51
	keyMinus     = 12
	keySlash     = 53
)

// charKey maps a printable character to (keycode, needsShift).
// Returns (0, false) for unsupported characters.
func charKey(r rune) (int, bool) {
	if r >= 'a' && r <= 'z' {
		return letterKey(r), false
	}
	if r >= 'A' && r <= 'Z' {
		return letterKey(r + ('a' - 'A')), true
	}
	if r >= '0' && r <= '9' {
		// digits row: '1'=2, '2'=3, ..., '9'=10, '0'=11
		if r == '0' {
			return 11, false
		}
		return int(r-'1') + 2, false
	}
	switch r {
	case ' ':
		return keySpace, false
	case '\n':
		return keyEnter, false
	case '.':
		return keyDot, false
	case ',':
		return keyComma, false
	case '-':
		return keyMinus, false
	case '/':
		return keySlash, false
	case '!':
		return 2, true // shift+1
	case '@':
		return 3, true // shift+2
	case '#':
		return 4, true
	case '$':
		return 5, true
	case '%':
		return 6, true
	case '+':
		return 13, true // shift+= (KEY_EQUAL=13)
	case '_':
		return keyMinus, true
	case '?':
		return keySlash, true
	case '(':
		return 10, true // shift+9
	case ')':
		return 11, true // shift+0
	}
	return 0, false
}

// letterKey returns scancode for a-z (English QWERTY layout).
func letterKey(r rune) int {
	// QWERTY row 1: q w e r t y u i o p   (16-25)
	// row 2:        a s d f g h j k l     (30-38)
	// row 3:        z x c v b n m         (44-50)
	row1 := "qwertyuiop"
	row2 := "asdfghjkl"
	row3 := "zxcvbnm"
	if i := strings.IndexRune(row1, r); i >= 0 {
		return 16 + i
	}
	if i := strings.IndexRune(row2, r); i >= 0 {
		return 30 + i
	}
	if i := strings.IndexRune(row3, r); i >= 0 {
		return 44 + i
	}
	return 0
}

func (c *ADBClient) InputText(port int, text string) error {
	if text == "" {
		return nil
	}
	var events [][3]int
	for _, r := range text {
		kc, shift := charKey(r)
		if kc == 0 {
			continue // skip unsupported
		}
		if shift {
			events = append(events,
				[3]int{evKey, keyLeftShift, 1},
				[3]int{evSyn, synReport, 0},
			)
		}
		events = append(events,
			[3]int{evKey, kc, 1},
			[3]int{evSyn, synReport, 0},
			[3]int{evKey, kc, 0},
			[3]int{evSyn, synReport, 0},
		)
		if shift {
			events = append(events,
				[3]int{evKey, keyLeftShift, 0},
				[3]int{evSyn, synReport, 0},
			)
		}
	}
	if len(events) == 0 {
		return nil
	}
	return c.sendeventBatch(port, events)
}

func (c *ADBClient) KeyEvent(port int, keycode int) error {
	// Navigation/system keys (BACK, HOME, RECENTS, MENU, POWER, VOL+/-) work
	// reliably via `input keyevent` because they bypass the IME entirely and
	// hit the Android InputDispatcher directly. We don't need sendevent here.
	out, err := c.Exec(port, "shell", "input", "keyevent", fmt.Sprintf("%d", keycode))
	if err != nil {
		return fmt.Errorf("keyevent failed: %w (%s)", err, string(out))
	}
	return nil
}
