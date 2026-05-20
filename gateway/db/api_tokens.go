package db

import (
	"database/sql"

	"github.com/emulador/gateway/models"
)

func CreateAPIToken(db *sql.DB, userID, name, prefix, tokenHash string) (*models.APIToken, error) {
	t := &models.APIToken{}
	err := db.QueryRow(
		`INSERT INTO api_tokens (user_id, name, prefix, token_hash)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id, user_id, name, prefix, last_used_at, created_at, revoked_at`,
		userID, name, prefix, tokenHash,
	).Scan(&t.ID, &t.UserID, &t.Name, &t.Prefix, &t.LastUsedAt, &t.CreatedAt, &t.RevokedAt)
	if err != nil {
		return nil, err
	}
	return t, nil
}

func ListAPITokensByUser(db *sql.DB, userID string) ([]models.APIToken, error) {
	rows, err := db.Query(
		`SELECT id, user_id, name, prefix, last_used_at, created_at, revoked_at
		 FROM api_tokens WHERE user_id = $1 ORDER BY created_at DESC`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ts []models.APIToken
	for rows.Next() {
		var t models.APIToken
		if err := rows.Scan(&t.ID, &t.UserID, &t.Name, &t.Prefix, &t.LastUsedAt, &t.CreatedAt, &t.RevokedAt); err != nil {
			return nil, err
		}
		ts = append(ts, t)
	}
	return ts, rows.Err()
}

func GetAPITokenByHash(db *sql.DB, hash string) (*models.APIToken, error) {
	t := &models.APIToken{}
	err := db.QueryRow(
		`SELECT id, user_id, name, prefix, last_used_at, created_at, revoked_at
		 FROM api_tokens WHERE token_hash = $1 AND revoked_at IS NULL`,
		hash,
	).Scan(&t.ID, &t.UserID, &t.Name, &t.Prefix, &t.LastUsedAt, &t.CreatedAt, &t.RevokedAt)
	if err != nil {
		return nil, err
	}
	return t, nil
}

func RevokeAPIToken(db *sql.DB, id, userID string) (int64, error) {
	res, err := db.Exec(
		`UPDATE api_tokens SET revoked_at = NOW()
		 WHERE id = $1 AND user_id = $2 AND revoked_at IS NULL`,
		id, userID,
	)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func TouchAPITokenLastUsed(db *sql.DB, id string) {
	_, _ = db.Exec(`UPDATE api_tokens SET last_used_at = NOW() WHERE id = $1`, id)
}
