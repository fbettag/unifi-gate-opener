package gate

import (
	"fmt"
	"net/http"
	"time"

	"github.com/sirupsen/logrus"
)

type Controller struct {
	triggerURL string
	client     *http.Client
	logger     *logrus.Logger
}

func NewController(triggerURL string, logger *logrus.Logger) *Controller {
	return &Controller{
		triggerURL: triggerURL,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
		logger: logger,
	}
}

func (c *Controller) OpenGate() error {
	if c.triggerURL == "" {
		return fmt.Errorf("gate trigger URL not configured")
	}

	c.logger.Infof("Triggering gate open via: %s", c.triggerURL)

	resp, err := c.client.Get(c.triggerURL)
	if err != nil {
		return fmt.Errorf("failed to trigger gate: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("gate trigger returned status %d", resp.StatusCode)
	}

	c.logger.Info("Gate opened successfully")
	return nil
}

func (c *Controller) TestConnection() error {
	if c.triggerURL == "" {
		return fmt.Errorf("gate trigger URL not configured")
	}

	// Try to reach the endpoint with a HEAD request
	req, err := http.NewRequest("HEAD", c.triggerURL, nil)
	if err != nil {
		return err
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to connect to gate controller: %w", err)
	}
	defer resp.Body.Close()

	// Accept any 2xx or 4xx status (4xx might mean auth required which is still a valid endpoint)
	if resp.StatusCode >= 500 {
		return fmt.Errorf("gate controller returned server error: %d", resp.StatusCode)
	}

	return nil
}

func (c *Controller) UpdateURL(newURL string) {
	c.triggerURL = newURL
}
