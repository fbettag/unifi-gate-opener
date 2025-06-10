package unifi

import (
	"fmt"
	"strings"
	"time"

	"github.com/unpoller/unifi/v5"
)

// Client wraps the unpoller/unifi client
type Client struct {
	client   *unifi.Unifi
	baseURL  string
	username string
	password string
	logger   Logger
}

// NewClient creates a new UniFi client using the unpoller/unifi library
func NewClient(baseURL, username, password string, logger Logger) *Client {
	return &Client{
		baseURL:  strings.TrimRight(baseURL, "/"),
		username: username,
		password: password,
		logger:   logger,
	}
}

// Login authenticates with the UniFi controller
func (c *Client) Login() error {
	c.logger.Debugf("Attempting to login to UniFi controller at %s", c.baseURL)
	c.logger.Debugf("Username: %s", c.username)

	// Create config
	config := &unifi.Config{
		User:      c.username,
		Pass:      c.password,
		URL:       c.baseURL,
		VerifySSL: false, // Allow self-signed certificates
		Timeout:   30 * time.Second,
		ErrorLog:  c.logger.Errorf,
		DebugLog:  c.logger.Debugf,
	}

	// Create client
	client, err := unifi.NewUnifi(config)
	if err != nil {
		c.logger.Errorf("Failed to create UniFi client: %v", err)
		return fmt.Errorf("failed to create UniFi client: %w", err)
	}

	// Login
	c.logger.Debugf("Calling Login() on UniFi client...")
	if err := client.Login(); err != nil {
		c.logger.Errorf("Login failed: %v", err)
		return fmt.Errorf("failed to login: %w", err)
	}

	c.client = client
	c.logger.Infof("Successfully logged in to UniFi controller")
	return nil
}

// GetSites returns all sites
func (c *Client) GetSites() ([]Site, error) {
	if c.client == nil {
		return nil, fmt.Errorf("not logged in")
	}

	unifiSites, err := c.client.GetSites()
	if err != nil {
		return nil, fmt.Errorf("failed to get sites: %w", err)
	}

	// Convert to our Site type
	sites := make([]Site, len(unifiSites))
	for i, s := range unifiSites {
		sites[i] = Site{
			ID:          s.ID,
			Name:        s.Name,
			Description: s.Desc,
		}
	}

	return sites, nil
}

// GetAccessPoints returns all access points for a site
func (c *Client) GetAccessPoints(siteID string) ([]AccessPoint, error) {
	if c.client == nil {
		return nil, fmt.Errorf("not logged in")
	}

	// Get devices for all sites or specific site
	var sites []*unifi.Site
	if siteID != "" {
		sites = []*unifi.Site{{Name: siteID}}
	}

	devices, err := c.client.GetDevices(sites)
	if err != nil {
		return nil, fmt.Errorf("failed to get devices: %w", err)
	}

	// Filter for access points
	var aps []AccessPoint
	for _, d := range devices.UAPs {
		ap := AccessPoint{
			ID:      d.ID,
			MAC:     d.Mac,
			Name:    d.Name,
			Model:   d.Model,
			IP:      d.IP,
			State:   int(d.State.Val),
			Adopted: d.Adopted.Val,
			SiteID:  d.SiteName,
		}
		aps = append(aps, ap)
	}

	return aps, nil
}

// GetActiveClients returns all active wireless clients for a site
func (c *Client) GetActiveClients(siteID string) ([]WirelessClient, error) {
	if c.client == nil {
		return nil, fmt.Errorf("not logged in")
	}

	// Get clients for specific site
	sites := []*unifi.Site{{Name: siteID}}
	clients, err := c.client.GetClients(sites)
	if err != nil {
		return nil, fmt.Errorf("failed to get clients: %w", err)
	}

	// Convert to our WirelessClient type and filter for active clients
	var activeClients []WirelessClient
	for _, client := range clients {
		// Check if client is wireless and was seen recently (within 5 minutes)
		lastSeenTime := time.Unix(int64(client.LastSeen.Val), 0)
		if !client.IsWired.Val && time.Since(lastSeenTime) < 5*time.Minute {
			wc := WirelessClient{
				ID:         client.ID,
				MAC:        client.Mac,
				Name:       client.Name,
				Hostname:   client.Hostname,
				IP:         client.IP,
				AP_MAC:     client.ApMac,
				ESSID:      client.Essid,
				Radio:      client.Radio,
				RadioProto: client.RadioProto,
				Channel:    int(client.Channel.Val),
				LastSeen:   int64(client.LastSeen.Val),
				Uptime:     int64(client.Uptime.Val),
				TxBytes:    int64(client.TxBytes.Val),
				RxBytes:    int64(client.RxBytes.Val),
				Signal:     int(client.Signal.Val),
				Noise:      int(client.Noise.Val),
				IsGuest:    client.IsGuest.Val,
				IsWired:    client.IsWired.Val,
				Authorized: true, // UniFi API doesn't provide this field directly
			}
			activeClients = append(activeClients, wc)
		}
	}

	return activeClients, nil
}

// GetClientHistory is not directly supported by the library, returning empty for now
func (c *Client) GetClientHistory(siteID, mac string, hours int) ([]WirelessClient, error) {
	// This would require implementing event parsing from the UniFi API
	// For now, return empty
	return []WirelessClient{}, nil
}
