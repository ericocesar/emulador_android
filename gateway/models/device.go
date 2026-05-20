package models

import "time"

const (
	StatusCreating   = "creating"
	StatusBooting    = "booting"
	StatusInstalling = "installing"
	StatusReady      = "ready"
	StatusStopped    = "stopped"
	StatusError      = "error"
)

type Device struct {
	ID               string    `json:"id"`
	UserID           string    `json:"user_id"`
	Name             string    `json:"name"`
	ContainerID      string    `json:"container_id,omitempty"`
	ContainerName    string    `json:"container_name,omitempty"`
	ADBPort          int       `json:"adb_port"`
	Status           string    `json:"status"`
	ErrorMessage     string    `json:"error_message,omitempty"`
	AndroidVersion   string    `json:"android_version"`
	InstalledAPKHash string    `json:"installed_apk_hash"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

type DeviceLog struct {
	ID        string    `json:"id"`
	DeviceID  string    `json:"device_id"`
	Action    string    `json:"action"`
	Message   string    `json:"message"`
	CreatedAt time.Time `json:"created_at"`
}
