package streaming

import (
	"context"
	"log"
	"sync"

	"github.com/emulador/gateway/adb"
)

type subscriber struct {
	ch chan []byte
}

type session struct {
	cancel      context.CancelFunc
	mu          sync.RWMutex
	subscribers map[*subscriber]struct{}
}

func (s *session) addSubscriber() (*subscriber, func()) {
	sub := &subscriber{
		ch: make(chan []byte, 5), // buffer a few frames
	}
	s.mu.Lock()
	s.subscribers[sub] = struct{}{}
	s.mu.Unlock()

	unsub := func() {
		s.mu.Lock()
		delete(s.subscribers, sub)
		close(sub.ch)
		s.mu.Unlock()
	}
	return sub, unsub
}

func (s *session) broadcast(frame []byte) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for sub := range s.subscribers {
		select {
		case sub.ch <- frame:
		default:
			// drop frame if subscriber is too slow
		}
	}
}

func (s *session) subscriberCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.subscribers)
}

type StreamManager struct {
	sessions  sync.Map // map[string]*session
	adbClient *adb.ADBClient
}

func NewStreamManager(adbClient *adb.ADBClient) *StreamManager {
	return &StreamManager{
		adbClient: adbClient,
	}
}

func (m *StreamManager) StartStream(deviceID string, adbPort int, containerName string) {
	// If session already exists, do nothing
	if _, loaded := m.sessions.Load(deviceID); loaded {
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	sess := &session{
		cancel:      cancel,
		subscribers: make(map[*subscriber]struct{}),
	}

	// Store session, but check if another goroutine created one first
	if actual, loaded := m.sessions.LoadOrStore(deviceID, sess); loaded {
		cancel() // we lost the race, cancel our context
		_ = actual
		return
	}

	log.Printf("starting stream for device %s on port %d", deviceID, adbPort)

	go func() {
		captureLoop(ctx, m.adbClient, adbPort, containerName, sess.broadcast)
		m.sessions.Delete(deviceID)
		log.Printf("stream stopped for device %s", deviceID)
	}()
}

func (m *StreamManager) StopStream(deviceID string) {
	if val, ok := m.sessions.Load(deviceID); ok {
		sess := val.(*session)
		sess.cancel()
		m.sessions.Delete(deviceID)
		log.Printf("requested stream stop for device %s", deviceID)
	}
}

func (m *StreamManager) Subscribe(deviceID string) (chan []byte, func()) {
	val, ok := m.sessions.Load(deviceID)
	if !ok {
		return nil, func() {}
	}
	sess := val.(*session)
	sub, unsub := sess.addSubscriber()
	return sub.ch, unsub
}
