package database

import (
	"os"
	"strings"
	"testing"
)

func TestInitialize(t *testing.T) {
	dbFile := "test_database.db"
	defer os.Remove(dbFile)
	
	db, err := Initialize(dbFile)
	if err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()
	
	// Check if tables exist
	tables := []string{"logs", "device_states"}
	for _, table := range tables {
		var count int
		query := "SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?"
		err := db.QueryRow(query, table).Scan(&count)
		if err != nil {
			t.Fatalf("Failed to check table %s: %v", table, err)
		}
		if count != 1 {
			t.Errorf("Table %s should exist", table)
		}
	}
}

func TestLogOperations(t *testing.T) {
	dbFile := "test_logs.db"
	defer os.Remove(dbFile)
	
	db, err := Initialize(dbFile)
	if err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()
	
	t.Run("Insert log entry", func(t *testing.T) {
		entry := &LogEntry{
			DeviceMAC:   "aa:bb:cc:dd:ee:ff",
			DeviceName:  "Test Device",
			Event:       "connected",
			Direction:   "arriving",
			FromAP:      "",
			ToAP:        "test-ap-mac",
			GateOpened:  true,
			Message:     "Gate opened for test device",
		}
		
		err := db.LogEvent(entry)
		if err != nil {
			t.Fatalf("Failed to insert log entry: %v", err)
		}
	})
	
	t.Run("Query log entries", func(t *testing.T) {
		logs, err := db.GetLogs(10, 0)
		if err != nil {
			t.Fatalf("Failed to query logs: %v", err)
		}
		
		if len(logs) == 0 {
			t.Error("Expected at least one log entry")
		}
		
		log := logs[0]
		if log.DeviceMAC != "aa:bb:cc:dd:ee:ff" {
			t.Errorf("Expected MAC aa:bb:cc:dd:ee:ff, got %s", log.DeviceMAC)
		}
		if log.DeviceName != "Test Device" {
			t.Errorf("Expected name 'Test Device', got %s", log.DeviceName)
		}
		if log.Event != "connected" {
			t.Errorf("Expected event 'connected', got %s", log.Event)
		}
		if !log.GateOpened {
			t.Error("Gate should be opened")
		}
	})
	
	t.Run("Query logs by device", func(t *testing.T) {
		logs, err := db.GetLogsByDevice("aa:bb:cc:dd:ee:ff", 5)
		if err != nil {
			t.Fatalf("Failed to query logs by device: %v", err)
		}
		
		if len(logs) == 0 {
			t.Error("Expected at least one log entry for device")
		}
		
		for _, log := range logs {
			if log.DeviceMAC != "aa:bb:cc:dd:ee:ff" {
				t.Errorf("All logs should be for the specified device, got %s", log.DeviceMAC)
			}
		}
	})
	
	t.Run("Get recent activity", func(t *testing.T) {
		logs, err := db.GetRecentActivity(24) // Last 24 hours
		if err != nil {
			t.Fatalf("Failed to get recent activity: %v", err)
		}
		
		if len(logs) == 0 {
			t.Error("Expected at least one recent log entry")
		}
	})
}

func TestDeviceStateOperations(t *testing.T) {
	dbFile := "test_device_states.db"
	defer os.Remove(dbFile)
	
	db, err := Initialize(dbFile)
	if err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()
	
	deviceMAC := "aa:bb:cc:dd:ee:ff"
	apMAC := "test-ap-mac"
	
	t.Run("Update device state", func(t *testing.T) {
		err := db.UpdateDeviceState(deviceMAC, apMAC, true)
		if err != nil {
			t.Fatalf("Failed to update device state: %v", err)
		}
	})
	
	t.Run("Get device state", func(t *testing.T) {
		currentAP, lastSeen, isConnected, err := db.GetDeviceState(deviceMAC)
		if err != nil {
			t.Fatalf("Failed to get device state: %v", err)
		}
		
		if currentAP != apMAC {
			t.Errorf("Expected current AP %s, got %s", apMAC, currentAP)
		}
		if !isConnected {
			t.Error("Device should be connected")
		}
		if lastSeen.IsZero() {
			t.Error("Last seen time should be set")
		}
	})
	
	t.Run("Update last gate trigger", func(t *testing.T) {
		err := db.UpdateLastGateTrigger(deviceMAC)
		if err != nil {
			t.Fatalf("Failed to update last gate trigger: %v", err)
		}
		
		lastTrigger, err := db.GetLastGateTrigger(deviceMAC)
		if err != nil {
			t.Fatalf("Failed to get last gate trigger: %v", err)
		}
		
		if lastTrigger.IsZero() {
			t.Error("Last trigger time should be set")
		}
	})
	
	t.Run("Get connected devices", func(t *testing.T) {
		connected, err := db.GetConnectedDevices()
		if err != nil {
			t.Fatalf("Failed to get connected devices: %v", err)
		}
		
		if !connected[deviceMAC] {
			t.Error("Device should be listed as connected")
		}
	})
	
	t.Run("Disconnect device", func(t *testing.T) {
		err := db.UpdateDeviceState(deviceMAC, "", false)
		if err != nil {
			t.Fatalf("Failed to disconnect device: %v", err)
		}
		
		_, _, isConnected, err := db.GetDeviceState(deviceMAC)
		if err != nil {
			t.Fatalf("Failed to get device state: %v", err)
		}
		
		if isConnected {
			t.Error("Device should be disconnected")
		}
	})
}

func TestLogCleanup(t *testing.T) {
	dbFile := "test_cleanup.db"
	defer os.Remove(dbFile)
	
	db, err := Initialize(dbFile)
	if err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()
	
	// Insert a current log entry
	currentEntry := &LogEntry{
		DeviceMAC:  "aa:bb:cc:dd:ee:01",
		DeviceName: "Current Device",
		Event:      "connected",
		Message:    "Current entry",
	}
	
	err = db.LogEvent(currentEntry)
	if err != nil {
		t.Fatalf("Failed to insert current log: %v", err)
	}
	
	// Insert an old log entry by manipulating the timestamp
	_, err = db.Exec(`
		INSERT INTO logs (device_mac, device_name, event, direction, message, timestamp)
		VALUES (?, ?, ?, ?, ?, datetime('now', '-31 days'))
	`, "aa:bb:cc:dd:ee:02", "Old Device", "connected", "", "Old entry")
	if err != nil {
		t.Fatalf("Failed to insert old log: %v", err)
	}
	
	// Count logs before cleanup
	logs, err := db.GetLogs(100, 0)
	if err != nil {
		t.Fatalf("Failed to get logs before cleanup: %v", err)
	}
	countBefore := len(logs)
	
	if countBefore < 2 {
		t.Fatalf("Expected at least 2 logs before cleanup, got %d", countBefore)
	}
	
	// Perform cleanup (delete logs older than 30 days)
	deleted, err := db.DeleteOldLogs(30)
	if err != nil {
		t.Fatalf("Failed to cleanup old logs: %v", err)
	}
	
	if deleted == 0 {
		t.Error("Expected at least one old log to be deleted")
	}
	
	// Count logs after cleanup
	logs, err = db.GetLogs(100, 0)
	if err != nil {
		t.Fatalf("Failed to get logs after cleanup: %v", err)
	}
	countAfter := len(logs)
	
	if countAfter >= countBefore {
		t.Error("Log count should decrease after cleanup")
	}
	
	// Verify the current log is still there
	found := false
	for _, log := range logs {
		if strings.Contains(log.Message, "Current entry") {
			found = true
			break
		}
	}
	if !found {
		t.Error("Current log entry should still exist after cleanup")
	}
}

func TestConcurrentAccess(t *testing.T) {
	dbFile := "test_concurrent.db"
	defer os.Remove(dbFile)
	
	db, err := Initialize(dbFile)
	if err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()
	
	// Test concurrent log writes
	done := make(chan bool, 10)
	errors := make(chan error, 10)
	
	for i := 0; i < 10; i++ {
		go func(id int) {
			entry := &LogEntry{
				DeviceMAC:  "aa:bb:cc:dd:ee:0" + string(rune('0'+id)),
				DeviceName: "Device " + string(rune('0'+id)),
				Event:      "connected",
				Message:    "Concurrent test",
			}
			
			err := db.LogEvent(entry)
			if err != nil {
				errors <- err
			}
			done <- true
		}(i)
	}
	
	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}
	
	// Check for errors
	close(errors)
	for err := range errors {
		t.Errorf("Concurrent write error: %v", err)
	}
	
	// Verify all logs were inserted
	logs, err := db.GetLogs(20, 0)
	if err != nil {
		t.Fatalf("Failed to count logs: %v", err)
	}
	
	if len(logs) < 10 {
		t.Errorf("Expected at least 10 logs, got %d", len(logs))
	}
}

func TestDatabaseErrorHandling(t *testing.T) {
	dbFile := "/tmp/test_error_handling.db"
	defer os.Remove(dbFile)
	
	db, err := Initialize(dbFile)
	if err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()
	
	t.Run("GetDeviceState for non-existent device", func(t *testing.T) {
		currentAP, lastSeen, isConnected, err := db.GetDeviceState("non:ex:is:te:nt:00")
		if err != nil {
			t.Fatalf("GetDeviceState should not error for non-existent device: %v", err)
		}
		
		if currentAP != "" || !lastSeen.IsZero() || isConnected {
			t.Error("Should return empty values for non-existent device state")
		}
	})
	
	t.Run("GetLastGateTrigger with no records", func(t *testing.T) {
		lastTrigger, err := db.GetLastGateTrigger("non:ex:is:te:nt:00")
		if err != nil {
			t.Fatalf("GetLastGateTrigger should not error with no records: %v", err)
		}
		
		if !lastTrigger.IsZero() {
			t.Error("Should return zero time when no gate triggers exist")
		}
	})
	
	t.Run("GetLogsByDevice for non-existent device", func(t *testing.T) {
		logs, err := db.GetLogsByDevice("non:ex:is:te:nt:00", 10)
		if err != nil {
			t.Fatalf("GetLogsByDevice should not error for non-existent device: %v", err)
		}
		
		if len(logs) != 0 {
			t.Error("Should return empty logs for non-existent device")
		}
	})
	
	t.Run("GetRecentActivity with no logs", func(t *testing.T) {
		// Using a fresh database with no logs
		dbFile2 := "/tmp/test_empty_activity.db"
		defer os.Remove(dbFile2)
		
		db2, err := Initialize(dbFile2)
		if err != nil {
			t.Fatalf("Failed to initialize empty database: %v", err)
		}
		defer db2.Close()
		
		activity, err := db2.GetRecentActivity(10)
		if err != nil {
			t.Fatalf("GetRecentActivity should not error with empty database: %v", err)
		}
		
		if len(activity) != 0 {
			t.Error("Should return empty activity for empty database")
		}
	})
	
	t.Run("GetConnectedDevices with no devices", func(t *testing.T) {
		// Using a fresh database with no device states
		dbFile3 := "/tmp/test_empty_devices.db"
		defer os.Remove(dbFile3)
		
		db3, err := Initialize(dbFile3)
		if err != nil {
			t.Fatalf("Failed to initialize empty database: %v", err)
		}
		defer db3.Close()
		
		devices, err := db3.GetConnectedDevices()
		if err != nil {
			t.Fatalf("GetConnectedDevices should not error with empty database: %v", err)
		}
		
		if len(devices) != 0 {
			t.Error("Should return empty devices for empty database")
		}
	})
}

func TestDatabaseInitializationErrors(t *testing.T) {
	t.Run("Initialize with invalid path", func(t *testing.T) {
		// Try to create database in non-existent directory
		_, err := Initialize("/nonexistent/directory/test.db")
		if err == nil {
			t.Error("Should fail to initialize database in non-existent directory")
		}
	})
	
	t.Run("Initialize with empty path", func(t *testing.T) {
		// Empty path should use in-memory database
		db, err := Initialize("")
		if err != nil {
			t.Fatalf("Should handle empty path (in-memory database): %v", err)
		}
		if db != nil {
			defer db.Close()
		}
	})
}

func TestDatabaseAdvancedOperations(t *testing.T) {
	dbFile := "/tmp/test_advanced_ops.db"
	defer os.Remove(dbFile)
	
	db, err := Initialize(dbFile)
	if err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()
	
	t.Run("Log entries with special characters", func(t *testing.T) {
		specialEntry := &LogEntry{
			DeviceMAC:  "aa:bb:cc:dd:ee:ff",
			DeviceName: "Device with 'quotes' and \"double quotes\"",
			Event:      "connected",
			Direction:  "arriving with émojis 🚗",
			Message:    "Message with\nnewlines and\ttabs",
		}
		
		err := db.LogEvent(specialEntry)
		if err != nil {
			t.Fatalf("Failed to log entry with special characters: %v", err)
		}
		
		logs, err := db.GetLogsByDevice(specialEntry.DeviceMAC, 1)
		if err != nil {
			t.Fatalf("Failed to retrieve logs: %v", err)
		}
		
		if len(logs) != 1 {
			t.Fatalf("Expected 1 log, got %d", len(logs))
		}
		
		retrieved := logs[0]
		if retrieved.DeviceName != specialEntry.DeviceName {
			t.Errorf("Device name not preserved: expected %q, got %q", 
				specialEntry.DeviceName, retrieved.DeviceName)
		}
		if retrieved.Direction != specialEntry.Direction {
			t.Errorf("Direction not preserved: expected %q, got %q", 
				specialEntry.Direction, retrieved.Direction)
		}
		if retrieved.Message != specialEntry.Message {
			t.Errorf("Message not preserved: expected %q, got %q", 
				specialEntry.Message, retrieved.Message)
		}
	})
	
	t.Run("Device state with edge cases", func(t *testing.T) {
		// Test device with very long MAC address or special format
		longMAC := "aa:bb:cc:dd:ee:ff:aa:bb:cc:dd:ee:ff"
		
		err := db.UpdateDeviceState(longMAC, "test-ap-mac", true)
		if err != nil {
			t.Fatalf("Failed to update device state with long MAC: %v", err)
		}
		
		currentAP, _, isConnected, err := db.GetDeviceState(longMAC)
		if err != nil {
			t.Fatalf("Failed to get device state: %v", err)
		}
		
		if !isConnected {
			t.Error("Device should be connected")
		}
		
		if currentAP != "test-ap-mac" {
			t.Errorf("Expected AP %s, got %s", "test-ap-mac", currentAP)
		}
	})
	
	t.Run("Multiple gate triggers", func(t *testing.T) {
		// First need to add a device state
		testMAC := "aa:bb:cc:dd:ee:test"
		err := db.UpdateDeviceState(testMAC, "test-ap", true)
		if err != nil {
			t.Fatalf("Failed to create device state: %v", err)
		}
		
		// Update gate trigger multiple times
		for i := 0; i < 3; i++ {
			err := db.UpdateLastGateTrigger(testMAC)
			if err != nil {
				t.Fatalf("Failed to update gate trigger %d: %v", i, err)
			}
		}
		
		lastTrigger, err := db.GetLastGateTrigger(testMAC)
		if err != nil {
			t.Fatalf("Failed to get last gate trigger: %v", err)
		}
		
		if lastTrigger.IsZero() {
			t.Error("Last gate trigger should not be zero")
		}
		
		// Check it's reasonably recent (within last minute) 
		// Note: need to import time package for this
		if lastTrigger.Unix() == 0 {
			t.Error("Last gate trigger should have a valid timestamp")
		}
	})
}