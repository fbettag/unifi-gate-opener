package auth

import (
	"net/http/httptest"
	"testing"
)

func TestNewSessionStore(t *testing.T) {
	secret := "test-secret-key-32-characters!!"
	store := NewSessionStore(secret)
	
	if store == nil {
		t.Fatal("Session store should not be nil")
	}
}

func TestSessionOperations(t *testing.T) {
	secret := "test-secret-key-32-characters!!"
	store := NewSessionStore(secret)
	
	// Create test request and response recorder
	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	
	t.Run("Get new session", func(t *testing.T) {
		session, err := store.GetSession(req)
		if err != nil {
			t.Fatalf("Failed to get new session: %v", err)
		}
		
		if session == nil {
			t.Fatal("Session should not be nil")
		}
		
		if !session.IsNew {
			t.Error("New session should be marked as new")
		}
	})
	
	t.Run("User not authenticated initially", func(t *testing.T) {
		if store.IsAuthenticated(req) {
			t.Error("User should not be authenticated initially")
		}
	})
	
	t.Run("Login user", func(t *testing.T) {
		err := store.Login(req, w)
		if err != nil {
			t.Fatalf("Failed to login user: %v", err)
		}
		
		// Check that cookie was set
		cookies := w.Result().Cookies()
		found := false
		for _, cookie := range cookies {
			if cookie.Name == "gate-opener-session" {
				found = true
				if cookie.Value == "" {
					t.Error("Session cookie should have a value")
				}
				if !cookie.HttpOnly {
					t.Error("Session cookie should be HttpOnly")
				}
				break
			}
		}
		if !found {
			t.Error("Session cookie should be set")
		}
	})
	
	t.Run("User authenticated after login", func(t *testing.T) {
		// Create new request with the session cookie
		cookies := w.Result().Cookies()
		reqWithCookie := httptest.NewRequest("GET", "/", nil)
		for _, cookie := range cookies {
			if cookie.Name == "gate-opener-session" {
				reqWithCookie.AddCookie(cookie)
				break
			}
		}
		
		if !store.IsAuthenticated(reqWithCookie) {
			t.Error("User should be authenticated after login")
		}
	})
	
	t.Run("Logout user", func(t *testing.T) {
		// Create new request with the session cookie
		cookies := w.Result().Cookies()
		reqWithCookie := httptest.NewRequest("GET", "/", nil)
		for _, cookie := range cookies {
			if cookie.Name == "gate-opener-session" {
				reqWithCookie.AddCookie(cookie)
				break
			}
		}
		
		wLogout := httptest.NewRecorder()
		err := store.Logout(reqWithCookie, wLogout)
		if err != nil {
			t.Fatalf("Failed to logout user: %v", err)
		}
		
		// Create request with logout cookie
		logoutCookies := wLogout.Result().Cookies()
		reqAfterLogout := httptest.NewRequest("GET", "/", nil)
		for _, cookie := range logoutCookies {
			if cookie.Name == "gate-opener-session" {
				reqAfterLogout.AddCookie(cookie)
				break
			}
		}
		
		if store.IsAuthenticated(reqAfterLogout) {
			t.Error("User should not be authenticated after logout")
		}
	})
}

func TestSessionSecurity(t *testing.T) {
	secret := "test-secret-key-32-characters!!"
	store := NewSessionStore(secret)
	
	// Create test request and response recorder
	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	
	// Login user
	err := store.Login(req, w)
	if err != nil {
		t.Fatalf("Failed to login user: %v", err)
	}
	
	// Get the cookie value
	cookies := w.Result().Cookies()
	var cookieValue string
	for _, cookie := range cookies {
		if cookie.Name == "gate-opener-session" {
			cookieValue = cookie.Value
			break
		}
	}
	
	// Cookie value should exist and be encrypted/signed
	if cookieValue == "" {
		t.Fatal("Session cookie should have a value")
	}
	
	// The cookie should be encrypted/signed, not plaintext
	if cookieValue == "authenticated" || cookieValue == "true" {
		t.Error("Session cookie should not contain plaintext sensitive data")
	}
	
	// Cookie should be reasonably long (encrypted/signed)
	if len(cookieValue) < 20 {
		t.Error("Session cookie should be encrypted/signed and reasonably long")
	}
}

func TestSessionWithInvalidSecret(t *testing.T) {
	// Test with short secret
	store := NewSessionStore("short")
	
	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	
	// This should still work but might not be secure
	session, err := store.GetSession(req)
	if err != nil {
		t.Fatalf("Session should work even with short secret: %v", err)
	}
	
	if session == nil {
		t.Error("Session should not be nil")
	}
	
	// Should be able to login
	err = store.Login(req, w)
	if err != nil {
		t.Fatalf("Login should work even with short secret: %v", err)
	}
}

func TestMultipleSessions(t *testing.T) {
	secret := "test-secret-key-32-characters!!"
	store := NewSessionStore(secret)
	
	// Create two different requests
	req1 := httptest.NewRequest("GET", "/", nil)
	req2 := httptest.NewRequest("GET", "/", nil)
	
	// Get sessions
	session1, err := store.GetSession(req1)
	if err != nil {
		t.Fatalf("Failed to get session1: %v", err)
	}
	
	session2, err := store.GetSession(req2)
	if err != nil {
		t.Fatalf("Failed to get session2: %v", err)
	}
	
	// Both should be new sessions
	if !session1.IsNew {
		t.Error("Session1 should be new")
	}
	if !session2.IsNew {
		t.Error("Session2 should be new")
	}
	
	// Neither should be authenticated initially
	if store.IsAuthenticated(req1) {
		t.Error("Request1 should not be authenticated initially")
	}
	if store.IsAuthenticated(req2) {
		t.Error("Request2 should not be authenticated initially")
	}
	
	// Login one session
	w1 := httptest.NewRecorder()
	err = store.Login(req1, w1)
	if err != nil {
		t.Fatalf("Failed to login session1: %v", err)
	}
	
	// Create new request with session1 cookie
	cookies1 := w1.Result().Cookies()
	reqWithCookie1 := httptest.NewRequest("GET", "/", nil)
	for _, cookie := range cookies1 {
		if cookie.Name == "gate-opener-session" {
			reqWithCookie1.AddCookie(cookie)
			break
		}
	}
	
	// Session1 should be authenticated, session2 should not
	if !store.IsAuthenticated(reqWithCookie1) {
		t.Error("Session1 should be authenticated after login")
	}
	if store.IsAuthenticated(req2) {
		t.Error("Session2 should remain unauthenticated")
	}
}

func TestSessionSaveErrors(t *testing.T) {
	secret := "test-secret-key-32-characters!!"
	store := NewSessionStore(secret)
	
	// Test SaveSession with valid session
	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	
	session, err := store.GetSession(req)
	if err != nil {
		t.Fatalf("Failed to get session: %v", err)
	}
	
	err = store.SaveSession(req, w, session)
	if err != nil {
		t.Errorf("SaveSession should not fail with valid session: %v", err)
	}
}

func TestAuthenticationEdgeCases(t *testing.T) {
	secret := "test-secret-key-32-characters!!"
	store := NewSessionStore(secret)
	
	t.Run("IsAuthenticated with missing cookie", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		if store.IsAuthenticated(req) {
			t.Error("Should not be authenticated without session cookie")
		}
	})
	
	t.Run("IsAuthenticated with invalid session data", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		// Add invalid session cookie
		req.Header.Set("Cookie", "gate-opener-session=invalid-data")
		
		if store.IsAuthenticated(req) {
			t.Error("Should not be authenticated with invalid session data")
		}
	})
	
	t.Run("Login and verify session persistence", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()
		
		// Login
		err := store.Login(req, w)
		if err != nil {
			t.Fatalf("Login failed: %v", err)
		}
		
		// Get session and check it's properly set
		session, err := store.GetSession(req)
		if err != nil {
			t.Fatalf("Failed to get session after login: %v", err)
		}
		
		authenticated, exists := session.Values["authenticated"]
		if !exists {
			t.Error("Session should have authenticated key")
		}
		
		if authenticated != true {
			t.Error("Session authenticated value should be true")
		}
	})
}