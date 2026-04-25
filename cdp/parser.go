package cdp

import (
	"errors"
	"regexp"
	"strings"

	"github.com/lwlcom/cisco_exporter/rpc"
)

var (
	// deviceIDLineRe matches a device-ID line: starts at col 0, not a header.
	// Captures everything before the optional serial-number in parentheses.
	deviceIDLineRe = regexp.MustCompile(`^([^(\s]+)`)

	// headerRe matches lines we should skip (capability legend, column header,
	// separator dashes, or the echo of the command itself).
	headerRe = regexp.MustCompile(`(?i)Capability Codes|Device.?ID|Local Intrfce|Hldtme|^-+$|show cdp`)

	// localIfRe extracts the local interface from the start of a trimmed data line.
	// Handles "Gig 0/24" (IOS) and "Eth1/1" (NX-OS) forms.
	localIfRe = regexp.MustCompile(`^(\S+(?:\s+\d+(?:/\d+)+)?)`)

	// portIDRe extracts the remote port ID anchored to the end of the data line.
	// Handles "Eth 1/15" (IOS) and "Eth1/1" (NX-OS) forms.
	portIDRe = regexp.MustCompile(`(\S+(?:\s+\d+(?:/\d+)+)?)\s*$`)

	// holdtimeRe locates the holdtime integer so we can sanity-check the line.
	holdtimeRe = regexp.MustCompile(`\s{2,}(\d+)\s`)
)

// Parse parses the output of "show cdp neighbors" for all supported OS types.
func (c *cdpCollector) Parse(ostype string, output string) ([]CDPNeighbor, error) {
	switch ostype {
	case rpc.IOS, rpc.IOSXE, rpc.NXOS, rpc.IOSXR:
	default:
		return nil, errors.New("'show cdp neighbors' is not implemented for " + ostype)
	}

	var neighbors []CDPNeighbor
	var currentDeviceID string

	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimRight(line, "\r ")
		if line == "" {
			continue
		}

		// Skip header / legend / command-echo lines.
		if headerRe.MatchString(line) {
			currentDeviceID = ""
			continue
		}

		// Non-indented line → device ID.
		if line[0] != ' ' && line[0] != '\t' {
			if m := deviceIDLineRe.FindStringSubmatch(line); m != nil {
				currentDeviceID = m[1]
			}
			continue
		}

		// Indented line → data line for the most recent device ID.
		if currentDeviceID == "" {
			continue
		}

		trimmed := strings.TrimLeft(line, " \t")

		// Require a holdtime to be present; otherwise this is a continuation
		// of a long device-ID line or some other non-data line.
		if !holdtimeRe.MatchString(line) {
			continue
		}

		localIfMatch := localIfRe.FindStringSubmatch(trimmed)
		portIDMatch := portIDRe.FindStringSubmatch(trimmed)
		if localIfMatch == nil || portIDMatch == nil {
			continue
		}

		localIf := strings.ReplaceAll(strings.TrimSpace(localIfMatch[1]), " ", "")
		portID := strings.ReplaceAll(strings.TrimSpace(portIDMatch[1]), " ", "")

		// Sanity check: port ID must differ from the local interface and
		// contain "/" which all Cisco physical/logical port names do.
		if localIf == "" || portID == "" || !strings.Contains(portID, "/") {
			continue
		}

		neighbors = append(neighbors, CDPNeighbor{
			LocalInterface:    localIf,
			Neighbor:          currentDeviceID,
			NeighborInterface: portID,
		})
		currentDeviceID = "" // one data line per device-ID line
	}

	return neighbors, nil
}
