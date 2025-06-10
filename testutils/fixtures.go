package testutils

import (
	"log"
	"os"
	"testing"
	"time"

	"github.com/fbettag/unifi-gate-opener/internal/auth"
	"github.com/fbettag/unifi-gate-opener/internal/config"
	"github.com/fbettag/unifi-gate-opener/internal/database"
	"github.com/fbettag/unifi-gate-opener/internal/handlers"
	"github.com/sirupsen/logrus"
)

// TestApp holds test application context
type TestApp struct {
	App     *handlers.App
	Config  *config.Config
	Cleanup func()
}

// NewTestApp creates a new test application instance
func NewTestApp(t *testing.T) *TestApp {
	// Create unique test files
	configFile := "test_config_" + time.Now().Format("20060102150405") + ".yaml"
	dbFile := "test_db_" + time.Now().Format("20060102150405") + ".db"

	// Set up logger with test level
	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel) // Reduce noise in tests
	logger.SetFormatter(&logrus.TextFormatter{
		DisableColors:    true,
		DisableTimestamp: true,
	})

	// Create test config
	cfg := &config.Config{
		DatabasePath:  dbFile,
		SessionSecret: "test-session-secret-32-characters!",
		SetupComplete: false,
	}

	// Initialize database
	db, err := database.Initialize(cfg.DatabasePath)
	if err != nil {
		t.Fatalf("Failed to initialize test database: %v", err)
	}

	// Initialize session store
	sessionStore := auth.NewSessionStore(cfg.SessionSecret)

	// Create app context
	app := &handlers.App{
		Config:       cfg,
		DB:           db,
		Logger:       logger,
		SessionStore: sessionStore,
		// WebFS will be set by tests that need it
	}

	cleanup := func() {
		if db != nil {
			db.Close()
		}
		os.Remove(configFile)
		os.Remove(dbFile)
	}

	return &TestApp{
		App:     app,
		Config:  cfg,
		Cleanup: cleanup,
	}
}

// CompleteSetup sets up the app as if setup wizard was completed
func (ta *TestApp) CompleteSetup(unifiURL, username, password string) {
	ta.Config.SetupComplete = true
	ta.Config.Admin.Username = "admin"
	if err := ta.Config.SetAdminPassword("testpassword123"); err != nil {
		log.Fatalf("Failed to set admin password: %v", err)
	}

	ta.Config.UniFi.ControllerURL = unifiURL
	ta.Config.UniFi.Username = username
	ta.Config.UniFi.Password = password
	ta.Config.UniFi.SiteID = "default"
	ta.Config.UniFi.GateAPMAC = "aa:bb:cc:dd:ee:ff"
	ta.Config.UniFi.PollInterval = 1

	ta.Config.Shelly.TriggerURL = "http://test-shelly/relay/0?turn=on&timer=5"
	ta.Config.Gate.OpenDuration = 5
}

// CreateTestDevice adds a test device to the database
func (ta *TestApp) CreateTestDevice(mac, name string, enabled bool) error {
	_, err := ta.App.DB.Exec(`
		INSERT INTO devices (mac, name, enabled, created_at, updated_at)
		VALUES (?, ?, ?, datetime('now'), datetime('now'))
	`, mac, name, enabled)
	return err
}

// GetValidTestMAC returns a valid MAC address for testing
func GetValidTestMAC() string {
	return "aa:bb:cc:dd:ee:01"
}

// GetValidTestMACUppercase returns a valid uppercase MAC address for testing
func GetValidTestMACUppercase() string {
	return "AA:BB:CC:DD:EE:01"
}

// GetInvalidTestMACs returns various invalid MAC addresses for testing
func GetInvalidTestMACs() []string {
	return []string{
		"invalid-mac",
		"aa:bb:cc:dd:ee",       // too short
		"aa:bb:cc:dd:ee:ff:gg", // too long
		"zz:bb:cc:dd:ee:ff",    // invalid hex
		"",                     // empty
	}
}
