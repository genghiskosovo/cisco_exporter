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
	"Ethernet", // NX-OS physical ports
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
		// For IOS/IOSXE, skip interfaces that cannot have optical transceivers.
		// NXOS already filters sfpAbsent at the command level.
		if ostype != rpc.NXOS && !hasSFPCapablePrefix(name) {
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
