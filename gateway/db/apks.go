package db

import (
	"database/sql"

	"github.com/emulador/gateway/models"
)

const apkColumns = `id, filename, package_name, version_name, version_code, app_label, size_bytes, sha256, min_sdk, target_sdk, is_default, source, created_at`

func scanAPK(row interface {
	Scan(dest ...any) error
}) (*models.APK, error) {
	a := &models.APK{}
	err := row.Scan(&a.ID, &a.Filename, &a.PackageName, &a.VersionName, &a.VersionCode,
		&a.AppLabel, &a.SizeBytes, &a.SHA256, &a.MinSDK, &a.TargetSDK,
		&a.IsDefault, &a.Source, &a.CreatedAt)
	if err != nil {
		return nil, err
	}
	return a, nil
}

func ListAPKs(db *sql.DB) ([]models.APK, error) {
	rows, err := db.Query(`SELECT ` + apkColumns + ` FROM apks ORDER BY version_code DESC, created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.APK
	for rows.Next() {
		var a models.APK
		if err := rows.Scan(&a.ID, &a.Filename, &a.PackageName, &a.VersionName, &a.VersionCode,
			&a.AppLabel, &a.SizeBytes, &a.SHA256, &a.MinSDK, &a.TargetSDK,
			&a.IsDefault, &a.Source, &a.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func GetAPKByID(db *sql.DB, id string) (*models.APK, error) {
	row := db.QueryRow(`SELECT `+apkColumns+` FROM apks WHERE id = $1`, id)
	return scanAPK(row)
}

func GetAPKBySHA256(db *sql.DB, sha string) (*models.APK, error) {
	row := db.QueryRow(`SELECT `+apkColumns+` FROM apks WHERE sha256 = $1`, sha)
	return scanAPK(row)
}

func GetDefaultAPK(db *sql.DB) (*models.APK, error) {
	row := db.QueryRow(`SELECT ` + apkColumns + ` FROM apks WHERE is_default = TRUE LIMIT 1`)
	return scanAPK(row)
}

func GetLatestAPK(db *sql.DB) (*models.APK, error) {
	row := db.QueryRow(`SELECT ` + apkColumns + ` FROM apks ORDER BY version_code DESC, created_at DESC LIMIT 1`)
	return scanAPK(row)
}

func InsertAPK(db *sql.DB, a *models.APK) (*models.APK, error) {
	row := db.QueryRow(`
		INSERT INTO apks (filename, package_name, version_name, version_code, app_label,
		                  size_bytes, sha256, min_sdk, target_sdk, is_default, source)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING `+apkColumns,
		a.Filename, a.PackageName, a.VersionName, a.VersionCode, a.AppLabel,
		a.SizeBytes, a.SHA256, a.MinSDK, a.TargetSDK, a.IsDefault, a.Source,
	)
	return scanAPK(row)
}

// SetDefaultAPK clears any existing default and marks the given id as default.
// Runs in a transaction to keep the unique partial index happy.
func SetDefaultAPK(db *sql.DB, id string) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`UPDATE apks SET is_default = FALSE WHERE is_default = TRUE`); err != nil {
		return err
	}
	if _, err := tx.Exec(`UPDATE apks SET is_default = TRUE WHERE id = $1`, id); err != nil {
		return err
	}
	return tx.Commit()
}

// CountDevicesByAPK returns how many devices reference the given APK id.
func CountDevicesByAPK(db *sql.DB, apkID string) (int, error) {
	var n int
	err := db.QueryRow(`SELECT COUNT(*) FROM devices WHERE apk_id = $1`, apkID).Scan(&n)
	return n, err
}

// UpdateDeviceAPKID records which APK was actually installed on a device.
func UpdateDeviceAPKID(db *sql.DB, deviceID, apkID string) error {
	_, err := db.Exec(`UPDATE devices SET apk_id = $1, updated_at = NOW() WHERE id = $2`, apkID, deviceID)
	return err
}

// LinkDevicesByHash sets apk_id on every device whose installed_apk_hash matches
// the given sha. Used during the legacy ingest migration.
func LinkDevicesByHash(db *sql.DB, sha, apkID string) (int64, error) {
	res, err := db.Exec(
		`UPDATE devices SET apk_id = $1 WHERE installed_apk_hash = $2 AND apk_id IS NULL`,
		apkID, sha,
	)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}
