package proxmox

import (
	"errors"
	"os"
)

type Config struct {
	Host        string
	TokenID     string
	TokenSecret string
	InsecureTLS bool
}

func (c *Config) Load() error {
	c.Host = os.Getenv("PROXMOX_HOST")
	c.TokenID = os.Getenv("PROXMOX_TOKEN_ID")
	c.TokenSecret = os.Getenv("PROXMOX_TOKEN_SECRET")
	c.InsecureTLS = os.Getenv("PROXMOX_INSECURE_TLS") == "true"

	if c.Host == "" {
		return errors.New("PROXMOX_HOST is required")
	}
	if c.TokenID == "" {
		return errors.New("PROXMOX_TOKEN_ID is required")
	}
	if c.TokenSecret == "" {
		return errors.New("PROXMOX_TOKEN_SECRET is required")
	}

	return nil
}

func (c *Config) BaseURL() string {
	return "https://" + c.Host + ":8006/api2/json"
}
