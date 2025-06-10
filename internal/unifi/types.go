package unifi

// AccessPoint represents a UniFi access point
type AccessPoint struct {
	ID      string `json:"_id"`
	MAC     string `json:"mac"`
	Name    string `json:"name"`
	Model   string `json:"model"`
	IP      string `json:"ip"`
	State   int    `json:"state"`
	Adopted bool   `json:"adopted"`
	SiteID  string `json:"site_id"`
}

// Site represents a UniFi site
type Site struct {
	ID          string `json:"_id"`
	Name        string `json:"name"`
	Description string `json:"desc"`
	Role        string `json:"role"`
	Hidden      bool   `json:"attr_hidden_id,omitempty"`
	NoDelete    bool   `json:"attr_no_delete,omitempty"`
}

// WirelessClient represents a wireless client device
type WirelessClient struct {
	ID               string `json:"_id"`
	MAC              string `json:"mac"`
	Name             string `json:"name,omitempty"`
	Hostname         string `json:"hostname,omitempty"`
	IP               string `json:"ip"`
	AP_MAC           string `json:"ap_mac"`
	ESSID            string `json:"essid"`
	Radio            string `json:"radio"`
	RadioProto       string `json:"radio_proto"`
	Channel          int    `json:"channel"`
	PowerSave        bool   `json:"powersave_enabled"`
	LastSeen         int64  `json:"last_seen"`
	Uptime           int64  `json:"uptime"`
	TxBytes          int64  `json:"tx_bytes"`
	RxBytes          int64  `json:"rx_bytes"`
	Signal           int    `json:"signal"`
	Noise            int    `json:"noise"`
	IsGuest          bool   `json:"is_guest"`
	IsWired          bool   `json:"is_wired"`
	Authorized       bool   `json:"authorized"`
	QosPolicyApplied bool   `json:"qos_policy_applied"`
}
