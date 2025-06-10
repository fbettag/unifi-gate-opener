package handlers

import (
	"embed"
	"html/template"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/fbettag/unifi-gate-opener/internal/auth"
	"github.com/fbettag/unifi-gate-opener/internal/config"
	"github.com/fbettag/unifi-gate-opener/internal/database"
	"github.com/fbettag/unifi-gate-opener/internal/gate"
	"github.com/fbettag/unifi-gate-opener/internal/unifi"
	"github.com/sirupsen/logrus"
)

const (
	directionArriving = "arriving"
	directionLeaving  = "leaving"
	directionUnknown  = "unknown"
)

type App struct {
	Config         *config.Config
	DB             *database.DB
	Logger         *logrus.Logger
	WebFS          embed.FS
	SessionStore   *auth.SessionStore
	UniFiClient    *unifi.Client
	GateController *gate.Controller

	// Monitoring state
	monitoringMu   sync.RWMutex
	isMonitoring   bool
	stopMonitoring chan bool
	deviceStates   map[string]*DeviceState
}

type DeviceState struct {
	MAC             string
	Name            string
	CurrentAP       string
	PreviousAP      string
	LastSeen        time.Time
	IsConnected     bool
	LastGateTrigger time.Time
}

func (app *App) StartMonitoring() {
	app.monitoringMu.Lock()
	if app.isMonitoring {
		app.monitoringMu.Unlock()
		return
	}

	app.isMonitoring = true
	app.stopMonitoring = make(chan bool)
	app.deviceStates = make(map[string]*DeviceState)
	app.monitoringMu.Unlock()

	// Initialize gate controller
	app.GateController = gate.NewController(app.Config.Shelly.TriggerURL, app.Logger)

	// Load initial device states from database
	app.loadDeviceStates()

	app.Logger.Info("Starting device monitoring")

	// Start the cleanup job
	go app.startCleanupJob()

	ticker := time.NewTicker(time.Duration(app.Config.UniFi.PollInterval) * time.Second)
	defer ticker.Stop()

	// Initial poll
	app.pollUniFi()

	for {
		select {
		case <-ticker.C:
			app.pollUniFi()
		case <-app.stopMonitoring:
			app.Logger.Info("Stopping device monitoring")
			return
		}
	}
}

func (app *App) StopMonitoring() {
	app.monitoringMu.Lock()
	defer app.monitoringMu.Unlock()

	if app.isMonitoring {
		close(app.stopMonitoring)
		app.isMonitoring = false
	}
}

func (app *App) loadDeviceStates() {
	for _, device := range app.Config.Devices {
		if !device.Enabled {
			continue
		}

		currentAP, lastSeen, isConnected, err := app.DB.GetDeviceState(device.MAC)
		if err != nil {
			app.Logger.Errorf("Failed to load state for device %s: %v", device.MAC, err)
			continue
		}

		lastTrigger, _ := app.DB.GetLastGateTrigger(device.MAC)

		normalizedMAC := strings.ToUpper(device.MAC)
		app.deviceStates[normalizedMAC] = &DeviceState{
			MAC:             normalizedMAC,
			Name:            device.Name,
			CurrentAP:       currentAP,
			LastSeen:        lastSeen,
			IsConnected:     isConnected,
			LastGateTrigger: lastTrigger,
		}
	}
}

func (app *App) pollUniFi() {

	// Ensure we're logged in
	if app.UniFiClient == nil {
		app.Logger.Error("UniFi client not initialized")
		return
	}

	clients, err := app.UniFiClient.GetActiveClients(app.Config.UniFi.SiteID)
	if err != nil {
		app.Logger.Errorf("Failed to get active clients: %v", err)
		// Try to re-login if needed
		if err.Error() == "not logged in" {
			app.Logger.Info("Attempting to re-login to UniFi")
			if loginErr := app.UniFiClient.Login(); loginErr != nil {
				app.Logger.Errorf("Re-login failed: %v", loginErr)
				return
			}
			// Retry getting clients
			clients, err = app.UniFiClient.GetActiveClients(app.Config.UniFi.SiteID)
			if err != nil {
				app.Logger.Errorf("Failed to get active clients after re-login: %v", err)
				return
			}
		} else {
			return
		}
	}

	// Create a map of currently connected devices (normalize MAC addresses to uppercase)
	currentlyConnected := make(map[string]*unifi.WirelessClient)
	for i := range clients {
		normalizedMAC := strings.ToUpper(clients[i].MAC)
		currentlyConnected[normalizedMAC] = &clients[i]
	}

	app.monitoringMu.Lock()
	defer app.monitoringMu.Unlock()

	// Check each tracked device
	for mac, state := range app.deviceStates {
		client, isNowConnected := currentlyConnected[mac]

		if isNowConnected {
			// Device is connected
			newAP := client.AP_MAC

			if !state.IsConnected {
				// Device just connected
				// Check if it's a fresh connection to gate AP (not already sitting there)
				if newAP == app.Config.UniFi.GateAPMAC && client.Uptime < 30 {
					app.Logger.Infof("Device %s newly arrived at gate (uptime: %ds)", state.Name, client.Uptime)
					app.handleDeviceConnected(state, newAP, "nowhere")
				} else if newAP != app.Config.UniFi.GateAPMAC {
					// Connected to non-gate AP
					app.handleDeviceConnected(state, newAP, "nowhere")
				} else {
					// Already at gate for more than 30 seconds, just update state
					app.Logger.Infof("Device %s already at gate (uptime: %ds), not triggering", state.Name, client.Uptime)
				}
			} else if state.CurrentAP != newAP {
				// Device roamed to different AP
				app.handleDeviceRoamed(state, state.CurrentAP, newAP)
			}

			// Update state
			state.CurrentAP = newAP
			state.IsConnected = true
			state.LastSeen = time.Now()

			// Update database
			if err := app.DB.UpdateDeviceState(mac, newAP, true); err != nil {
				app.Logger.Errorf("Failed to update device state for %s: %v", mac, err)
			}

		} else if state.IsConnected {
			// Device disconnected
			app.handleDeviceDisconnected(state)

			// Update state
			state.PreviousAP = state.CurrentAP
			state.CurrentAP = ""
			state.IsConnected = false

			// Update database
			if err := app.DB.UpdateDeviceState(mac, "", false); err != nil {
				app.Logger.Errorf("Failed to update device state for %s: %v", mac, err)
			}
		}
	}
}

func (app *App) handleDeviceConnected(state *DeviceState, toAP, fromAP string) {
	direction := directionUnknown
	if toAP == app.Config.UniFi.GateAPMAC {
		direction = directionArriving
	}

	app.Logger.Infof("Device %s (%s) connected to AP %s", state.Name, state.MAC, toAP)

	// Log event if activity logging is enabled
	if app.Config.Gate.LogActivity {
		if err := app.DB.LogEvent(&database.LogEntry{
			DeviceMAC:  state.MAC,
			DeviceName: state.Name,
			Event:      "connected",
			Direction:  direction,
			FromAP:     fromAP,
			ToAP:       toAP,
			Message:    "Device connected to network",
		}); err != nil {
			app.Logger.Errorf("Failed to log event for %s: %v", state.MAC, err)
		}
	}

	// Check if we should open gate
	if toAP == app.Config.UniFi.GateAPMAC {
		app.checkAndOpenGate(state, direction)
	}
}

func (app *App) handleDeviceRoamed(state *DeviceState, fromAP, toAP string) {
	direction := directionUnknown

	// Determine direction based on AP movement
	if toAP == app.Config.UniFi.GateAPMAC {
		if fromAP != "" {
			direction = directionLeaving // Moving from inside to gate
		} else {
			direction = directionArriving // Connecting at gate
		}
	} else if fromAP == app.Config.UniFi.GateAPMAC {
		direction = directionArriving // Moving from gate to inside
	}

	app.Logger.Infof("Device %s (%s) roamed from AP %s to AP %s (direction: %s)",
		state.Name, state.MAC, fromAP, toAP, direction)

	// Log event
	if app.Config.Gate.LogActivity {
		if err := app.DB.LogEvent(&database.LogEntry{
			DeviceMAC:  state.MAC,
			DeviceName: state.Name,
			Event:      "roamed",
			Direction:  direction,
			FromAP:     fromAP,
			ToAP:       toAP,
			Message:    "Device roamed between access points",
		}); err != nil {
			app.Logger.Errorf("Failed to log event for %s: %v", state.MAC, err)
		}
	}

	// Check if we should open gate
	if toAP == app.Config.UniFi.GateAPMAC || fromAP == app.Config.UniFi.GateAPMAC {
		app.checkAndOpenGate(state, direction)
	}
}

func (app *App) handleDeviceDisconnected(state *DeviceState) {
	app.Logger.Infof("Device %s (%s) disconnected from AP %s", state.Name, state.MAC, state.CurrentAP)

	// Log event
	if app.Config.Gate.LogActivity {
		if err := app.DB.LogEvent(&database.LogEntry{
			DeviceMAC:  state.MAC,
			DeviceName: state.Name,
			Event:      "disconnected",
			FromAP:     state.CurrentAP,
			Message:    "Device disconnected from network",
		}); err != nil {
			app.Logger.Errorf("Failed to log event for %s: %v", state.MAC, err)
		}
	}
}

func (app *App) checkAndOpenGate(state *DeviceState, direction string) {
	// Check cooldown period (using open duration)
	cooldownDuration := time.Duration(app.Config.Gate.OpenDuration) * time.Minute
	if time.Since(state.LastGateTrigger) < cooldownDuration {
		app.Logger.Infof("Gate recently opened for %s, skipping (cooldown: %v remaining)",
			state.Name, cooldownDuration-time.Since(state.LastGateTrigger))

		if app.Config.Gate.LogActivity {
			if err := app.DB.LogEvent(&database.LogEntry{
				DeviceMAC:  state.MAC,
				DeviceName: state.Name,
				Event:      "gate_skipped",
				Direction:  direction,
				GateOpened: false,
				Message:    "Gate recently opened, cooldown active",
			}); err != nil {
				app.Logger.Errorf("Failed to log event for %s: %v", state.MAC, err)
			}
		}
		return
	}

	// Open gate
	app.Logger.Infof("Opening gate for %s (%s)", state.Name, direction)

	if err := app.GateController.OpenGate(); err != nil {
		app.Logger.Errorf("Failed to open gate: %v", err)

		if app.Config.Gate.LogActivity {
			if logErr := app.DB.LogEvent(&database.LogEntry{
				DeviceMAC:  state.MAC,
				DeviceName: state.Name,
				Event:      "gate_error",
				Direction:  direction,
				GateOpened: false,
				Message:    err.Error(),
			}); logErr != nil {
				app.Logger.Errorf("Failed to log event for %s: %v", state.MAC, logErr)
			}
		}
		return
	}

	// Update last trigger time
	state.LastGateTrigger = time.Now()
	if err := app.DB.UpdateLastGateTrigger(state.MAC); err != nil {
		app.Logger.Errorf("Failed to update last gate trigger for %s: %v", state.MAC, err)
	}

	// Log successful gate opening
	if app.Config.Gate.LogActivity {
		if err := app.DB.LogEvent(&database.LogEntry{
			DeviceMAC:  state.MAC,
			DeviceName: state.Name,
			Event:      "gate_triggered",
			Direction:  direction,
			GateOpened: true,
			Message:    "Gate opened successfully",
		}); err != nil {
			app.Logger.Errorf("Failed to log event for %s: %v", state.MAC, err)
		}
	}
}

// Template helper functions
func (app *App) loadTemplate(name string) (*template.Template, error) {
	return template.ParseFS(app.WebFS, "web/templates/base.html", "web/templates/"+name)
}

func (app *App) renderTemplate(w http.ResponseWriter, name string, data interface{}) {
	tmpl, err := app.loadTemplate(name)
	if err != nil {
		app.Logger.Errorf("Failed to load template %s: %v", name, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	if err := tmpl.Execute(w, data); err != nil {
		app.Logger.Errorf("Failed to execute template %s: %v", name, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// startCleanupJob runs a background job to clean up old logs every hour
func (app *App) startCleanupJob() {
	app.Logger.Info("Starting log cleanup job (runs every hour)")

	// Run cleanup every hour
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	// Run initial cleanup
	app.cleanupOldLogs()

	for {
		select {
		case <-ticker.C:
			app.cleanupOldLogs()
		case <-app.stopMonitoring:
			app.Logger.Info("Stopping log cleanup job")
			return
		}
	}
}

func (app *App) cleanupOldLogs() {
	// Delete logs older than 30 days
	deletedCount, err := app.DB.DeleteOldLogs(30)
	if err != nil {
		app.Logger.Errorf("Failed to delete old logs: %v", err)
		return
	}

	if deletedCount > 0 {
		app.Logger.Infof("Deleted %d old log entries (>30 days)", deletedCount)
	}
}
