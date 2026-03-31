package rpc

import (
	"errors"
	"strings"
	"time"

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
	conn            *connector.SSHConnection
	Debug           bool
	OSType          string
	ShowVersionCache string // cached output of "show version" from Identify()
}

// NewClient creates a new client connection
func NewClient(ssh *connector.SSHConnection, debug bool) *Client {
	rpc := &Client{conn: ssh, Debug: debug}

	return rpc
}

// Identify tries to identify the OS running on a Cisco device.
// IOS-XR "show version" dumps all installed packages and can take 8+ seconds.
// "show version brief" is IOS-XR specific and returns the same header in <1s.
// Strategy: try "show version brief" first — if accepted and contains "IOS XR"
// we're done. If the device rejects the command (invalid command marker in output)
// fall back to "show version" for IOS/IOS-XE/NX-OS.
func (c *Client) Identify() error {
	brief, err := c.RunCommand("show version brief")
	if err != nil {
		return err
	}
	if strings.Contains(brief, "IOS XR") {
		c.OSType = IOSXR
		c.ShowVersionCache = brief
		if c.Debug {
			log.Printf("Host %s identified as: %s\n", c.conn.Host, c.OSType)
		}
		return nil
	}

	// "show version brief" was rejected or returned non-XR output —
	// fall back to full "show version" for IOS/IOS-XE/NX-OS
	output, err := c.RunCommand("show version")
	if err != nil {
		return err
	}
	c.ShowVersionCache = output
	switch {
	case strings.Contains(output, "IOS XE"):
		c.OSType = IOSXE
	case strings.Contains(output, "NX-OS"):
		c.OSType = NXOS
	case strings.Contains(output, "IOS Software"):
		c.OSType = IOS
	default:
		preview := output
		if len(preview) > 512 {
			preview = preview[:512]
		}
		log.Errorf("Unknown OS on %s — show version output (%d bytes): %q", c.conn.Host, len(output), preview)
		return errors.New("Unknown OS")
	}
	if c.Debug {
		log.Printf("Host %s identified as: %s\n", c.conn.Host, c.OSType)
	}
	return nil
}

// RunCommand runs a command on a Cisco device
func (c *Client) RunCommand(cmd string) (string, error) {
	if c.Debug {
		log.Printf("Running command on %s: %s\n", c.conn.Host, cmd)
	}
	start := time.Now()
	output, err := c.conn.RunCommand(cmd)
	elapsed := time.Since(start)
	if err != nil {
		log.Errorf("Command error on %s after %s: %s\n", c.conn.Host, elapsed.Round(time.Millisecond), err.Error())
		return "", err
	}
	if c.Debug {
		log.Printf("Command on %s took %s: %s\n", c.conn.Host, elapsed.Round(time.Millisecond), cmd)
	}
	return output, nil
}
