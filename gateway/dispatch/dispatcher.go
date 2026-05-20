// Package dispatch runs background workers that send WhatsApp messages
// for active mass-dispatch campaigns. Each worker loop pulls the next
// pending row in the campaign, sends the message via the device's ADB
// connection, marks the row sent/failed, and sleeps a random delay
// between (delay_min, delay_max) before the next message.
package dispatch

import (
	"context"
	"log"
	"math/rand"
	"strings"
	"sync"
	"time"

	"github.com/emulador/gateway/adb"
	"github.com/emulador/gateway/db"
	"github.com/emulador/gateway/handlers"
	"github.com/emulador/gateway/models"

	dbpkg "database/sql"
)

type Dispatcher struct {
	db        *dbpkg.DB
	adb       *adb.ADBClient
	mu        sync.Mutex
	managed   map[string]context.CancelFunc // campaignID → cancel
}

func New(database *dbpkg.DB, adbClient *adb.ADBClient) *Dispatcher {
	return &Dispatcher{
		db:      database,
		adb:     adbClient,
		managed: map[string]context.CancelFunc{},
	}
}

// Start initialises the dispatcher: clears stuck "sending" rows, then
// resumes any campaigns that were running before a restart, and finally
// kicks off a poll loop that picks up newly-started campaigns every 5s.
func (d *Dispatcher) Start() {
	if err := db.ResetSendingToPending(d.db); err != nil {
		log.Printf("dispatcher: reset sending rows: %v", err)
	}
	go d.pollLoop()
}

func (d *Dispatcher) pollLoop() {
	t := time.NewTicker(5 * time.Second)
	defer t.Stop()
	for range t.C {
		running, err := db.ListRunningCampaigns(d.db)
		if err != nil {
			log.Printf("dispatcher: list running: %v", err)
			continue
		}
		for _, c := range running {
			d.ensureRunning(c)
		}
	}
}

// ensureRunning makes sure there is a worker goroutine pool for the given
// campaign. Idempotent.
func (d *Dispatcher) ensureRunning(c models.Campaign) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if _, ok := d.managed[c.ID]; ok {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	d.managed[c.ID] = cancel
	go d.runCampaign(ctx, c)
}

// Stop cancels in-memory workers for a campaign (used on pause/delete).
// Note: the row-level state stays in DB so we can resume cleanly.
func (d *Dispatcher) Stop(campaignID string) {
	d.mu.Lock()
	cancel, ok := d.managed[campaignID]
	if ok {
		delete(d.managed, campaignID)
	}
	d.mu.Unlock()
	if ok {
		cancel()
	}
}

// runCampaign orchestrates one worker per selected device and waits for
// them all to finish (no more pending rows or context cancelled). On
// natural completion it marks the campaign as completed.
func (d *Dispatcher) runCampaign(ctx context.Context, c models.Campaign) {
	defer func() {
		d.mu.Lock()
		delete(d.managed, c.ID)
		d.mu.Unlock()
	}()

	if len(c.DeviceIDs) == 0 {
		log.Printf("dispatcher: campaign %s has no devices selected — failing", c.ID)
		_ = db.UpdateCampaignStatus(d.db, c.ID, models.CampaignFailed)
		return
	}

	log.Printf("dispatcher: starting campaign %s with %d devices", c.ID, len(c.DeviceIDs))

	var wg sync.WaitGroup
	for _, deviceID := range c.DeviceIDs {
		wg.Add(1)
		go func(devID string) {
			defer wg.Done()
			d.workerLoop(ctx, c, devID)
		}(deviceID)
	}
	wg.Wait()

	// re-check status: only mark completed if there are no pending rows AND
	// the campaign is still in 'running' state (could've been paused).
	pending, _ := db.CountPendingRows(d.db, c.ID)
	cur, _ := db.GetCampaign(d.db, c.ID)
	if cur != nil && cur.Status == models.CampaignRunning && pending == 0 {
		log.Printf("dispatcher: campaign %s completed", c.ID)
		_ = db.UpdateCampaignStatus(d.db, c.ID, models.CampaignCompleted)
	}
}

func (d *Dispatcher) workerLoop(ctx context.Context, c models.Campaign, deviceID string) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		// re-check campaign status — bail out if paused/cancelled
		cur, err := db.GetCampaign(d.db, c.ID)
		if err != nil || cur == nil || cur.Status != models.CampaignRunning {
			return
		}

		// resolve the device — must be ready
		device, err := db.GetDeviceByID(d.db, deviceID)
		if err != nil || device == nil || device.Status != "ready" {
			log.Printf("dispatcher: campaign=%s device=%s not ready (%v) — skipping for 30s", c.ID, deviceID, err)
			select {
			case <-ctx.Done():
				return
			case <-time.After(30 * time.Second):
				continue
			}
		}

		row, err := db.ClaimNextRow(d.db, c.ID, deviceID)
		if err != nil {
			log.Printf("dispatcher: claim row failed campaign=%s: %v", c.ID, err)
			return
		}
		if row == nil {
			// no more pending rows for this campaign
			return
		}

		// render template (simple {{phone}} and {{name}} substitution)
		msg := renderTemplate(c.Message, row)

		log.Printf("dispatcher: campaign=%s device=%s sending to %s", c.ID, deviceID, row.Phone)
		sendCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
		res := handlers.InternalSendWhatsApp(sendCtx, d.adb, device.ADBPort, row.Phone, msg)
		cancel()

		if res.OK {
			_ = db.MarkRowSent(d.db, row.ID)
			log.Printf("dispatcher: campaign=%s row=%s sent OK", c.ID, row.ID)
		} else {
			_ = db.MarkRowFailed(d.db, row.ID, res.ErrMsg)
			log.Printf("dispatcher: campaign=%s row=%s FAILED: %s", c.ID, row.ID, res.ErrMsg)
		}

		// random delay between dispatches
		delay := randomDelay(c.DelayMinSeconds, c.DelayMaxSeconds)
		select {
		case <-ctx.Done():
			return
		case <-time.After(delay):
		}
	}
}

func renderTemplate(tpl string, row *models.CampaignRow) string {
	out := tpl
	out = strings.ReplaceAll(out, "{{phone}}", row.Phone)
	out = strings.ReplaceAll(out, "{{name}}", row.Name)
	return out
}

func randomDelay(min, max int) time.Duration {
	if min < 1 {
		min = 1
	}
	if max < min {
		max = min
	}
	span := max - min
	d := min
	if span > 0 {
		d = min + rand.Intn(span+1)
	}
	return time.Duration(d) * time.Second
}
