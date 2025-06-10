package testutils

import (
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/fbettag/unifi-gate-opener/internal/unifi"
)

// MockUniFiServer provides a mock UniFi controller for testing
type MockUniFiServer struct {
	Server *httptest.Server
	URL    string
}

// NewMockUniFiServer creates a new mock UniFi controller server
func NewMockUniFiServer() *MockUniFiServer {
	mux := http.NewServeMux()

	// Mock login endpoint
	mux.HandleFunc("/api/login", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Set a mock session cookie
		http.SetCookie(w, &http.Cookie{
			Name:  "unifises",
			Value: "mock-session-token",
			Path:  "/",
		})

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]interface{}{
			"meta": map[string]interface{}{
				"rc": "ok",
			},
		}); err != nil {
			log.Printf("Failed to encode response: %v", err)
		}
	})

	// Mock sites endpoint
	mux.HandleFunc("/api/stat/sites", func(w http.ResponseWriter, r *http.Request) {
		sites := []unifi.Site{
			{ID: "default", Name: "default", Description: "Default Site"},
			{ID: "site2", Name: "site2", Description: "Test Site 2"},
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]interface{}{
			"meta": map[string]interface{}{"rc": "ok"},
			"data": sites,
		}); err != nil {
			log.Printf("Failed to encode response: %v", err)
		}
	})

	// Mock access points endpoint
	mux.HandleFunc("/api/s/", func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/stat/device") {
			aps := []unifi.AccessPoint{
				{
					MAC:     "aa:bb:cc:dd:ee:ff",
					Name:    "Test AP 1",
					Model:   "U7MP",
					IP:      "192.168.1.10",
					Adopted: true,
				},
				{
					MAC:     "11:22:33:44:55:66",
					Name:    "Test AP 2",
					Model:   "UAP6MP",
					IP:      "192.168.1.11",
					Adopted: true,
				},
			}

			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(map[string]interface{}{
				"meta": map[string]interface{}{"rc": "ok"},
				"data": aps,
			}); err != nil {
				log.Printf("Failed to encode response: %v", err)
			}
		} else if strings.Contains(r.URL.Path, "/stat/sta") {
			clients := []unifi.WirelessClient{
				{
					MAC:      "aa:bb:cc:dd:ee:01",
					Hostname: "Test Device 1",
					IP:       "192.168.1.100",
					AP_MAC:   "aa:bb:cc:dd:ee:ff",
					Signal:   -45,
				},
				{
					MAC:      "aa:bb:cc:dd:ee:02",
					Hostname: "Test Device 2",
					IP:       "192.168.1.101",
					AP_MAC:   "11:22:33:44:55:66",
					Signal:   -52,
				},
			}

			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(map[string]interface{}{
				"meta": map[string]interface{}{"rc": "ok"},
				"data": clients,
			}); err != nil {
				log.Printf("Failed to encode response: %v", err)
			}
		} else {
			http.NotFound(w, r)
		}
	})

	// Mock status endpoint
	mux.HandleFunc("/status", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]interface{}{
			"meta": map[string]interface{}{"rc": "ok"},
		}); err != nil {
			log.Printf("Failed to encode response: %v", err)
		}
	})

	server := httptest.NewTLSServer(mux)

	return &MockUniFiServer{
		Server: server,
		URL:    server.URL,
	}
}

// Close shuts down the mock server
func (m *MockUniFiServer) Close() {
	m.Server.Close()
}

// GetTestCredentials returns test credentials for the mock server
func (m *MockUniFiServer) GetTestCredentials() (string, string, string) {
	return "testuser", "testpass", "default"
}
