package config

import (
	"os"
)

type Config struct {
	DatabaseURL    string
	RedisURL       string
	JWTSecret      string
	ADBHost        string
	WhatsAppAPK    string
	Port           string
}

func Load() *Config {
	return &Config{
		DatabaseURL:    getEnv("DATABASE_URL", "postgres://emulador:emulador@localhost:5432/emulador?sslmode=disable"),
		RedisURL:       getEnv("REDIS_URL", "redis://localhost:6379"),
		JWTSecret:      getEnv("JWT_SECRET", "change-me-in-production"),
		ADBHost:        getEnv("ADB_HOST", "172.17.0.1"),
		WhatsAppAPK:    getEnv("WHATSAPP_APK_PATH", ""),
		Port:           getEnv("PORT", "8080"),
	}
}

func getEnv(key, fallback string) string {
	if val, ok := os.LookupEnv(key); ok {
		return val
	}
	return fallback
}
