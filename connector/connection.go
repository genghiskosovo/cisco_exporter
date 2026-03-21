package connector

import (
	"bufio"
	"io"
	"regexp"
	"strings"
	"time"

	"github.com/lwlcom/cisco_exporter/config"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"
)

var promptRegexp = regexp.MustCompile(`.+#\s?$`)

// NewSSSHConnection connects to device
func NewSSSHConnection(device *Device, cfg *config.Config) (*SSHConnection, error) {
	deviceConfig := device.DeviceConfig

	legacyCiphers := cfg.LegacyCiphers
	if deviceConfig.LegacyCiphers != nil {
		legacyCiphers = *deviceConfig.LegacyCiphers
	}

	batchSize := cfg.BatchSize
	if deviceConfig.BatchSize != nil {
		batchSize = *deviceConfig.BatchSize
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
		batchSize:    batchSize,
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
	stdin        io.WriteCloser
	stdout       io.Reader
	buf          *bufio.Reader
	session      *ssh.Session
	batchSize    int
	clientConfig *ssh.ClientConfig
	noEcho       bool // true when device rejected PTY — commands are not echoed back
}

// Connect connects to the device
func (c *SSHConnection) Connect() error {
	var err error

	log.Debugf("[%s] dialing SSH", c.Host)
	c.client, err = ssh.Dial("tcp", c.Host, c.clientConfig)
	if err != nil {
		return err
	}
	log.Debugf("[%s] SSH connection established", c.Host)

	session, err := c.client.NewSession()
	if err != nil {
		c.client.Conn.Close()
		return err
	}

	c.stdin, err = session.StdinPipe()
	if err != nil {
		session.Close()
		c.client.Conn.Close()
		return err
	}

	c.stdout, err = session.StdoutPipe()
	if err != nil {
		session.Close()
		c.client.Conn.Close()
		return err
	}

	c.buf = bufio.NewReader(c.stdout)

	modes := ssh.TerminalModes{
		ssh.ECHO:  0,
		ssh.OCRNL: 0,
	}
	// RequestPty failure is non-fatal: IOS XR rejects PTY requests but still
	// allows shell access. Track whether PTY was accepted because devices with
	// a PTY echo commands back (used to detect end of command output), while
	// devices without a PTY do not.
	if err = session.RequestPty("vt100", 24, 2000, modes); err != nil {
		log.Debugf("[%s] PTY request rejected (non-fatal): %v", c.Host, err)
		c.noEcho = true
	}

	if err = session.Shell(); err != nil {
		session.Close()
		c.client.Conn.Close()
		return err
	}
	c.session = session
	log.Debugf("[%s] shell opened, waiting for initial prompt", c.Host)

	if _, err = c.RunCommand(""); err != nil {
		return errors.Wrap(err, "timeout waiting for initial prompt — consider increasing ssh.timeout")
	}
	if _, err = c.RunCommand("terminal length 0"); err != nil {
		log.Debugf("[%s] 'terminal length 0' failed (non-fatal): %v", c.Host, err)
	}

	return nil
}

type result struct {
	output string
	err    error
}

// RunCommand runs a command against the device
func (c *SSHConnection) RunCommand(cmd string) (string, error) {
	log.Debugf("[%s] sending: %q", c.Host, cmd)

	if _, err := io.WriteString(c.stdin, cmd+"\n"); err != nil {
		return "", err
	}

	outputChan := make(chan result, 1)
	go func() {
		c.readln(outputChan, cmd, c.buf)
	}()
	select {
	case res := <-outputChan:
		if res.err != nil {
			log.Debugf("[%s] command %q returned error: %v", c.Host, cmd, res.err)
		} else {
			log.Debugf("[%s] command %q completed (%d bytes)", c.Host, cmd, len(res.output))
		}
		return res.output, res.err
	case <-time.After(c.clientConfig.Timeout):
		log.Debugf("[%s] command %q timed out, closing connection", c.Host, cmd)
		// Close so the reader goroutine unblocks and exits cleanly.
		c.Close()
		return "", errors.New("Timeout reached")
	}
}

// Close closes connection
func (c *SSHConnection) Close() {
	if c.session != nil {
		c.session.Close()
	}
	if c.client != nil && c.client.Conn != nil {
		c.client.Conn.Close()
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

// readln reads from r until the shell prompt is detected.
// Devices with a PTY echo commands back, so we require both the command text
// and the prompt to appear in the output before returning. Devices without a
// PTY (e.g. IOS XR, c.noEcho=true) do not echo, so we wait for the prompt alone.
func (c *SSHConnection) readln(ch chan result, cmd string, r io.Reader) {
	buf := make([]byte, c.batchSize)
	loadStr := ""
	for {
		n, err := r.Read(buf)
		if err != nil {
			log.Debugf("[%s] readln read error: %v", c.Host, err)
			ch <- result{output: "", err: err}
			return
		}
		chunk := string(buf[:n])
		log.Debugf("[%s] readln received %d bytes: %q", c.Host, n, chunk)
		loadStr += chunk
		if promptRegexp.MatchString(loadStr) {
			if cmd == "" || c.noEcho || strings.Contains(loadStr, cmd) {
				log.Debugf("[%s] prompt detected", c.Host)
				break
			}
		}
	}
	loadStr = strings.Replace(loadStr, "\r", "", -1)
	ch <- result{output: loadStr, err: nil}
}
