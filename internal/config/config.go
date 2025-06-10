package config

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"os"
	"time"

	"github.com/spf13/viper"
	"golang.org/x/crypto/bcrypt"
)

type Config struct {
	Admin         AdminConfig    `mapstructure:"admin"`
	UniFi         UniFiConfig    `mapstructure:"unifi"`
	Shelly        ShellyConfig   `mapstructure:"shelly"`
	Gate          GateConfig     `mapstructure:"gate"`
	DatabasePath  string         `mapstructure:"database_path"`
	SessionSecret string         `mapstructure:"session_secret"`
	Devices       []DeviceConfig `mapstructure:"devices"`
	SetupComplete bool           `mapstructure:"setup_complete"`
}

type AdminConfig struct {
	Username     string `mapstructure:"username"`
	PasswordHash string `mapstructure:"password_hash"`
}

type UniFiConfig struct {
	ControllerURL string `mapstructure:"controller_url"`
	Username      string `mapstructure:"username"`
	Password      string `mapstructure:"password"`
	SiteID        string `mapstructure:"site_id"`
	GateAPMAC     string `mapstructure:"gate_ap_mac"`
	PollInterval  int    `mapstructure:"poll_interval"` // seconds
}

type ShellyConfig struct {
	TriggerURL string `mapstructure:"trigger_url"`
}

type GateConfig struct {
	OpenDuration int  `mapstructure:"open_duration"` // minutes (also used as cooldown)
	LogActivity  bool `mapstructure:"log_activity"`  // whether to log device activity
}

type DeviceConfig struct {
	MAC           string    `mapstructure:"mac" json:"mac"`
	Name          string    `mapstructure:"name" json:"name"`
	Enabled       bool      `mapstructure:"enabled" json:"enabled"`
	LastSeen      time.Time `mapstructure:"last_seen" json:"last_seen"`
	LastTriggered time.Time `mapstructure:"last_triggered" json:"last_triggered"`
}

func LoadOrInitialize(configPath string) (*Config, error) {
	viper.SetConfigFile(configPath)
	viper.SetConfigType("yaml")

	// Set defaults
	viper.SetDefault("database_path", "gate_opener.db")
	viper.SetDefault("unifi.poll_interval", 1)
	viper.SetDefault("unifi.site_id", "default")
	viper.SetDefault("gate.open_duration", 10)
	viper.SetDefault("gate.log_activity", false)
	viper.SetDefault("setup_complete", false)

	// Check if config file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		// Create new config with defaults
		cfg := &Config{
			DatabasePath:  viper.GetString("database_path"),
			SessionSecret: generateSessionSecret(),
			UniFi: UniFiConfig{
				PollInterval: viper.GetInt("unifi.poll_interval"),
				SiteID:       viper.GetString("unifi.site_id"),
			},
			Gate: GateConfig{
				OpenDuration: viper.GetInt("gate.open_duration"),
			},
			SetupComplete: false,
		}

		// Save initial config
		if err := SaveConfig(configPath, cfg); err != nil {
			return nil, err
		}

		return cfg, nil
	}

	// Read existing config
	if err := viper.ReadInConfig(); err != nil {
		return nil, err
	}

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, err
	}

	// Ensure session secret exists
	if cfg.SessionSecret == "" {
		cfg.SessionSecret = generateSessionSecret()
		if err := SaveConfig(configPath, &cfg); err != nil {
			return nil, err
		}
	}

	return &cfg, nil
}

func SaveConfig(configPath string, cfg *Config) error {
	viper.Set("admin.username", cfg.Admin.Username)
	viper.Set("admin.password_hash", cfg.Admin.PasswordHash)

	viper.Set("unifi.controller_url", cfg.UniFi.ControllerURL)
	viper.Set("unifi.username", cfg.UniFi.Username)
	viper.Set("unifi.password", cfg.UniFi.Password)
	viper.Set("unifi.site_id", cfg.UniFi.SiteID)
	viper.Set("unifi.gate_ap_mac", cfg.UniFi.GateAPMAC)
	viper.Set("unifi.poll_interval", cfg.UniFi.PollInterval)

	viper.Set("shelly.trigger_url", cfg.Shelly.TriggerURL)
	viper.Set("gate.open_duration", cfg.Gate.OpenDuration)
	viper.Set("gate.log_activity", cfg.Gate.LogActivity)
	viper.Set("database_path", cfg.DatabasePath)
	viper.Set("session_secret", cfg.SessionSecret)
	viper.Set("setup_complete", cfg.SetupComplete)

	// Manually set devices to ensure correct field names
	var devices []map[string]interface{}
	for _, d := range cfg.Devices {
		devices = append(devices, map[string]interface{}{
			"mac":            d.MAC,
			"name":           d.Name,
			"enabled":        d.Enabled,
			"last_seen":      d.LastSeen,
			"last_triggered": d.LastTriggered,
		})
	}
	viper.Set("devices", devices)

	return viper.WriteConfigAs(configPath)
}

func (c *Config) IsConfigured() bool {
	return c.SetupComplete && c.Admin.Username != "" && c.UniFi.ControllerURL != ""
}

func (c *Config) SetAdminPassword(password string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	c.Admin.PasswordHash = string(hash)
	return nil
}

func (c *Config) VerifyAdminPassword(password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(c.Admin.PasswordHash), []byte(password))
	return err == nil
}

func (c *Config) AddDevice(mac, name string) error {
	// Check if device already exists
	for _, d := range c.Devices {
		if d.MAC == mac {
			return errors.New("device already exists")
		}
	}

	c.Devices = append(c.Devices, DeviceConfig{
		MAC:     mac,
		Name:    name,
		Enabled: true,
	})

	return nil
}

func (c *Config) UpdateDevice(mac, name string, enabled bool) error {
	for i, d := range c.Devices {
		if d.MAC == mac {
			c.Devices[i].Name = name
			c.Devices[i].Enabled = enabled
			return nil
		}
	}
	return errors.New("device not found")
}

func (c *Config) RemoveDevice(mac string) error {
	for i, d := range c.Devices {
		if d.MAC == mac {
			c.Devices = append(c.Devices[:i], c.Devices[i+1:]...)
			return nil
		}
	}
	return errors.New("device not found")
}

func (c *Config) GetDevice(mac string) *DeviceConfig {
	for i := range c.Devices {
		if c.Devices[i].MAC == mac {
			return &c.Devices[i]
		}
	}
	return nil
}

func generateSessionSecret() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		// This should never happen with crypto/rand
		panic(err)
	}
	return base64.URLEncoding.EncodeToString(b)
}
