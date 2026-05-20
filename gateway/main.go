package main

import (
	"context"
	"database/sql"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/docker/docker/client"
	"github.com/gorilla/mux"

	"github.com/emulador/gateway/adb"
	"github.com/emulador/gateway/auth"
	"github.com/emulador/gateway/config"
	"github.com/emulador/gateway/db"
	"github.com/emulador/gateway/dispatch"
	dkr "github.com/emulador/gateway/docker"
	"github.com/emulador/gateway/handlers"
	"github.com/emulador/gateway/models"
	"github.com/emulador/gateway/streaming"
)

func main() {
	cfg := config.Load()

	// Connect to PostgreSQL
	database, err := db.NewPostgresDB(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer database.Close()
	log.Println("connected to PostgreSQL")

	// Run migrations
	if err := db.RunMigrations(database); err != nil {
		log.Fatalf("failed to run migrations: %v", err)
	}

	// One-time legacy ingest: import /apk/whatsapp.apk into the apks table
	// (so existing devices keep working, and future uploads go in the same lib).
	handlers.IngestLegacyAPK(database, cfg.WhatsAppAPK)

	// Normalise app_label for known packages (legacy rows may have non-Latin labels)
	handlers.FixupKnownAPKLabels(database)

	// Start the once-a-day WhatsApp upstream checker
	handlers.StartDailyAPKChecker(database, cfg.WhatsAppAPK)

	// Connect to Docker
	dockerCli, err := dkr.NewDockerClient()
	if err != nil {
		log.Fatalf("failed to connect to Docker: %v", err)
	}
	defer dockerCli.Close()
	log.Println("connected to Docker")

	// Initialize ADB client
	adbClient := adb.NewADBClient(cfg.ADBHost)

	// Initialize stream manager
	streamMgr := streaming.NewStreamManager(adbClient)

	// Reconcile devices on startup
	reconcileDevices(database, dockerCli)

	// Start the mass-dispatch background worker (after ADB ready)
	dispatcher := dispatch.New(database, adbClient)
	dispatcher.Start()

	// Setup router
	r := mux.NewRouter()
	r.Use(corsMiddleware)

	// Public routes
	r.HandleFunc("/health", handlers.HealthHandler()).Methods("GET")
	r.HandleFunc("/api/auth/register", handlers.RegisterHandler(database, cfg.JWTSecret)).Methods("POST")
	r.HandleFunc("/api/auth/login", handlers.LoginHandler(database, cfg.JWTSecret)).Methods("POST")

	// WebSocket route (auth via query param)
	r.HandleFunc("/api/devices/{id}/stream", handlers.StreamHandler(database, streamMgr, adbClient, cfg.JWTSecret)).Methods("GET")

	// Protected routes (accept JWT or API token)
	api := r.PathPrefix("/api").Subrouter()
	api.Use(auth.AuthMiddleware(cfg.JWTSecret, database))

	api.HandleFunc("/auth/me", handlers.MeHandler(database)).Methods("GET")
	api.HandleFunc("/devices", handlers.ListDevicesHandler(database)).Methods("GET")
	api.HandleFunc("/devices", handlers.CreateDeviceHandler(database, dockerCli, adbClient, cfg.WhatsAppAPK)).Methods("POST")
	api.HandleFunc("/devices/{id}", handlers.GetDeviceHandler(database)).Methods("GET")
	api.HandleFunc("/devices/{id}", handlers.UpdateDeviceHandler(database)).Methods("PATCH", "PUT")
	api.HandleFunc("/devices/{id}", handlers.DeleteDeviceHandler(database, dockerCli, streamMgr)).Methods("DELETE")
	api.HandleFunc("/devices/{id}/start", handlers.StartDeviceHandler(database, dockerCli, adbClient)).Methods("POST")
	api.HandleFunc("/devices/{id}/stop", handlers.StopDeviceHandler(database, dockerCli, streamMgr)).Methods("POST")
	api.HandleFunc("/devices/{id}/stats", handlers.DeviceStatsHandler(database, dockerCli)).Methods("GET")
	api.HandleFunc("/devices/{id}/update-app", handlers.UpdateDeviceAppHandler(database, dockerCli, adbClient, cfg.WhatsAppAPK)).Methods("POST")
	api.HandleFunc("/devices/{id}/whatsapp/open", handlers.WhatsAppOpenHandler(database, adbClient)).Methods("POST")
	api.HandleFunc("/devices/{id}/whatsapp/send", handlers.WhatsAppSendMessageHandler(database, adbClient)).Methods("POST")

	// APK management — legacy single-file endpoints kept for backwards compat
	api.HandleFunc("/apk", handlers.GetAPKInfoHandler(cfg.WhatsAppAPK)).Methods("GET")

	// Multi-version APK management (new)
	api.HandleFunc("/apks", handlers.ListAPKsHandler(database)).Methods("GET")
	api.HandleFunc("/apks", handlers.UploadAPKv2Handler(database, cfg.WhatsAppAPK)).Methods("POST")
	api.HandleFunc("/apks/check-update", handlers.CheckUpdateHandler(database, cfg.WhatsAppAPK)).Methods("POST")
	api.HandleFunc("/apks/{id}/default", handlers.SetDefaultAPKHandler(database)).Methods("PATCH", "POST")

	// API tokens (CRUD via panel JWT only — but middleware accepts both,
	// allowing one token to mint another if needed)
	api.HandleFunc("/tokens", handlers.ListAPITokensHandler(database)).Methods("GET")
	api.HandleFunc("/tokens", handlers.CreateAPITokenHandler(database)).Methods("POST")
	api.HandleFunc("/tokens/{id}", handlers.RevokeAPITokenHandler(database)).Methods("DELETE")

	// Mass dispatch campaigns
	api.HandleFunc("/campaigns", handlers.ListCampaignsHandler(database)).Methods("GET")
	api.HandleFunc("/campaigns", handlers.CreateCampaignHandler(database)).Methods("POST")
	api.HandleFunc("/campaigns/{id}", handlers.GetCampaignHandler(database)).Methods("GET")
	api.HandleFunc("/campaigns/{id}", handlers.DeleteCampaignHandler(database, dispatcher)).Methods("DELETE")
	api.HandleFunc("/campaigns/{id}/rows", handlers.ListCampaignRowsHandler(database)).Methods("GET")
	api.HandleFunc("/campaigns/{id}/start", handlers.StartCampaignHandler(database)).Methods("POST")
	api.HandleFunc("/campaigns/{id}/pause", handlers.PauseCampaignHandler(database, dispatcher)).Methods("POST")

	// Start server
	srv := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: r,
		// No read/write timeouts - WebSocket connections need to stay open
		IdleTimeout: 120 * time.Second,
	}

	go func() {
		log.Printf("server starting on :%s", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("server forced to shutdown: %v", err)
	}

	log.Println("server stopped")
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func reconcileDevices(database *sql.DB, dockerCli *client.Client) {
	log.Println("reconciling device states with Docker...")

	devices, err := db.GetAllActiveDevices(database)
	if err != nil {
		log.Printf("error fetching active devices for reconciliation: %v", err)
		return
	}

	for _, device := range devices {
		if device.ContainerID == "" {
			// No container yet, mark as error if stuck
			if device.Status == models.StatusCreating {
				log.Printf("device %s stuck in creating state, marking as error", device.ID)
				db.UpdateDeviceStatus(database, device.ID, models.StatusError, "stuck in creating state on restart")
			}
			continue
		}

		exists, err := dkr.ContainerExists(dockerCli, device.ContainerID)
		if err != nil {
			log.Printf("error checking container %s for device %s: %v", device.ContainerID, device.ID, err)
			continue
		}

		if !exists {
			log.Printf("container %s for device %s no longer exists, marking as error", device.ContainerID, device.ID)
			db.UpdateDeviceStatus(database, device.ID, models.StatusError, "container not found on restart")
			continue
		}

		running, err := dkr.IsContainerRunning(dockerCli, device.ContainerID)
		if err != nil {
			log.Printf("error checking if container %s is running: %v", device.ContainerID, err)
			continue
		}

		if !running {
			log.Printf("container %s for device %s is not running, marking as stopped", device.ContainerID, device.ID)
			db.UpdateDeviceStatus(database, device.ID, models.StatusStopped, "")
		} else if device.Status == models.StatusCreating || device.Status == models.StatusBooting || device.Status == models.StatusInstalling {
			// Container is running but was in a transitional state. Mark as ready since we can't resume provisioning.
			log.Printf("device %s was in %s state with running container, marking as ready", device.ID, device.Status)
			db.UpdateDeviceStatus(database, device.ID, models.StatusReady, "")
		}
	}

	log.Println("device reconciliation complete")
}
