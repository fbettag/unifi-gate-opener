//go:build integration

package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/cookiejar"
	"os"
	"testing"
	"time"

	"github.com/fbettag/unifi-gate-opener/internal/auth"
	"github.com/fbettag/unifi-gate-opener/internal/config"
	"github.com/fbettag/unifi-gate-opener/internal/database"
	"github.com/fbettag/unifi-gate-opener/internal/handlers"
	"github.com/sirupsen/logrus"
)

type integrationTestServer struct {
	app     *handlers.App
	server  *http.Server
	client  *http.Client
	baseURL string
}

// Test configuration from environment variables
var (
	testUniFiURL      = os.Getenv("UNIFI_CONTROLLER_URL")
	testUniFiUsername = os.Getenv("UNIFI_USERNAME")
	testUniFiPassword = os.Getenv("UNIFI_PASSWORD")
	testUniFiSiteID   = os.Getenv("UNIFI_SITE_ID")
)

func TestIntegration(t *testing.T) {
	// Skip if environment variables are not set
	if testUniFiURL == "" || testUniFiUsername == "" || testUniFiPassword == "" {
		t.Skip("Integration tests require UNIFI_CONTROLLER_URL, UNIFI_USERNAME, and UNIFI_PASSWORD environment variables")
	}

	if testUniFiSiteID == "" {
		testUniFiSiteID = "default"
	}

	// Set up test server
	ts := setupIntegrationTestServer(t)
	defer ts.cleanup()

	t.Run("Complete application flow with real UniFi controller", func(t *testing.T) {
		// Test UniFi connection
		testUniFiConnection(t, ts)
		
		// Complete setup
		completeSetup(t, ts)
		
		// Test authentication
		testAuthentication(t, ts)
		
		// Test API endpoints
		testAPIEndpoints(t, ts)
	})
}

func setupIntegrationTestServer(t *testing.T) *integrationTestServer {
	// Clean up any existing test files
	os.Remove("integration_test_config.yaml")
	os.Remove("integration_test.db")

	// Set up logger with reduced noise
	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)
	logger.SetFormatter(&logrus.TextFormatter{
		DisableColors:    true,
		DisableTimestamp: true,
	})

	// Initialize test config
	cfg := &config.Config{
		DatabasePath:  "integration_test.db",
		SessionSecret: "integration-test-secret-32-chars!",
		SetupComplete: false,
	}

	// Initialize database
	db, err := database.Initialize(cfg.DatabasePath)
	if err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}

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

	// Set up routes
	router := setupRoutes(app)

	// Create test server
	server := &http.Server{
		Addr:    ":8086",
		Handler: router,
	}

	// Start server in background
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			t.Logf("Server error: %v", err)
		}
	}()

	// Wait for server to start
	time.Sleep(200 * time.Millisecond)

	// Create HTTP client with cookie jar
	jar, _ := cookiejar.New(nil)
	client := &http.Client{
		Jar: jar,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse // Don't follow redirects automatically
		},
	}

	return &integrationTestServer{
		app:     app,
		server:  server,
		client:  client,
		baseURL: "http://localhost:8086",
	}
}

func (ts *integrationTestServer) cleanup() {
	ts.server.Close()
	ts.app.DB.Close()
	os.Remove("integration_test_config.yaml")
	os.Remove("integration_test.db")
}

func (ts *integrationTestServer) request(method, path string, body interface{}) (*http.Response, error) {
	var bodyReader io.Reader
	if body != nil {
		jsonData, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		bodyReader = bytes.NewReader(jsonData)
	}

	req, err := http.NewRequest(method, ts.baseURL+path, bodyReader)
	if err != nil {
		return nil, err
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	return ts.client.Do(req)
}

func testUniFiConnection(t *testing.T, ts *integrationTestServer) {
	t.Run("Test UniFi connection", func(t *testing.T) {
		payload := map[string]string{
			"controller_url": testUniFiURL,
			"username":       testUniFiUsername,
			"password":       testUniFiPassword,
			"site_id":        testUniFiSiteID,
		}

		resp, err := ts.request("POST", "/api/test-unifi", payload)
		if err != nil {
			t.Fatalf("Failed to test UniFi connection: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("UniFi test failed with status %d", resp.StatusCode)
		}

		var result map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		if !result["success"].(bool) {
			t.Fatalf("UniFi test failed: %v", result["error"])
		}

		aps, ok := result["access_points"].([]interface{})
		if !ok || len(aps) == 0 {
			t.Fatal("Expected to get access points from UniFi")
		}

		t.Logf("Successfully connected to UniFi controller, found %d access points", len(aps))
	})
}

func completeSetup(t *testing.T, ts *integrationTestServer) {
	t.Run("Complete setup", func(t *testing.T) {
		setupPayload := map[string]interface{}{
			"admin": map[string]string{
				"username": "admin",
				"password": "testpassword123",
			},
			"unifi": map[string]string{
				"controller_url": testUniFiURL,
				"username":       testUniFiUsername,
				"password":       testUniFiPassword,
				"site_id":        testUniFiSiteID,
				"gate_ap_mac":    "00:00:00:00:00:00", // Placeholder
			},
			"shelly": map[string]string{
				"trigger_url": "http://test-shelly/relay/0?turn=on&timer=5",
			},
			"gate": map[string]int{
				"open_duration": 5,
			},
		}

		resp, err := ts.request("POST", "/api/setup", setupPayload)
		if err != nil {
			t.Fatalf("Failed to complete setup: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("Setup failed with status %d", resp.StatusCode)
		}

		var result map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			t.Fatalf("Failed to decode setup response: %v", err)
		}

		if !result["success"].(bool) {
			t.Fatal("Setup returned success=false")
		}

		t.Log("Setup completed successfully")
	})
}

func testAuthentication(t *testing.T, ts *integrationTestServer) {
	t.Run("Test authentication flow", func(t *testing.T) {
		// Clear cookies
		ts.client.Jar, _ = cookiejar.New(nil)

		// Try to access protected endpoint - should be redirected
		resp, err := ts.request("GET", "/api/devices", nil)
		if err != nil {
			t.Fatalf("Failed to access protected endpoint: %v", err)
		}
		resp.Body.Close()

		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("Expected 401 for unauthenticated request, got %d", resp.StatusCode)
		}

		// Login
		loginPayload := map[string]string{
			"username": "admin",
			"password": "testpassword123",
		}

		resp2, err := ts.request("POST", "/api/login", loginPayload)
		if err != nil {
			t.Fatalf("Failed to login: %v", err)
		}
		defer resp2.Body.Close()

		if resp2.StatusCode != http.StatusOK {
			t.Fatalf("Login failed with status %d", resp2.StatusCode)
		}

		var result map[string]interface{}
		if err := json.NewDecoder(resp2.Body).Decode(&result); err != nil {
			t.Fatalf("Failed to decode login response: %v", err)
		}

		if !result["success"].(bool) {
			t.Fatal("Login returned success=false")
		}

		t.Log("Authentication flow successful")
	})
}

func testAPIEndpoints(t *testing.T, ts *integrationTestServer) {
	t.Run("Test API endpoints", func(t *testing.T) {
		// Test devices endpoint
		resp, err := ts.request("GET", "/api/devices", nil)
		if err != nil {
			t.Fatalf("Failed to get devices: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Get devices failed with status %d", resp.StatusCode)
		}

		// Test status endpoint
		resp2, err := ts.request("GET", "/api/status", nil)
		if err != nil {
			t.Fatalf("Failed to get status: %v", err)
		}
		defer resp2.Body.Close()

		if resp2.StatusCode != http.StatusOK {
			t.Errorf("Get status failed with status %d", resp2.StatusCode)
		}

		var status map[string]interface{}
		if err := json.NewDecoder(resp2.Body).Decode(&status); err != nil {
			t.Fatalf("Failed to decode status: %v", err)
		}

		t.Logf("Monitoring status: %v", status["is_monitoring"])

		// Test UniFi endpoints (these should work with real credentials)
		resp3, err := ts.request("GET", "/api/unifi/aps", nil)
		if err != nil {
			t.Fatalf("Failed to get access points: %v", err)
		}
		defer resp3.Body.Close()

		if resp3.StatusCode != http.StatusOK {
			t.Errorf("Get access points failed with status %d", resp3.StatusCode)
		}

		t.Log("All API endpoints working correctly")
	})
}