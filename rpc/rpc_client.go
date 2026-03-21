package rpc

import (
	"errors"
	"strings"

	"github.com/lwlcom/cisco_exporter/connector"
	log "github.com/sirupsen/logrus"
)

const (
	IOSXE string = "IOSXE"
	NXOS  string = "NXOS"
	IOS   string = "IOS"
	IOSXR string = "IOSXR"
)

// Client sends commands to a Cisco device
type Client struct {
	conn   *connector.SSHConnection
	OSType string
}

// NewClient creates a new client connection
func NewClient(ssh *connector.SSHConnection) *Client {
	return &Client{conn: ssh}
}

// Identify tries to identify the OS running on a Cisco device
func (c *Client) Identify() error {
	output, err := c.RunCommand("show version")
	if err != nil {
		return err
	}
	switch {
	case strings.Contains(output, "IOS XE"):
		c.OSType = IOSXE
	case strings.Contains(output, "NX-OS"):
		c.OSType = NXOS
	case strings.Contains(output, "IOS Software"):
		c.OSType = IOS
	case strings.Contains(output, "iosxr"):
		c.OSType = IOSXR
	default:
		return errors.New("Unknown OS")
	}
	log.Debugf("Host %s identified as: %s", c.conn.Host, c.OSType)
	return nil
}

// RunCommand runs a command on a Cisco device
func (c *Client) RunCommand(cmd string) (string, error) {
	log.Debugf("Running command on %s: %s", c.conn.Host, cmd)
	output, err := c.conn.RunCommand(cmd)
	if err != nil {
		log.Debugf("Command error on %s: %s", c.conn.Host, err.Error())
		return "", err
	}
	return output, nil
}
