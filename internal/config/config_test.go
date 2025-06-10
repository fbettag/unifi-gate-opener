package config

import (
	"os"
	"testing"
)

func TestLoadOrInitialize(t *testing.T) {
	testFile := "test_config_load.yaml"
	defer os.Remove(testFile)
	
	t.Run("Create new config", func(t *testing.T) {
		cfg, err := LoadOrInitialize(testFile)
		if err != nil {
			t.Fatalf("Failed to create new config: %v", err)
		}
		
		if cfg.SetupComplete {
			t.Error("New config should not be setup complete")
		}
		
		if cfg.SessionSecret == "" {
			t.Error("Session secret should be generated")
		}
		
		if len(cfg.SessionSecret) != 44 { // 32 bytes base64 encoded = 44 chars
			t.Errorf("Session secret should be 44 chars (32 bytes base64 encoded), got %d", len(cfg.SessionSecret))
		}
	})
	
	t.Run("Load existing config", func(t *testing.T) {
		// Create initial config
		cfg1, err := LoadOrInitialize(testFile)
		if err != nil {
			t.Fatalf("Failed to create config: %v", err)
		}
		originalSecret := cfg1.SessionSecret
		
		// Save it
		if err := SaveConfig(testFile, cfg1); err != nil {
			t.Fatalf("Failed to save config: %v", err)
		}
		
		// Load it again
		cfg2, err := LoadOrInitialize(testFile)
		if err != nil {
			t.Fatalf("Failed to load existing config: %v", err)
		}
		
		if cfg2.SessionSecret != originalSecret {
			t.Error("Session secret should be preserved when loading existing config")
		}
	})
}

func TestIsConfigured(t *testing.T) {
	tests := []struct {
		name   string
		config *Config
		want   bool
	}{
		{
			name: "Empty config",
			config: &Config{},
			want: false,
		},
		{
			name: "Partially configured",
			config: &Config{
				SetupComplete: true,
				UniFi: UniFiConfig{
					ControllerURL: "https://test.com",
				},
			},
			want: false,
		},
		{
			name: "Fully configured",
			config: &Config{
				SetupComplete: true,
				Admin: AdminConfig{
					Username: "admin",
				},
				UniFi: UniFiConfig{
					ControllerURL: "https://test.com",
					Username:      "user",
					Password:      "pass",
					SiteID:        "default",
					GateAPMAC:     "aa:bb:cc:dd:ee:ff",
				},
				Shelly: ShellyConfig{
					TriggerURL: "http://test.com",
				},
			},
			want: true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.config.IsConfigured(); got != tt.want {
				t.Errorf("IsConfigured() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSetAdminPassword(t *testing.T) {
	cfg := &Config{}
	
	password := "testpassword123"
	err := cfg.SetAdminPassword(password)
	if err != nil {
		t.Fatalf("Failed to set admin password: %v", err)
	}
	
	if cfg.Admin.PasswordHash == "" {
		t.Error("Password hash should be set")
	}
	
	if cfg.Admin.PasswordHash == password {
		t.Error("Password should be hashed, not stored in plaintext")
	}
}

func TestVerifyAdminPassword(t *testing.T) {
	cfg := &Config{}
	password := "testpassword123"
	
	// Set password
	err := cfg.SetAdminPassword(password)
	if err != nil {
		t.Fatalf("Failed to set admin password: %v", err)
	}
	
	// Test correct password
	if !cfg.VerifyAdminPassword(password) {
		t.Error("Should verify correct password")
	}
	
	// Test incorrect password
	if cfg.VerifyAdminPassword("wrongpassword") {
		t.Error("Should not verify incorrect password")
	}
	
	// Test empty password
	if cfg.VerifyAdminPassword("") {
		t.Error("Should not verify empty password")
	}
}

func TestDeviceManagement(t *testing.T) {
	cfg := &Config{}
	
	// Test adding devices
	device1MAC := "aa:bb:cc:dd:ee:01"
	device1Name := "Test Device 1"
	
	device2MAC := "aa:bb:cc:dd:ee:02"
	device2Name := "Test Device 2"
	
	t.Run("Add first device", func(t *testing.T) {
		err := cfg.AddDevice(device1MAC, device1Name)
		if err != nil {
			t.Fatalf("Failed to add device1: %v", err)
		}
		
		if len(cfg.Devices) != 1 {
			t.Errorf("Expected 1 device, got %d", len(cfg.Devices))
		}
		
		if cfg.Devices[0].MAC != device1MAC {
			t.Errorf("Expected device MAC %s, got %s", device1MAC, cfg.Devices[0].MAC)
		}
		
		if cfg.Devices[0].Name != device1Name {
			t.Errorf("Expected device name %s, got %s", device1Name, cfg.Devices[0].Name)
		}
		
		if !cfg.Devices[0].Enabled {
			t.Error("New device should be enabled by default")
		}
	})
	
	t.Run("Add second device", func(t *testing.T) {
		err := cfg.AddDevice(device2MAC, device2Name)
		if err != nil {
			t.Fatalf("Failed to add device2: %v", err)
		}
		
		if len(cfg.Devices) != 2 {
			t.Errorf("Expected 2 devices, got %d", len(cfg.Devices))
		}
	})
	
	t.Run("Add duplicate device", func(t *testing.T) {
		err := cfg.AddDevice(device1MAC, device1Name)
		if err == nil {
			t.Error("Should fail to add duplicate device")
		}
		
		if len(cfg.Devices) != 2 {
			t.Errorf("Device count should remain 2, got %d", len(cfg.Devices))
		}
	})
	
	t.Run("Get existing device", func(t *testing.T) {
		device := cfg.GetDevice(device1MAC)
		if device == nil {
			t.Error("Should find existing device")
		} else if device.MAC != device1MAC {
			t.Errorf("Expected device MAC %s, got %s", device1MAC, device.MAC)
		}
	})
	
	t.Run("Get non-existent device", func(t *testing.T) {
		device := cfg.GetDevice("non:ex:is:te:nt:00")
		if device != nil {
			t.Error("Should not find non-existent device")
		}
	})
	
	t.Run("Update existing device", func(t *testing.T) {
		updatedName := "Updated Test Device 1"
		updatedEnabled := false
		
		err := cfg.UpdateDevice(device1MAC, updatedName, updatedEnabled)
		if err != nil {
			t.Fatalf("Failed to update device: %v", err)
		}
		
		device := cfg.GetDevice(device1MAC)
		if device == nil {
			t.Error("Device should still exist after update")
		} else {
			if device.Name != updatedName {
				t.Errorf("Expected updated name %s, got %s", updatedName, device.Name)
			}
			if device.Enabled != updatedEnabled {
				t.Errorf("Expected device enabled to be %v, got %v", updatedEnabled, device.Enabled)
			}
		}
	})
	
	t.Run("Update non-existent device", func(t *testing.T) {
		err := cfg.UpdateDevice("non:ex:is:te:nt:00", "Non-existent", true)
		if err == nil {
			t.Error("Should fail to update non-existent device")
		}
	})
	
	t.Run("Remove existing device", func(t *testing.T) {
		err := cfg.RemoveDevice(device1MAC)
		if err != nil {
			t.Fatalf("Failed to remove device: %v", err)
		}
		
		if len(cfg.Devices) != 1 {
			t.Errorf("Expected 1 device after removal, got %d", len(cfg.Devices))
		}
		
		device := cfg.GetDevice(device1MAC)
		if device != nil {
			t.Error("Device should not exist after removal")
		}
	})
	
	t.Run("Remove non-existent device", func(t *testing.T) {
		err := cfg.RemoveDevice("non:ex:is:te:nt:00")
		if err == nil {
			t.Error("Should fail to remove non-existent device")
		}
	})
	
	t.Run("Remove last device", func(t *testing.T) {
		err := cfg.RemoveDevice(device2MAC)
		if err != nil {
			t.Fatalf("Failed to remove last device: %v", err)
		}
		
		if len(cfg.Devices) != 0 {
			t.Errorf("Expected 0 devices after removing all, got %d", len(cfg.Devices))
		}
	})
}

func TestConfigEdgeCases(t *testing.T) {
	t.Run("LoadOrInitialize with invalid file path", func(t *testing.T) {
		// Try to load from a directory that doesn't exist
		_, err := LoadOrInitialize("/nonexistent/directory/config.yaml")
		if err == nil {
			t.Error("Should fail to load from non-existent directory")
		}
	})
	
	t.Run("SaveConfig with invalid file path", func(t *testing.T) {
		cfg := &Config{}
		err := SaveConfig("/nonexistent/directory/config.yaml", cfg)
		if err == nil {
			t.Error("Should fail to save to non-existent directory")
		}
	})
	
	t.Run("SetAdminPassword with empty password", func(t *testing.T) {
		cfg := &Config{}
		err := cfg.SetAdminPassword("")
		if err != nil {
			t.Fatalf("Should handle empty password: %v", err)
		}
		
		// Verify empty password actually does authenticate empty against empty hash
		// (this documents the current behavior)
		if !cfg.VerifyAdminPassword("") {
			t.Error("Empty password should authenticate against empty hash in current implementation")
		}
		
		// But non-empty passwords should not authenticate against empty hash
		if cfg.VerifyAdminPassword("somepassword") {
			t.Error("Non-empty password should not authenticate against empty hash")
		}
	})
	
	t.Run("VerifyAdminPassword with no hash set", func(t *testing.T) {
		cfg := &Config{}
		if cfg.VerifyAdminPassword("anypassword") {
			t.Error("Should not verify when no hash is set")
		}
	})
	
	t.Run("AddDevice with empty MAC", func(t *testing.T) {
		cfg := &Config{}
		err := cfg.AddDevice("", "Empty MAC Device")
		// Current implementation allows empty MAC - test documents current behavior
		if err != nil {
			t.Errorf("AddDevice should not fail with empty MAC in current implementation: %v", err)
		}
	})
	
	t.Run("UpdateDevice with empty MAC", func(t *testing.T) {
		cfg := &Config{}
		err := cfg.UpdateDevice("", "Empty MAC Device", true)
		// Current implementation will not find device with empty MAC, so should fail
		if err == nil {
			t.Error("Should fail to update device with empty MAC (not found)")
		}
	})
	
	t.Run("RemoveDevice with empty MAC", func(t *testing.T) {
		cfg := &Config{}
		err := cfg.RemoveDevice("")
		// Current implementation will not find device with empty MAC, so should fail
		if err == nil {
			t.Error("Should fail to remove device with empty MAC (not found)")
		}
	})
}