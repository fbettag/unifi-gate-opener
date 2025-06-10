package gate

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sirupsen/logrus"
)

func TestNewController(t *testing.T) {
	logger := logrus.New()
	triggerURL := "http://test.com/trigger"
	
	controller := NewController(triggerURL, logger)
	
	if controller == nil {
		t.Fatal("Controller should not be nil")
	}
	
	if controller.triggerURL != triggerURL {
		t.Errorf("Expected trigger URL %s, got %s", triggerURL, controller.triggerURL)
	}
	
	if controller.logger != logger {
		t.Error("Logger should be set correctly")
	}
	
	if controller.client == nil {
		t.Error("HTTP client should be initialized")
	}
	
	// Check timeout is set
	if controller.client.Timeout == 0 {
		t.Error("HTTP client timeout should be set")
	}
}

func TestOpenGate(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.PanicLevel) // Suppress log output in tests
	
	t.Run("Success", func(t *testing.T) {
		// Create mock server that returns 200 OK
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("OK"))
		}))
		defer server.Close()
		
		controller := NewController(server.URL, logger)
		
		err := controller.OpenGate()
		if err != nil {
			t.Fatalf("OpenGate should succeed: %v", err)
		}
	})
	
	t.Run("Empty URL", func(t *testing.T) {
		controller := NewController("", logger)
		
		err := controller.OpenGate()
		if err == nil {
			t.Error("OpenGate should fail with empty URL")
		}
		
		expectedMsg := "gate trigger URL not configured"
		if err.Error() != expectedMsg {
			t.Errorf("Expected error '%s', got '%s'", expectedMsg, err.Error())
		}
	})
	
	t.Run("Server Error", func(t *testing.T) {
		// Create mock server that returns 500 error
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("Server Error"))
		}))
		defer server.Close()
		
		controller := NewController(server.URL, logger)
		
		err := controller.OpenGate()
		if err == nil {
			t.Error("OpenGate should fail with 500 status")
		}
	})
	
	t.Run("Network Error", func(t *testing.T) {
		controller := NewController("http://non-existent-server.test", logger)
		
		err := controller.OpenGate()
		if err == nil {
			t.Error("OpenGate should fail with network error")
		}
	})
	
	t.Run("Different Status Codes", func(t *testing.T) {
		testCases := []struct {
			statusCode int
			shouldFail bool
		}{
			{http.StatusOK, false},
			{http.StatusCreated, true}, // Only 200 OK is accepted
			{http.StatusNotFound, true},
			{http.StatusUnauthorized, true},
			{http.StatusForbidden, true},
		}
		
		for _, tc := range testCases {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.statusCode)
			}))
			
			controller := NewController(server.URL, logger)
			err := controller.OpenGate()
			
			if tc.shouldFail && err == nil {
				t.Errorf("Expected failure for status code %d", tc.statusCode)
			} else if !tc.shouldFail && err != nil {
				t.Errorf("Expected success for status code %d, got error: %v", tc.statusCode, err)
			}
			
			server.Close()
		}
	})
}

func TestTestConnection(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.PanicLevel) // Suppress log output in tests
	
	t.Run("Success with 200", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != "HEAD" {
				t.Errorf("Expected HEAD request, got %s", r.Method)
			}
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()
		
		controller := NewController(server.URL, logger)
		
		err := controller.TestConnection()
		if err != nil {
			t.Fatalf("TestConnection should succeed: %v", err)
		}
	})
	
	t.Run("Success with 404 (valid endpoint)", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		}))
		defer server.Close()
		
		controller := NewController(server.URL, logger)
		
		err := controller.TestConnection()
		if err != nil {
			t.Fatalf("TestConnection should succeed with 404: %v", err)
		}
	})
	
	t.Run("Success with 401 (auth required)", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
		}))
		defer server.Close()
		
		controller := NewController(server.URL, logger)
		
		err := controller.TestConnection()
		if err != nil {
			t.Fatalf("TestConnection should succeed with 401: %v", err)
		}
	})
	
	t.Run("Fail with 500", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer server.Close()
		
		controller := NewController(server.URL, logger)
		
		err := controller.TestConnection()
		if err == nil {
			t.Error("TestConnection should fail with 500")
		}
	})
	
	t.Run("Fail with 502", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadGateway)
		}))
		defer server.Close()
		
		controller := NewController(server.URL, logger)
		
		err := controller.TestConnection()
		if err == nil {
			t.Error("TestConnection should fail with 502")
		}
	})
	
	t.Run("Empty URL", func(t *testing.T) {
		controller := NewController("", logger)
		
		err := controller.TestConnection()
		if err == nil {
			t.Error("TestConnection should fail with empty URL")
		}
		
		expectedMsg := "gate trigger URL not configured"
		if err.Error() != expectedMsg {
			t.Errorf("Expected error '%s', got '%s'", expectedMsg, err.Error())
		}
	})
	
	t.Run("Network Error", func(t *testing.T) {
		controller := NewController("http://non-existent-server.test", logger)
		
		err := controller.TestConnection()
		if err == nil {
			t.Error("TestConnection should fail with network error")
		}
	})
	
	t.Run("Invalid URL", func(t *testing.T) {
		controller := NewController("not-a-url", logger)
		
		err := controller.TestConnection()
		if err == nil {
			t.Error("TestConnection should fail with invalid URL")
		}
	})
}

func TestUpdateURL(t *testing.T) {
	logger := logrus.New()
	originalURL := "http://original.com"
	newURL := "http://new.com"
	
	controller := NewController(originalURL, logger)
	
	if controller.triggerURL != originalURL {
		t.Errorf("Expected original URL %s, got %s", originalURL, controller.triggerURL)
	}
	
	controller.UpdateURL(newURL)
	
	if controller.triggerURL != newURL {
		t.Errorf("Expected updated URL %s, got %s", newURL, controller.triggerURL)
	}
}

func TestControllerEdgeCases(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.PanicLevel) // Suppress log output in tests
	
	t.Run("Multiple operations", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()
		
		controller := NewController(server.URL, logger)
		
		// Test connection multiple times
		for i := 0; i < 3; i++ {
			err := controller.TestConnection()
			if err != nil {
				t.Fatalf("TestConnection %d failed: %v", i, err)
			}
		}
		
		// Open gate multiple times
		for i := 0; i < 3; i++ {
			err := controller.OpenGate()
			if err != nil {
				t.Fatalf("OpenGate %d failed: %v", i, err)
			}
		}
	})
	
	t.Run("Update URL and test again", func(t *testing.T) {
		server1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer server1.Close()
		
		server2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer server2.Close()
		
		controller := NewController(server1.URL, logger)
		
		// Test with first URL
		err := controller.TestConnection()
		if err != nil {
			t.Fatalf("First TestConnection failed: %v", err)
		}
		
		// Update to second URL
		controller.UpdateURL(server2.URL)
		
		// Test with second URL
		err = controller.TestConnection()
		if err != nil {
			t.Fatalf("Second TestConnection failed: %v", err)
		}
		
		// Open gate with second URL
		err = controller.OpenGate()
		if err != nil {
			t.Fatalf("OpenGate with second URL failed: %v", err)
		}
	})
	
	t.Run("Empty URL update", func(t *testing.T) {
		controller := NewController("http://original.com", logger)
		
		controller.UpdateURL("")
		
		err := controller.TestConnection()
		if err == nil {
			t.Error("Should fail after updating to empty URL")
		}
		
		err = controller.OpenGate()
		if err == nil {
			t.Error("Should fail after updating to empty URL")
		}
	})
}