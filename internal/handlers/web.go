package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/fbettag/unifi-gate-opener/internal/config"
	"github.com/fbettag/unifi-gate-opener/internal/database"
	"github.com/fbettag/unifi-gate-opener/internal/gate"
	"github.com/fbettag/unifi-gate-opener/internal/unifi"
	"github.com/gorilla/mux"
)

// Middleware to check if setup is complete
func (app *App) CheckSetupMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Always allow static files and login page
		if (len(r.URL.Path) > 7 && r.URL.Path[:7] == "/static") ||
			r.URL.Path == "/login" ||
			r.URL.Path == "/api/login" {
			next.ServeHTTP(w, r)
			return
		}

		// If setup is already complete
		if app.Config.IsConfigured() {
			// Block access to setup pages
			if r.URL.Path == "/setup" ||
				r.URL.Path == "/api/setup" ||
				r.URL.Path == "/api/test-unifi" ||
				r.URL.Path == "/api/test-unifi-sites" {
				// Redirect to home page which will then redirect to login/dashboard
				http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
				return
			}
		} else {
			// Setup not complete
			// Allow setup routes and test endpoints
			if r.URL.Path == "/setup" ||
				r.URL.Path == "/api/setup" ||
				r.URL.Path == "/api/test-unifi" ||
				r.URL.Path == "/api/test-unifi-sites" {
				next.ServeHTTP(w, r)
				return
			}
			// For all other routes, redirect to setup
			http.Redirect(w, r, "/setup", http.StatusTemporaryRedirect)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// Middleware to check authentication
func (app *App) AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !app.SessionStore.IsAuthenticated(r) {
			if r.URL.Path[:4] == "/api" {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
			} else {
				http.Redirect(w, r, "/login", http.StatusTemporaryRedirect)
			}
			return
		}
		next.ServeHTTP(w, r)
	})
}

// Index handler - redirects to appropriate page
func (app *App) IndexHandler(w http.ResponseWriter, r *http.Request) {
	if !app.Config.IsConfigured() {
		http.Redirect(w, r, "/setup", http.StatusTemporaryRedirect)
		return
	}

	if app.SessionStore.IsAuthenticated(r) {
		http.Redirect(w, r, "/dashboard", http.StatusTemporaryRedirect)
		return
	}

	http.Redirect(w, r, "/login", http.StatusTemporaryRedirect)
}

// Setup wizard page
func (app *App) SetupWizardHandler(w http.ResponseWriter, r *http.Request) {
	if app.Config.IsConfigured() {
		http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
		return
	}

	app.renderTemplate(w, "setup.html", nil)
}

// Setup API endpoint
func (app *App) SetupAPIHandler(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Admin struct {
			Username string `json:"username"`
			Password string `json:"password"`
		} `json:"admin"`
		UniFi struct {
			ControllerURL string `json:"controller_url"`
			Username      string `json:"username"`
			Password      string `json:"password"`
			SiteID        string `json:"site_id"`
			GateAPMAC     string `json:"gate_ap_mac"`
		} `json:"unifi"`
		Shelly struct {
			TriggerURL string `json:"trigger_url"`
		} `json:"shelly"`
		Gate struct {
			OpenDuration int `json:"open_duration"`
		} `json:"gate"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	// Update configuration
	app.Config.Admin.Username = req.Admin.Username
	if err := app.Config.SetAdminPassword(req.Admin.Password); err != nil {
		http.Error(w, "Failed to set password", http.StatusInternalServerError)
		return
	}

	app.Config.UniFi.ControllerURL = req.UniFi.ControllerURL
	app.Config.UniFi.Username = req.UniFi.Username
	app.Config.UniFi.Password = req.UniFi.Password
	app.Config.UniFi.SiteID = req.UniFi.SiteID
	app.Config.UniFi.GateAPMAC = req.UniFi.GateAPMAC
	app.Config.UniFi.PollInterval = 1 // Default to 1 second

	app.Config.Shelly.TriggerURL = req.Shelly.TriggerURL
	app.Config.Gate.OpenDuration = req.Gate.OpenDuration

	app.Config.SetupComplete = true

	// Save configuration
	if err := config.SaveConfig("config.yaml", app.Config); err != nil {
		http.Error(w, "Failed to save configuration", http.StatusInternalServerError)
		return
	}

	// Initialize UniFi client
	unifiLogger := unifi.NewLogrusAdapter(app.Logger)
	app.UniFiClient = unifi.NewClient(
		app.Config.UniFi.ControllerURL,
		app.Config.UniFi.Username,
		app.Config.UniFi.Password,
		unifiLogger,
	)

	// Login to UniFi
	if err := app.UniFiClient.Login(); err != nil {
		app.Logger.Errorf("Failed to login to UniFi after setup: %v", err)
		// Don't fail setup, monitoring will retry
	}

	// Start monitoring
	go app.StartMonitoring()

	// Log in the user
	if err := app.SessionStore.Login(r, w); err != nil {
		app.Logger.Errorf("Failed to create session after setup: %v", err)
		// Don't fail setup, continue anyway
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]bool{"success": true}); err != nil {
		app.Logger.Errorf("Failed to encode response: %v", err)
	}
}

// Login page
func (app *App) LoginPageHandler(w http.ResponseWriter, r *http.Request) {
	if app.SessionStore.IsAuthenticated(r) {
		http.Redirect(w, r, "/dashboard", http.StatusTemporaryRedirect)
		return
	}

	app.renderTemplate(w, "login.html", nil)
}

// Login API endpoint
func (app *App) LoginHandler(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	// Verify credentials
	if req.Username != app.Config.Admin.Username ||
		!app.Config.VerifyAdminPassword(req.Password) {
		http.Error(w, "Invalid credentials", http.StatusUnauthorized)
		return
	}

	// Create session
	if err := app.SessionStore.Login(r, w); err != nil {
		http.Error(w, "Failed to create session", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]bool{"success": true}); err != nil {
		app.Logger.Errorf("Failed to encode response: %v", err)
	}
}

// Logout handler
func (app *App) LogoutHandler(w http.ResponseWriter, r *http.Request) {
	if err := app.SessionStore.Logout(r, w); err != nil {
		app.Logger.Errorf("Failed to logout: %v", err)
		// Continue with redirect anyway
	}
	http.Redirect(w, r, "/login", http.StatusTemporaryRedirect)
}

// Dashboard page
func (app *App) DashboardHandler(w http.ResponseWriter, r *http.Request) {
	// Get recent activity
	logs, _ := app.DB.GetRecentActivity(24)

	// Get connected devices
	connectedDevices, _ := app.DB.GetConnectedDevices()

	data := struct {
		Config           *config.Config
		RecentActivity   []database.LogEntry
		ConnectedDevices map[string]bool
		IsMonitoring     bool
	}{
		Config:           app.Config,
		RecentActivity:   logs,
		ConnectedDevices: connectedDevices,
		IsMonitoring:     app.isMonitoring,
	}

	app.renderTemplate(w, "dashboard.html", data)
}

// Get devices API
func (app *App) GetDevicesHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(app.Config.Devices); err != nil {
		app.Logger.Errorf("Failed to encode devices: %v", err)
	}
}

// Add device API
func (app *App) AddDeviceHandler(w http.ResponseWriter, r *http.Request) {
	var req struct {
		MAC  string `json:"mac"`
		Name string `json:"name"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	if err := app.Config.AddDevice(req.MAC, req.Name); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Save configuration
	if err := config.SaveConfig("config.yaml", app.Config); err != nil {
		http.Error(w, "Failed to save configuration", http.StatusInternalServerError)
		return
	}

	// Add to monitoring if active
	app.monitoringMu.Lock()
	if app.isMonitoring {
		normalizedMAC := strings.ToUpper(req.MAC)
		app.deviceStates[normalizedMAC] = &DeviceState{
			MAC:  normalizedMAC,
			Name: req.Name,
		}
	}
	app.monitoringMu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]bool{"success": true}); err != nil {
		app.Logger.Errorf("Failed to encode response: %v", err)
	}
}

// Update device API
func (app *App) UpdateDeviceHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	mac := vars["id"]

	var req struct {
		Name    string `json:"name"`
		Enabled bool   `json:"enabled"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	if err := app.Config.UpdateDevice(mac, req.Name, req.Enabled); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	// Save configuration
	if err := config.SaveConfig("config.yaml", app.Config); err != nil {
		http.Error(w, "Failed to save configuration", http.StatusInternalServerError)
		return
	}

	// Update monitoring state
	app.monitoringMu.Lock()
	if state, exists := app.deviceStates[mac]; exists {
		state.Name = req.Name
		if !req.Enabled {
			delete(app.deviceStates, mac)
		}
	} else if req.Enabled && app.isMonitoring {
		app.deviceStates[mac] = &DeviceState{
			MAC:  mac,
			Name: req.Name,
		}
	}
	app.monitoringMu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]bool{"success": true}); err != nil {
		app.Logger.Errorf("Failed to encode response: %v", err)
	}
}

// Delete device API
func (app *App) DeleteDeviceHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	mac := vars["id"]

	if err := app.Config.RemoveDevice(mac); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	// Save configuration
	if err := config.SaveConfig("config.yaml", app.Config); err != nil {
		http.Error(w, "Failed to save configuration", http.StatusInternalServerError)
		return
	}

	// Remove from monitoring
	app.monitoringMu.Lock()
	delete(app.deviceStates, mac)
	app.monitoringMu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]bool{"success": true}); err != nil {
		app.Logger.Errorf("Failed to encode response: %v", err)
	}
}

// Get settings API
func (app *App) GetSettingsHandler(w http.ResponseWriter, r *http.Request) {
	settings := map[string]interface{}{
		"unifi": map[string]interface{}{
			"controller_url": app.Config.UniFi.ControllerURL,
			"username":       app.Config.UniFi.Username,
			"site_id":        app.Config.UniFi.SiteID,
			"gate_ap_mac":    app.Config.UniFi.GateAPMAC,
			"poll_interval":  app.Config.UniFi.PollInterval,
		},
		"shelly": map[string]interface{}{
			"trigger_url": app.Config.Shelly.TriggerURL,
		},
		"gate": map[string]interface{}{
			"open_duration": app.Config.Gate.OpenDuration,
			"log_activity":  app.Config.Gate.LogActivity,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(settings); err != nil {
		app.Logger.Errorf("Failed to encode settings: %v", err)
	}
}

// Update settings API
func (app *App) UpdateSettingsHandler(w http.ResponseWriter, r *http.Request) {
	var req struct {
		UniFi struct {
			ControllerURL string `json:"controller_url"`
			Username      string `json:"username"`
			Password      string `json:"password,omitempty"`
			SiteID        string `json:"site_id"`
			GateAPMAC     string `json:"gate_ap_mac"`
			PollInterval  int    `json:"poll_interval"`
		} `json:"unifi"`
		Shelly struct {
			TriggerURL string `json:"trigger_url"`
		} `json:"shelly"`
		Gate struct {
			OpenDuration int  `json:"open_duration"`
			LogActivity  bool `json:"log_activity"`
		} `json:"gate"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	// Update configuration
	app.Config.UniFi.ControllerURL = req.UniFi.ControllerURL
	app.Config.UniFi.Username = req.UniFi.Username
	if req.UniFi.Password != "" {
		app.Config.UniFi.Password = req.UniFi.Password
	}
	app.Config.UniFi.SiteID = req.UniFi.SiteID
	app.Config.UniFi.GateAPMAC = req.UniFi.GateAPMAC
	app.Config.UniFi.PollInterval = req.UniFi.PollInterval

	app.Config.Shelly.TriggerURL = req.Shelly.TriggerURL
	app.Config.Gate.OpenDuration = req.Gate.OpenDuration
	app.Config.Gate.LogActivity = req.Gate.LogActivity

	// Save configuration
	if err := config.SaveConfig("config.yaml", app.Config); err != nil {
		http.Error(w, "Failed to save configuration", http.StatusInternalServerError)
		return
	}

	// Update gate controller URL
	if app.GateController != nil {
		app.GateController.UpdateURL(req.Shelly.TriggerURL)
	}

	// Restart monitoring if UniFi settings changed
	if app.isMonitoring {
		app.StopMonitoring()
		unifiLogger := unifi.NewLogrusAdapter(app.Logger)
		app.UniFiClient = unifi.NewClient(
			app.Config.UniFi.ControllerURL,
			app.Config.UniFi.Username,
			app.Config.UniFi.Password,
			unifiLogger,
		)
		go app.StartMonitoring()
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]bool{"success": true}); err != nil {
		app.Logger.Errorf("Failed to encode response: %v", err)
	}
}

// Get logs API
func (app *App) GetLogsHandler(w http.ResponseWriter, r *http.Request) {
	limit := 100
	offset := 0

	if l := r.URL.Query().Get("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil {
			limit = v
		}
	}

	if o := r.URL.Query().Get("offset"); o != "" {
		if v, err := strconv.Atoi(o); err == nil {
			offset = v
		}
	}

	logs, err := app.DB.GetLogs(limit, offset)
	if err != nil {
		http.Error(w, "Failed to get logs", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(logs); err != nil {
		app.Logger.Errorf("Failed to encode logs: %v", err)
	}
}

// Get status API
func (app *App) GetStatusHandler(w http.ResponseWriter, r *http.Request) {
	app.monitoringMu.RLock()
	deviceStates := make(map[string]interface{})
	for mac, state := range app.deviceStates {
		deviceStates[mac] = map[string]interface{}{
			"name":         state.Name,
			"current_ap":   state.CurrentAP,
			"is_connected": state.IsConnected,
			"last_seen":    state.LastSeen,
		}
	}
	app.monitoringMu.RUnlock()

	status := map[string]interface{}{
		"is_monitoring": app.isMonitoring,
		"devices":       deviceStates,
		"config": map[string]interface{}{
			"gate_ap_mac":   app.Config.UniFi.GateAPMAC,
			"poll_interval": app.Config.UniFi.PollInterval,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(status); err != nil {
		app.Logger.Errorf("Failed to encode status: %v", err)
	}
}

// Get UniFi access points API
func (app *App) GetAccessPointsHandler(w http.ResponseWriter, r *http.Request) {
	if app.UniFiClient == nil {
		http.Error(w, "UniFi not configured", http.StatusBadRequest)
		return
	}

	// Ensure we're logged in
	if err := app.UniFiClient.Login(); err != nil {
		http.Error(w, "Failed to connect to UniFi", http.StatusInternalServerError)
		return
	}

	aps, err := app.UniFiClient.GetAccessPoints(app.Config.UniFi.SiteID)
	if err != nil {
		http.Error(w, "Failed to get access points", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(aps); err != nil {
		app.Logger.Errorf("Failed to encode access points: %v", err)
	}
}

// Get all UniFi clients API
func (app *App) GetUniFiClientsHandler(w http.ResponseWriter, r *http.Request) {
	if app.UniFiClient == nil {
		http.Error(w, "UniFi not configured", http.StatusBadRequest)
		return
	}

	// Ensure we're logged in
	if err := app.UniFiClient.Login(); err != nil {
		http.Error(w, "Failed to connect to UniFi", http.StatusInternalServerError)
		return
	}

	clients, err := app.UniFiClient.GetActiveClients(app.Config.UniFi.SiteID)
	if err != nil {
		http.Error(w, "Failed to get clients", http.StatusInternalServerError)
		return
	}

	// Format clients for frontend
	formattedClients := make([]map[string]interface{}, len(clients))
	for i, client := range clients {
		formattedClients[i] = map[string]interface{}{
			"mac":      client.MAC,
			"name":     client.Name,
			"hostname": client.Hostname,
			"ip":       client.IP,
			"ap":       client.AP_MAC,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(formattedClients); err != nil {
		app.Logger.Errorf("Failed to encode clients: %v", err)
	}
}

// Test gate API
func (app *App) TestGateHandler(w http.ResponseWriter, r *http.Request) {
	if app.GateController == nil {
		app.GateController = gate.NewController(app.Config.Shelly.TriggerURL, app.Logger)
	}

	if err := app.GateController.OpenGate(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Log the test if activity logging is enabled
	if app.Config.Gate.LogActivity {
		if err := app.DB.LogEvent(&database.LogEntry{
			DeviceMAC:  "manual",
			DeviceName: "Manual Test",
			Event:      "gate_triggered",
			Direction:  "manual",
			GateOpened: true,
			Message:    "Gate opened via manual test",
		}); err != nil {
			app.Logger.Errorf("Failed to log test event: %v", err)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]bool{"success": true}); err != nil {
		app.Logger.Errorf("Failed to encode response: %v", err)
	}
}
