package db

import (
	"database/sql"
	"encoding/json"

	"github.com/emulador/gateway/models"
)

func scanCampaign(row interface{ Scan(...any) error }) (*models.Campaign, error) {
	c := &models.Campaign{}
	var deviceJSON string
	err := row.Scan(&c.ID, &c.UserID, &c.Name, &c.Message,
		&c.DelayMinSeconds, &c.DelayMaxSeconds, &deviceJSON,
		&c.Status, &c.Total, &c.Sent, &c.Failed,
		&c.CreatedAt, &c.StartedAt, &c.CompletedAt)
	if err != nil {
		return nil, err
	}
	if deviceJSON == "" {
		deviceJSON = "[]"
	}
	if err := json.Unmarshal([]byte(deviceJSON), &c.DeviceIDs); err != nil {
		c.DeviceIDs = nil
	}
	return c, nil
}

const campaignCols = `id, user_id, name, message, delay_min_seconds, delay_max_seconds, device_ids, status, total, sent, failed, created_at, started_at, completed_at`

func CreateCampaign(db *sql.DB, c *models.Campaign) (*models.Campaign, error) {
	deviceJSON, _ := json.Marshal(c.DeviceIDs)
	row := db.QueryRow(`INSERT INTO campaigns
		(user_id, name, message, delay_min_seconds, delay_max_seconds, device_ids, status, total)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		RETURNING `+campaignCols,
		c.UserID, c.Name, c.Message, c.DelayMinSeconds, c.DelayMaxSeconds,
		string(deviceJSON), c.Status, c.Total)
	return scanCampaign(row)
}

func ListCampaignsByUser(db *sql.DB, userID string) ([]models.Campaign, error) {
	rows, err := db.Query(`SELECT `+campaignCols+` FROM campaigns WHERE user_id = $1 ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.Campaign
	for rows.Next() {
		c, err := scanCampaign(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *c)
	}
	return out, rows.Err()
}

func GetCampaign(db *sql.DB, id string) (*models.Campaign, error) {
	return scanCampaign(db.QueryRow(`SELECT `+campaignCols+` FROM campaigns WHERE id = $1`, id))
}

func UpdateCampaignStatus(db *sql.DB, id, status string) error {
	q := `UPDATE campaigns SET status = $1`
	args := []any{status}
	switch status {
	case models.CampaignRunning:
		q += `, started_at = COALESCE(started_at, NOW())`
	case models.CampaignCompleted, models.CampaignFailed:
		q += `, completed_at = NOW()`
	}
	q += ` WHERE id = $2`
	args = append(args, id)
	_, err := db.Exec(q, args...)
	return err
}

func DeleteCampaign(db *sql.DB, id string) error {
	_, err := db.Exec(`DELETE FROM campaigns WHERE id = $1`, id)
	return err
}

func ListRunningCampaigns(db *sql.DB) ([]models.Campaign, error) {
	rows, err := db.Query(`SELECT ` + campaignCols + ` FROM campaigns WHERE status = 'running'`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.Campaign
	for rows.Next() {
		c, err := scanCampaign(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *c)
	}
	return out, rows.Err()
}

// ─── rows ─────────────────────────────────────────────────────────────

func InsertCampaignRows(db *sql.DB, campaignID string, rows []models.CampaignRow) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	stmt, err := tx.Prepare(`INSERT INTO campaign_rows (campaign_id, phone, name) VALUES ($1, $2, $3)`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, r := range rows {
		if _, err := stmt.Exec(campaignID, r.Phone, r.Name); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func ListCampaignRows(db *sql.DB, campaignID, status string, limit, offset int) ([]models.CampaignRow, error) {
	q := `SELECT id, campaign_id, phone, name, status, error_msg, device_id, sent_at, created_at FROM campaign_rows WHERE campaign_id = $1`
	args := []any{campaignID}
	if status != "" {
		q += ` AND status = $2`
		args = append(args, status)
	}
	q += ` ORDER BY created_at ASC LIMIT ` + itoa(limit) + ` OFFSET ` + itoa(offset)
	rows, err := db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.CampaignRow
	for rows.Next() {
		var r models.CampaignRow
		var devID sql.NullString
		var sentAt sql.NullTime
		if err := rows.Scan(&r.ID, &r.CampaignID, &r.Phone, &r.Name, &r.Status,
			&r.ErrorMsg, &devID, &sentAt, &r.CreatedAt); err != nil {
			return nil, err
		}
		if devID.Valid {
			s := devID.String
			r.DeviceID = &s
		}
		if sentAt.Valid {
			t := sentAt.Time
			r.SentAt = &t
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// ClaimNextRow picks one pending row in this campaign and atomically marks it
// "sending" with the given device id. Returns nil if no pending row left.
func ClaimNextRow(db *sql.DB, campaignID, deviceID string) (*models.CampaignRow, error) {
	row := db.QueryRow(`
		UPDATE campaign_rows
		SET status = 'sending', device_id = $2
		WHERE id = (
			SELECT id FROM campaign_rows
			WHERE campaign_id = $1 AND status = 'pending'
			ORDER BY created_at ASC
			LIMIT 1
			FOR UPDATE SKIP LOCKED
		)
		RETURNING id, campaign_id, phone, name, status, error_msg, device_id, sent_at, created_at`,
		campaignID, deviceID)
	var r models.CampaignRow
	var devID sql.NullString
	var sentAt sql.NullTime
	if err := row.Scan(&r.ID, &r.CampaignID, &r.Phone, &r.Name, &r.Status,
		&r.ErrorMsg, &devID, &sentAt, &r.CreatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	if devID.Valid {
		s := devID.String
		r.DeviceID = &s
	}
	return &r, nil
}

func MarkRowSent(db *sql.DB, rowID string) error {
	_, err := db.Exec(`UPDATE campaign_rows SET status = 'sent', sent_at = NOW() WHERE id = $1`, rowID)
	if err == nil {
		// bump campaign counter
		_, _ = db.Exec(`UPDATE campaigns SET sent = sent + 1 WHERE id = (SELECT campaign_id FROM campaign_rows WHERE id = $1)`, rowID)
	}
	return err
}

func MarkRowFailed(db *sql.DB, rowID, msg string) error {
	_, err := db.Exec(`UPDATE campaign_rows SET status = 'failed', error_msg = $1 WHERE id = $2`, msg, rowID)
	if err == nil {
		_, _ = db.Exec(`UPDATE campaigns SET failed = failed + 1 WHERE id = (SELECT campaign_id FROM campaign_rows WHERE id = $1)`, rowID)
	}
	return err
}

// CountPending returns the number of remaining pending rows in a campaign.
func CountPendingRows(db *sql.DB, campaignID string) (int, error) {
	var n int
	err := db.QueryRow(`SELECT COUNT(*) FROM campaign_rows WHERE campaign_id = $1 AND status = 'pending'`, campaignID).Scan(&n)
	return n, err
}

// ResetSendingToPending is run on dispatcher startup to recover from crashes:
// any rows stuck in 'sending' are returned to the queue.
func ResetSendingToPending(db *sql.DB) error {
	_, err := db.Exec(`UPDATE campaign_rows SET status = 'pending', device_id = NULL WHERE status = 'sending'`)
	return err
}

func itoa(n int) string {
	if n < 0 {
		return "0"
	}
	if n == 0 {
		return "0"
	}
	const digits = "0123456789"
	if n < 10 {
		return string(digits[n])
	}
	out := ""
	for n > 0 {
		out = string(digits[n%10]) + out
		n /= 10
	}
	return out
}
