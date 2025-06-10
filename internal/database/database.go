package database

import (
	"database/sql"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

type DB struct {
	*sql.DB
}

type LogEntry struct {
	ID         int64     `json:"id"`
	Timestamp  time.Time `json:"timestamp"`
	DeviceMAC  string    `json:"device_mac"`
	DeviceName string    `json:"device_name"`
	Event      string    `json:"event"`     // "connected", "disconnected", "gate_triggered", "gate_skipped"
	Direction  string    `json:"direction"` // "arriving", "leaving", "unknown"
	FromAP     string    `json:"from_ap,omitempty"`
	ToAP       string    `json:"to_ap,omitempty"`
	GateOpened bool      `json:"gate_opened"`
	Message    string    `json:"message"`
}

func Initialize(dbPath string) (*DB, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, err
	}

	// Create tables
	if err := createTables(db); err != nil {
		return nil, err
	}

	return &DB{db}, nil
}

func createTables(db *sql.DB) error {
	schema := `
	CREATE TABLE IF NOT EXISTS logs (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
		device_mac TEXT NOT NULL,
		device_name TEXT,
		event TEXT NOT NULL,
		direction TEXT,
		from_ap TEXT,
		to_ap TEXT,
		gate_opened BOOLEAN DEFAULT FALSE,
		message TEXT
	);

	CREATE INDEX IF NOT EXISTS idx_logs_timestamp ON logs(timestamp);
	CREATE INDEX IF NOT EXISTS idx_logs_device_mac ON logs(device_mac);
	CREATE INDEX IF NOT EXISTS idx_logs_event ON logs(event);

	CREATE TABLE IF NOT EXISTS device_states (
		mac TEXT PRIMARY KEY,
		current_ap TEXT,
		last_seen DATETIME,
		is_connected BOOLEAN DEFAULT FALSE,
		last_gate_trigger DATETIME
	);
	`

	_, err := db.Exec(schema)
	return err
}

func (db *DB) LogEvent(entry *LogEntry) error {
	query := `
		INSERT INTO logs (device_mac, device_name, event, direction, from_ap, to_ap, gate_opened, message)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`
	_, err := db.Exec(query, entry.DeviceMAC, entry.DeviceName, entry.Event, entry.Direction,
		entry.FromAP, entry.ToAP, entry.GateOpened, entry.Message)
	return err
}

func (db *DB) GetLogs(limit int, offset int) ([]LogEntry, error) {
	query := `
		SELECT id, timestamp, device_mac, device_name, event, direction, 
		       COALESCE(from_ap, ''), COALESCE(to_ap, ''), gate_opened, message
		FROM logs
		ORDER BY timestamp DESC
		LIMIT ? OFFSET ?
	`

	rows, err := db.Query(query, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []LogEntry
	for rows.Next() {
		var log LogEntry
		err := rows.Scan(&log.ID, &log.Timestamp, &log.DeviceMAC, &log.DeviceName,
			&log.Event, &log.Direction, &log.FromAP, &log.ToAP, &log.GateOpened, &log.Message)
		if err != nil {
			return nil, err
		}
		logs = append(logs, log)
	}

	return logs, nil
}

func (db *DB) GetLogsByDevice(mac string, limit int) ([]LogEntry, error) {
	query := `
		SELECT id, timestamp, device_mac, device_name, event, direction, 
		       COALESCE(from_ap, ''), COALESCE(to_ap, ''), gate_opened, message
		FROM logs
		WHERE device_mac = ?
		ORDER BY timestamp DESC
		LIMIT ?
	`

	rows, err := db.Query(query, mac, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []LogEntry
	for rows.Next() {
		var log LogEntry
		err := rows.Scan(&log.ID, &log.Timestamp, &log.DeviceMAC, &log.DeviceName,
			&log.Event, &log.Direction, &log.FromAP, &log.ToAP, &log.GateOpened, &log.Message)
		if err != nil {
			return nil, err
		}
		logs = append(logs, log)
	}

	return logs, nil
}

func (db *DB) UpdateDeviceState(mac, currentAP string, isConnected bool) error {
	query := `
		INSERT INTO device_states (mac, current_ap, last_seen, is_connected)
		VALUES (?, ?, CURRENT_TIMESTAMP, ?)
		ON CONFLICT(mac) DO UPDATE SET
			current_ap = excluded.current_ap,
			last_seen = excluded.last_seen,
			is_connected = excluded.is_connected
	`
	_, err := db.Exec(query, mac, currentAP, isConnected)
	return err
}

func (db *DB) GetDeviceState(mac string) (currentAP string, lastSeen time.Time, isConnected bool, err error) {
	query := `SELECT current_ap, last_seen, is_connected FROM device_states WHERE mac = ?`
	err = db.QueryRow(query, mac).Scan(&currentAP, &lastSeen, &isConnected)
	if err == sql.ErrNoRows {
		return "", time.Time{}, false, nil
	}
	return
}

func (db *DB) UpdateLastGateTrigger(mac string) error {
	query := `
		UPDATE device_states 
		SET last_gate_trigger = CURRENT_TIMESTAMP 
		WHERE mac = ?
	`
	_, err := db.Exec(query, mac)
	return err
}

func (db *DB) GetLastGateTrigger(mac string) (time.Time, error) {
	var lastTrigger sql.NullTime
	query := `SELECT last_gate_trigger FROM device_states WHERE mac = ?`
	err := db.QueryRow(query, mac).Scan(&lastTrigger)
	if err == sql.ErrNoRows {
		return time.Time{}, nil
	}
	if err != nil {
		return time.Time{}, err
	}
	if lastTrigger.Valid {
		return lastTrigger.Time, nil
	}
	return time.Time{}, nil
}

func (db *DB) GetRecentActivity(hours int) ([]LogEntry, error) {
	query := `
		SELECT id, timestamp, device_mac, device_name, event, direction, 
		       COALESCE(from_ap, ''), COALESCE(to_ap, ''), gate_opened, message
		FROM logs
		WHERE timestamp > datetime('now', '-' || ? || ' hours')
		ORDER BY timestamp DESC
	`

	rows, err := db.Query(query, hours)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []LogEntry
	for rows.Next() {
		var log LogEntry
		err := rows.Scan(&log.ID, &log.Timestamp, &log.DeviceMAC, &log.DeviceName,
			&log.Event, &log.Direction, &log.FromAP, &log.ToAP, &log.GateOpened, &log.Message)
		if err != nil {
			return nil, err
		}
		logs = append(logs, log)
	}

	return logs, nil
}

func (db *DB) GetConnectedDevices() (map[string]bool, error) {
	query := `SELECT mac FROM device_states WHERE is_connected = true`
	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	connected := make(map[string]bool)
	for rows.Next() {
		var mac string
		if err := rows.Scan(&mac); err != nil {
			return nil, err
		}
		connected[mac] = true
	}

	return connected, nil
}

// DeleteOldLogs deletes log entries older than the specified number of days
func (db *DB) DeleteOldLogs(daysToKeep int) (int64, error) {
	query := `DELETE FROM logs WHERE timestamp < datetime('now', '-' || ? || ' days')`
	result, err := db.Exec(query, daysToKeep)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}
