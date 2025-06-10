package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/fbettag/unifi-gate-opener/internal/unifi"
)

// Helper function to send JSON error responses
func (app *App) sendJSONError(w http.ResponseWriter, message string, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(map[string]interface{}{
		"success": false,
		"error":   message,
	}); err != nil {
		app.Logger.Errorf("Failed to encode error response: %v", err)
	}
}

// Test UniFi connection endpoint
func (app *App) TestUniFiHandler(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ControllerURL string `json:"controller_url"`
		Username      string `json:"username"`
		Password      string `json:"password"`
		SiteID        string `json:"site_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		app.Logger.Errorf("Failed to decode request body: %v", err)
		app.sendJSONError(w, fmt.Sprintf("Invalid request: %v", err), http.StatusBadRequest)
		return
	}

	app.Logger.Debugf("TestUniFi request: URL=%s, User=%s, Site=%s", req.ControllerURL, req.Username, req.SiteID)

	// Create a temporary UniFi client
	unifiLogger := unifi.NewLogrusAdapter(app.Logger)
	testClient := unifi.NewClient(req.ControllerURL, req.Username, req.Password, unifiLogger)

	// Try to login
	if err := testClient.Login(); err != nil {
		app.Logger.Errorf("UniFi login test failed: %v", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		if err := json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "Failed to connect to UniFi Controller. Please check your credentials and URL.",
		}); err != nil {
			app.Logger.Errorf("Failed to encode error response: %v", err)
		}
		return
	}

	// Try to get access points to verify site access
	aps, err := testClient.GetAccessPoints(req.SiteID)
	if err != nil {
		app.Logger.Errorf("Failed to get access points: %v", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		if err := json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "Connected to UniFi but failed to access site. Please check the Site ID.",
		}); err != nil {
			app.Logger.Errorf("Failed to encode error response: %v", err)
		}
		return
	}

	// Return success with access points
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]interface{}{
		"success":       true,
		"access_points": aps,
	}); err != nil {
		app.Logger.Errorf("Failed to encode success response: %v", err)
	}
}

// Test UniFi and get sites
func (app *App) TestUniFiSitesHandler(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ControllerURL string `json:"controller_url"`
		Username      string `json:"username"`
		Password      string `json:"password"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		app.sendJSONError(w, "Invalid request", http.StatusBadRequest)
		return
	}

	// Create a temporary UniFi client
	unifiLogger := unifi.NewLogrusAdapter(app.Logger)
	testClient := unifi.NewClient(req.ControllerURL, req.Username, req.Password, unifiLogger)

	// Try to login
	if err := testClient.Login(); err != nil {
		app.Logger.Errorf("UniFi login test failed: %v", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		if err := json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "Failed to connect to UniFi Controller. Please check your credentials and URL.",
		}); err != nil {
			app.Logger.Errorf("Failed to encode error response: %v", err)
		}
		return
	}

	// Get sites
	sites, err := testClient.GetSites()
	if err != nil {
		app.Logger.Warnf("Failed to get sites, using default: %v", err)
		// Return default site if we can't get sites
		sites = []unifi.Site{{Name: "default", Description: "Default Site"}}
	}

	// Return success with sites
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"sites":   sites,
	}); err != nil {
		app.Logger.Errorf("Failed to encode success response: %v", err)
	}
}
