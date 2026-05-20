package models

import "time"

type APK struct {
	ID          string    `json:"id"`
	Filename    string    `json:"filename"`
	PackageName string    `json:"package_name"`
	VersionName string    `json:"version_name"`
	VersionCode int64     `json:"version_code"`
	AppLabel    string    `json:"app_label"`
	SizeBytes   int64     `json:"size_bytes"`
	SHA256      string    `json:"sha256"`
	MinSDK      int       `json:"min_sdk"`
	TargetSDK   int       `json:"target_sdk"`
	IsDefault   bool      `json:"is_default"`
	Source      string    `json:"source"` // "upload" | "auto-download" | "legacy"
	CreatedAt   time.Time `json:"created_at"`
}
