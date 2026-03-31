package optics

import (
	"errors"
	"regexp"
	"strings"

	"github.com/lwlcom/cisco_exporter/rpc"
	"github.com/lwlcom/cisco_exporter/util"
)

// sfpCapablePrefixes lists interface name prefixes that can physically hold an SFP/transceiver.
// FastEthernet, Vlan, Loopback, Tunnel, Port-channel etc. are excluded.
var sfpCapablePrefixes = []string{
	"GigabitEthernet",
	"TenGigabitEthernet",
	"TwentyFiveGigE",
	"FortyGigabitEthernet",
	"HundredGigabitEthernet",
	"Ethernet",
	"Eth", // NX-OS abbreviated physical ports (Eth1/1, Eth1/2, ...)
}

func hasSFPCapablePrefix(name string) bool {
	for _, prefix := range sfpCapablePrefixes {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return false
}

// ParseInterfaces parses cli output and returns list of SFP-capable interface names
func (c *opticsCollector) ParseInterfaces(ostype string, output string) ([]string, error) {
	if ostype != rpc.IOSXE && ostype != rpc.NXOS && ostype != rpc.IOS {
		return nil, errors.New("'show interfaces stats' is not implemented for " + ostype)
	}
	var items []string
	deviceNameRegexp := regexp.MustCompile(`^([a-zA-Z0-9\/\.-]+)\s*`)
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		matches := deviceNameRegexp.FindStringSubmatch(line)
		if matches == nil {
			continue
		}
		name := matches[1]
		// Skip interfaces that cannot physically hold an SFP transceiver.
		// This applies to all OS types — NX-OS filters sfpAbsent at the command
		// level but still returns Port-channels, VLANs, and header lines that
		// must be excluded here.
		if !hasSFPCapablePrefix(name) {
			continue
		}
		items = append(items, name)
	}
	return items, nil
}

var transceiverRegexp = map[string]*regexp.Regexp{
	rpc.IOS:   regexp.MustCompile(`\S+\s+(?:(?:-)?\d+\.\d+)\s+(?:(?:-)?\d+\.\d+)\s+((?:-)?\d+\.\d+)\s+((?:-)?\d+\.\d+)\s*`),
	rpc.NXOS:  regexp.MustCompile(`\s*Tx Power\s*((?:-)?\d+\.\d+).*\s*Rx Power\s*((?:-)?\d+\.\d+).*`),
	rpc.IOSXE: regexp.MustCompile(`\s+Transceiver Tx power\s+= ((?:-)?\d+\.\d+).*\s*Transceiver Rx optical power\s+= ((?:-)?\d+\.\d+).*`),
}

var (
	bulkIfaceRegexp = regexp.MustCompile(`^(Ethernet\S+)`)
	bulkTxRegexp    = regexp.MustCompile(`Tx Power\s+((?:-)?\d+\.\d+)`)
	bulkRxRegexp    = regexp.MustCompile(`Rx Power\s+((?:-)?\d+\.\d+)`)
)

// ParseTransceiverBulk parses NX-OS "show interface transceiver" output which
// contains all interfaces in one response. Returns a map of interface -> Optics.
func (c *opticsCollector) ParseTransceiverBulk(output string) (map[string]Optics, error) {
	items := make(map[string]Optics)
	current := ""
	var tx, rx float64
	hasTx, hasRx := false, false

	for _, line := range strings.Split(output, "\n") {
		if m := bulkIfaceRegexp.FindStringSubmatch(line); m != nil {
			// Save previous interface if we have both values
			if current != "" && hasTx && hasRx {
				items[current] = Optics{TxPower: tx, RxPower: rx}
			}
			current = m[1]
			hasTx, hasRx = false, false
		} else if m := bulkTxRegexp.FindStringSubmatch(line); m != nil {
			tx = util.Str2float64(m[1])
			hasTx = true
		} else if m := bulkRxRegexp.FindStringSubmatch(line); m != nil {
			rx = util.Str2float64(m[1])
			hasRx = true
		}
	}
	// Save last interface
	if current != "" && hasTx && hasRx {
		items[current] = Optics{TxPower: tx, RxPower: rx}
	}
	if len(items) == 0 {
		return nil, errors.New("no transceivers found")
	}
	return items, nil
}

// ParseTransceiver parses cli output and tries to find tx/rx power for an interface
func (c *opticsCollector) ParseTransceiver(ostype string, output string) (Optics, error) {
	re, ok := transceiverRegexp[ostype]
	if !ok {
		return Optics{}, errors.New("Transceiver data is not implemented for " + ostype)
	}
	matches := re.FindStringSubmatch(output)
	if matches == nil {
		return Optics{}, errors.New("Transceiver not found")
	}
	return Optics{
		TxPower: util.Str2float64(matches[1]),
		RxPower: util.Str2float64(matches[2]),
	}, nil
}
