package main

import (
	"embed"
	"flag"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/fbettag/unifi-gate-opener/internal/auth"
	"github.com/fbettag/unifi-gate-opener/internal/config"
	"github.com/fbettag/unifi-gate-opener/internal/database"
	"github.com/fbettag/unifi-gate-opener/internal/handlers"
	"github.com/fbettag/unifi-gate-opener/internal/unifi"
	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
)

var (
	//go:embed all:web
	webFiles embed.FS
)

var (
	Version = "dev" // Set by build process
)

var (
	configFile  = flag.String("config", "config.yaml", "Path to configuration file")
	port        = flag.Int("port", 8080, "Port to run the web server on")
	dbPath      = flag.String("database", "", "Path to database file (overrides config)")
	logLevel    = flag.String("log-level", "info", "Log level (debug, info, warn, error)")
	showVersion = flag.Bool("version", false, "Show version and exit")
)

func main() {
	flag.Parse()

	// Handle version flag
	if *showVersion {
		fmt.Printf("UniFi Gate Opener %s\n", Version)
		fmt.Println("Built with ❤️ for reliable gate automation")
		os.Exit(0)
	}

	// Initialize logger
	logger := logrus.New()
	logger.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
	})

	// Set log level from flag
	switch *logLevel {
	case "debug":
		logger.SetLevel(logrus.DebugLevel)
	case "info":
		logger.SetLevel(logrus.InfoLevel)
	case "warn":
		logger.SetLevel(logrus.WarnLevel)
	case "error":
		logger.SetLevel(logrus.ErrorLevel)
	default:
		logger.SetLevel(logrus.InfoLevel)
	}

	logger.Infof("Starting UniFi Gate Opener %s", Version)

	// Load or initialize configuration
	cfg, err := config.LoadOrInitialize(*configFile)
	if err != nil {
		logger.Fatalf("Failed to load configuration: %v", err)
	}

	// Override database path if provided via flag
	databasePath := cfg.DatabasePath
	if *dbPath != "" {
		databasePath = *dbPath
		logger.Infof("Using database path from command line: %s", databasePath)
	}

	// Initialize database
	db, err := database.Initialize(databasePath)
	if err != nil {
		logger.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	// Initialize session store
	sessionStore := auth.NewSessionStore(cfg.SessionSecret)

	// Create app context
	app := &handlers.App{
		Config:       cfg,
		DB:           db,
		Logger:       logger,
		WebFS:        webFiles,
		SessionStore: sessionStore,
	}

	// Initialize UniFi client if configured
	if cfg.IsConfigured() {
		unifiLogger := unifi.NewLogrusAdapter(logger)
		unifiClient := unifi.NewClient(cfg.UniFi.ControllerURL, cfg.UniFi.Username, cfg.UniFi.Password, unifiLogger)
		app.UniFiClient = unifiClient

		// Start monitoring in background
		go app.StartMonitoring()
	}

	// Setup routes
	router := setupRoutes(app)

	// Start server
	addr := fmt.Sprintf(":%d", *port)
	logger.Infof("Starting server on http://localhost%s", addr)

	// Handle graceful shutdown
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		logger.Info("Shutting down...")
		os.Exit(0)
	}()

	// Create server with timeouts
	server := &http.Server{
		Addr:         addr,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	if err := server.ListenAndServe(); err != nil {
		logger.Fatalf("Failed to start server: %v", err)
	}
}

func setupRoutes(app *handlers.App) *mux.Router {
	router := mux.NewRouter()

	// Check if setup is complete middleware (must be first)
	router.Use(app.CheckSetupMiddleware)

	// Static files
	staticFS, _ := fs.Sub(webFiles, "web/static")
	router.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.FS(staticFS))))

	// Setup wizard routes (no auth required)
	router.HandleFunc("/setup", app.SetupWizardHandler).Methods("GET")
	router.HandleFunc("/api/setup", app.SetupAPIHandler).Methods("POST")
	router.HandleFunc("/api/test-unifi", app.TestUniFiHandler).Methods("POST")
	router.HandleFunc("/api/test-unifi-sites", app.TestUniFiSitesHandler).Methods("POST")

	// Public routes
	router.HandleFunc("/", app.IndexHandler).Methods("GET")
	router.HandleFunc("/login", app.LoginPageHandler).Methods("GET")
	router.HandleFunc("/api/login", app.LoginHandler).Methods("POST")

	// Protected routes (require authentication)
	protected := router.PathPrefix("/").Subrouter()
	protected.Use(app.AuthMiddleware)

	// Dashboard
	protected.HandleFunc("/dashboard", app.DashboardHandler).Methods("GET")
	protected.HandleFunc("/logout", app.LogoutHandler).Methods("GET", "POST")

	// API routes
	api := protected.PathPrefix("/api").Subrouter()
	api.HandleFunc("/devices", app.GetDevicesHandler).Methods("GET")
	api.HandleFunc("/devices", app.AddDeviceHandler).Methods("POST")
	api.HandleFunc("/devices/{id}", app.UpdateDeviceHandler).Methods("PUT")
	api.HandleFunc("/devices/{id}", app.DeleteDeviceHandler).Methods("DELETE")

	api.HandleFunc("/settings", app.GetSettingsHandler).Methods("GET")
	api.HandleFunc("/settings", app.UpdateSettingsHandler).Methods("PUT")

	api.HandleFunc("/logs", app.GetLogsHandler).Methods("GET")
	api.HandleFunc("/status", app.GetStatusHandler).Methods("GET")

	api.HandleFunc("/unifi/aps", app.GetAccessPointsHandler).Methods("GET")
	api.HandleFunc("/unifi/clients", app.GetUniFiClientsHandler).Methods("GET")
	api.HandleFunc("/test-gate", app.TestGateHandler).Methods("POST")

	return router
}
