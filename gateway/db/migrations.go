package db

import (
	"database/sql"
	"log"
)

func RunMigrations(db *sql.DB) error {
	queries := []string{
		`CREATE EXTENSION IF NOT EXISTS "uuid-ossp"`,

		`CREATE TABLE IF NOT EXISTS users (
			id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
			email VARCHAR(255) UNIQUE NOT NULL,
			password VARCHAR(255) NOT NULL,
			name VARCHAR(255) NOT NULL,
			is_admin BOOLEAN NOT NULL DEFAULT false,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,

		`CREATE TABLE IF NOT EXISTS devices (
			id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
			user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			name VARCHAR(255) NOT NULL,
			container_id VARCHAR(255) DEFAULT '',
			container_name VARCHAR(255) DEFAULT '',
			adb_port INTEGER NOT NULL DEFAULT 0,
			status VARCHAR(50) NOT NULL DEFAULT 'creating',
			error_message TEXT DEFAULT '',
			android_version VARCHAR(50) NOT NULL DEFAULT '11',
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,

		`CREATE TABLE IF NOT EXISTS device_logs (
			id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
			device_id UUID NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
			action VARCHAR(100) NOT NULL,
			message TEXT NOT NULL DEFAULT '',
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,

		`CREATE INDEX IF NOT EXISTS idx_devices_user_id ON devices(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_devices_status ON devices(status)`,
		`CREATE INDEX IF NOT EXISTS idx_device_logs_device_id ON device_logs(device_id)`,
		`CREATE INDEX IF NOT EXISTS idx_users_email ON users(email)`,

		`CREATE TABLE IF NOT EXISTS api_tokens (
			id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
			user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			name VARCHAR(120) NOT NULL,
			token_hash VARCHAR(64) UNIQUE NOT NULL,
			prefix VARCHAR(16) NOT NULL,
			last_used_at TIMESTAMPTZ,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			revoked_at TIMESTAMPTZ
		)`,
		`CREATE INDEX IF NOT EXISTS idx_api_tokens_user_id ON api_tokens(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_api_tokens_token_hash ON api_tokens(token_hash)`,

		// Track which APK version is installed on each device, so the panel
		// can show "Atualizar app" only when there's a newer APK uploaded.
		`ALTER TABLE devices ADD COLUMN IF NOT EXISTS installed_apk_hash VARCHAR(64) NOT NULL DEFAULT ''`,

		// Multi-version APK storage. We never delete entries here — old APKs
		// may still be linked to running devices.
		`CREATE TABLE IF NOT EXISTS apks (
			id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
			filename VARCHAR(255) NOT NULL,
			package_name VARCHAR(255) NOT NULL DEFAULT '',
			version_name VARCHAR(100) NOT NULL DEFAULT '',
			version_code BIGINT NOT NULL DEFAULT 0,
			app_label VARCHAR(255) NOT NULL DEFAULT '',
			size_bytes BIGINT NOT NULL DEFAULT 0,
			sha256 VARCHAR(64) UNIQUE NOT NULL,
			min_sdk INTEGER NOT NULL DEFAULT 0,
			target_sdk INTEGER NOT NULL DEFAULT 0,
			is_default BOOLEAN NOT NULL DEFAULT FALSE,
			source VARCHAR(50) NOT NULL DEFAULT 'upload',
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_apks_default_unique ON apks(is_default) WHERE is_default = TRUE`,
		`CREATE INDEX IF NOT EXISTS idx_apks_version_code ON apks(version_code DESC)`,

		// Devices reference which APK version they were provisioned with
		`ALTER TABLE devices ADD COLUMN IF NOT EXISTS apk_id UUID REFERENCES apks(id) ON DELETE SET NULL`,

		// Mass dispatch campaigns
		`CREATE TABLE IF NOT EXISTS campaigns (
			id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
			user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			name VARCHAR(255) NOT NULL,
			message TEXT NOT NULL,
			delay_min_seconds INTEGER NOT NULL DEFAULT 30,
			delay_max_seconds INTEGER NOT NULL DEFAULT 90,
			device_ids TEXT NOT NULL DEFAULT '[]',
			status VARCHAR(20) NOT NULL DEFAULT 'draft',
			total INTEGER NOT NULL DEFAULT 0,
			sent INTEGER NOT NULL DEFAULT 0,
			failed INTEGER NOT NULL DEFAULT 0,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			started_at TIMESTAMPTZ,
			completed_at TIMESTAMPTZ
		)`,
		`CREATE INDEX IF NOT EXISTS idx_campaigns_user_id ON campaigns(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_campaigns_status ON campaigns(status)`,

		`CREATE TABLE IF NOT EXISTS campaign_rows (
			id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
			campaign_id UUID NOT NULL REFERENCES campaigns(id) ON DELETE CASCADE,
			phone VARCHAR(50) NOT NULL,
			name VARCHAR(255) NOT NULL DEFAULT '',
			status VARCHAR(20) NOT NULL DEFAULT 'pending',
			error_msg TEXT NOT NULL DEFAULT '',
			device_id UUID,
			sent_at TIMESTAMPTZ,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,
		`CREATE INDEX IF NOT EXISTS idx_campaign_rows_campaign ON campaign_rows(campaign_id, status)`,
	}

	for _, q := range queries {
		if _, err := db.Exec(q); err != nil {
			log.Printf("migration error on query: %s\nerror: %v", q, err)
			return err
		}
	}

	log.Println("database migrations completed successfully")
	return nil
}
