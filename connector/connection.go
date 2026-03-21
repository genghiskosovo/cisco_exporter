package connector

import (
	"io"
	"time"

	"github.com/lwlcom/cisco_exporter/config"
	"github.com/pkg/errors"
	"golang.org/x/crypto/ssh"
)

// NewSSSHConnection connects to device
func NewSSSHConnection(device *Device, cfg *config.Config) (*SSHConnection, error) {
	deviceConfig := device.DeviceConfig

	legacyCiphers := cfg.LegacyCiphers
	if deviceConfig.LegacyCiphers != nil {
		legacyCiphers = *deviceConfig.LegacyCiphers
	}

	timeout := cfg.Timeout
	if deviceConfig.Timeout != nil {
		timeout = *deviceConfig.Timeout
	}

	sshConfig := &ssh.ClientConfig{
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         time.Duration(timeout) * time.Second,
	}
	if legacyCiphers {
		sshConfig.SetDefaults()
		sshConfig.Ciphers = append(sshConfig.Ciphers, "aes128-cbc", "3des-cbc")
		sshConfig.KeyExchanges = append(sshConfig.KeyExchanges, "diffie-hellman-group1-sha1")
		sshConfig.MACs = append(sshConfig.MACs, "hmac-sha1")
	}

	device.Auth(sshConfig)

	c := &SSHConnection{
		Host:         device.Host + ":" + device.Port,
		timeout:      time.Duration(timeout) * time.Second,
		clientConfig: sshConfig,
	}

	if err := c.Connect(); err != nil {
		return nil, err
	}

	return c, nil
}

// SSHConnection encapsulates the connection to the device
type SSHConnection struct {
	client       *ssh.Client
	Host         string
	timeout      time.Duration
	clientConfig *ssh.ClientConfig
}

// Connect connects to the device
func (c *SSHConnection) Connect() error {
	var err error
	c.client, err = ssh.Dial("tcp", c.Host, c.clientConfig)
	return err
}

// RunCommand runs a command against the device using a dedicated exec session.
// Each call opens a new SSH channel on the existing connection, sends the
// command, and returns the full output once the command exits. No PTY or
// interactive shell is needed; Cisco devices do not paginate output in exec mode.
func (c *SSHConnection) RunCommand(cmd string) (string, error) {
	session, err := c.client.NewSession()
	if err != nil {
		return "", err
	}
	defer session.Close()

	type result struct {
		out []byte
		err error
	}
	ch := make(chan result, 1)
	go func() {
		out, err := session.Output(cmd)
		ch <- result{out, err}
	}()

	select {
	case res := <-ch:
		return string(res.out), res.err
	case <-time.After(c.timeout):
		return "", errors.New("timeout reached")
	}
}

// Close closes the connection
func (c *SSHConnection) Close() {
	if c.client != nil {
		c.client.Close()
	}
}

func loadPrivateKey(r io.Reader) (ssh.AuthMethod, error) {
	b, err := io.ReadAll(r)
	if err != nil {
		return nil, errors.Wrap(err, "could not read from reader")
	}

	key, err := ssh.ParsePrivateKey(b)
	if err != nil {
		return nil, errors.Wrap(err, "could not parse private key")
	}

	return ssh.PublicKeys(key), nil
}
