package models

import "time"

const (
	CampaignDraft     = "draft"
	CampaignRunning   = "running"
	CampaignPaused    = "paused"
	CampaignCompleted = "completed"
	CampaignFailed    = "failed"
)

const (
	RowPending = "pending"
	RowSending = "sending"
	RowSent    = "sent"
	RowFailed  = "failed"
	RowSkipped = "skipped"
)

type Campaign struct {
	ID              string     `json:"id"`
	UserID          string     `json:"user_id"`
	Name            string     `json:"name"`
	Message         string     `json:"message"`
	DelayMinSeconds int        `json:"delay_min_seconds"`
	DelayMaxSeconds int        `json:"delay_max_seconds"`
	DeviceIDs       []string   `json:"device_ids"`
	Status          string     `json:"status"`
	Total           int        `json:"total"`
	Sent            int        `json:"sent"`
	Failed          int        `json:"failed"`
	CreatedAt       time.Time  `json:"created_at"`
	StartedAt       *time.Time `json:"started_at,omitempty"`
	CompletedAt     *time.Time `json:"completed_at,omitempty"`
}

type CampaignRow struct {
	ID         string     `json:"id"`
	CampaignID string     `json:"campaign_id"`
	Phone      string     `json:"phone"`
	Name       string     `json:"name"`
	Status     string     `json:"status"`
	ErrorMsg   string     `json:"error_msg,omitempty"`
	DeviceID   *string    `json:"device_id,omitempty"`
	SentAt     *time.Time `json:"sent_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
}
