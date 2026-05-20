package db

import (
	"database/sql"
	"fmt"

	"github.com/emulador/gateway/models"
)

func CreateUser(db *sql.DB, email, passwordHash, name string) (*models.User, error) {
	user := &models.User{}
	err := db.QueryRow(
		`INSERT INTO users (email, password, name) VALUES ($1, $2, $3)
		 RETURNING id, email, password, name, is_admin, created_at, updated_at`,
		email, passwordHash, name,
	).Scan(&user.ID, &user.Email, &user.Password, &user.Name, &user.IsAdmin, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return user, nil
}

func GetUserByEmail(db *sql.DB, email string) (*models.User, error) {
	user := &models.User{}
	err := db.QueryRow(
		`SELECT id, email, password, name, is_admin, created_at, updated_at FROM users WHERE email = $1`,
		email,
	).Scan(&user.ID, &user.Email, &user.Password, &user.Name, &user.IsAdmin, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return user, nil
}

func GetUserByID(db *sql.DB, id string) (*models.User, error) {
	user := &models.User{}
	err := db.QueryRow(
		`SELECT id, email, password, name, is_admin, created_at, updated_at FROM users WHERE id = $1`,
		id,
	).Scan(&user.ID, &user.Email, &user.Password, &user.Name, &user.IsAdmin, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return user, nil
}

func CreateDevice(db *sql.DB, userID, name, androidVersion string) (*models.Device, error) {
	device := &models.Device{}
	err := db.QueryRow(
		`INSERT INTO devices (user_id, name, android_version) VALUES ($1, $2, $3)
		 RETURNING id, user_id, name, container_id, container_name, adb_port, status, error_message, android_version, installed_apk_hash, created_at, updated_at`,
		userID, name, androidVersion,
	).Scan(&device.ID, &device.UserID, &device.Name, &device.ContainerID, &device.ContainerName,
		&device.ADBPort, &device.Status, &device.ErrorMessage, &device.AndroidVersion, &device.InstalledAPKHash, &device.CreatedAt, &device.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return device, nil
}

func GetDevicesByUserID(db *sql.DB, userID string) ([]models.Device, error) {
	rows, err := db.Query(
		`SELECT id, user_id, name, container_id, container_name, adb_port, status, error_message, android_version, installed_apk_hash, created_at, updated_at
		 FROM devices WHERE user_id = $1 ORDER BY created_at DESC`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var devices []models.Device
	for rows.Next() {
		var d models.Device
		if err := rows.Scan(&d.ID, &d.UserID, &d.Name, &d.ContainerID, &d.ContainerName,
			&d.ADBPort, &d.Status, &d.ErrorMessage, &d.AndroidVersion, &d.InstalledAPKHash, &d.CreatedAt, &d.UpdatedAt); err != nil {
			return nil, err
		}
		devices = append(devices, d)
	}
	return devices, rows.Err()
}

func GetDeviceByID(db *sql.DB, id string) (*models.Device, error) {
	device := &models.Device{}
	err := db.QueryRow(
		`SELECT id, user_id, name, container_id, container_name, adb_port, status, error_message, android_version, installed_apk_hash, created_at, updated_at
		 FROM devices WHERE id = $1`,
		id,
	).Scan(&device.ID, &device.UserID, &device.Name, &device.ContainerID, &device.ContainerName,
		&device.ADBPort, &device.Status, &device.ErrorMessage, &device.AndroidVersion, &device.InstalledAPKHash, &device.CreatedAt, &device.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return device, nil
}

func UpdateDeviceName(db *sql.DB, id, name string) error {
	_, err := db.Exec(
		`UPDATE devices SET name = $1, updated_at = NOW() WHERE id = $2`,
		name, id,
	)
	return err
}

func UpdateDeviceStatus(db *sql.DB, id, status, errorMsg string) error {
	_, err := db.Exec(
		`UPDATE devices SET status = $1, error_message = $2, updated_at = NOW() WHERE id = $3`,
		status, errorMsg, id,
	)
	return err
}

func UpdateDeviceContainer(db *sql.DB, id, containerID, containerName string, adbPort int) error {
	_, err := db.Exec(
		`UPDATE devices SET container_id = $1, container_name = $2, adb_port = $3, updated_at = NOW() WHERE id = $4`,
		containerID, containerName, adbPort, id,
	)
	return err
}

func DeleteDevice(db *sql.DB, id string) error {
	_, err := db.Exec(`DELETE FROM devices WHERE id = $1`, id)
	return err
}

func GetAllActiveDevices(db *sql.DB) ([]models.Device, error) {
	rows, err := db.Query(
		`SELECT id, user_id, name, container_id, container_name, adb_port, status, error_message, android_version, installed_apk_hash, created_at, updated_at
		 FROM devices WHERE status IN ($1, $2, $3, $4)`,
		models.StatusCreating, models.StatusBooting, models.StatusInstalling, models.StatusReady,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var devices []models.Device
	for rows.Next() {
		var d models.Device
		if err := rows.Scan(&d.ID, &d.UserID, &d.Name, &d.ContainerID, &d.ContainerName,
			&d.ADBPort, &d.Status, &d.ErrorMessage, &d.AndroidVersion, &d.InstalledAPKHash, &d.CreatedAt, &d.UpdatedAt); err != nil {
			return nil, err
		}
		devices = append(devices, d)
	}
	return devices, rows.Err()
}

func UpdateDeviceAPKHash(db *sql.DB, id, hash string) error {
	_, err := db.Exec(
		`UPDATE devices SET installed_apk_hash = $1, updated_at = NOW() WHERE id = $2`,
		hash, id,
	)
	return err
}

func AllocateADBPort(db *sql.DB) (int, error) {
	for port := 5555; port <= 5600; port++ {
		var count int
		err := db.QueryRow(
			`SELECT COUNT(*) FROM devices WHERE adb_port = $1`,
			port,
		).Scan(&count)
		if err != nil {
			return 0, err
		}
		if count == 0 {
			return port, nil
		}
	}
	return 0, fmt.Errorf("no available ADB ports in range 5555-5600")
}

func CreateDeviceLog(db *sql.DB, deviceID, action, message string) error {
	_, err := db.Exec(
		`INSERT INTO device_logs (device_id, action, message) VALUES ($1, $2, $3)`,
		deviceID, action, message,
	)
	return err
}
