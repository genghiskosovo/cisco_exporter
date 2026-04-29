package bgp

import (
	"errors"
	"regexp"
	"strings"

	"github.com/lwlcom/cisco_exporter/rpc"
	"github.com/lwlcom/cisco_exporter/util"
)

// iosxrSummaryLineRegexp matches a neighbor row in "show bgp summary" output.
// Captures: neighbor IP (1), Up/Down duration (2).
// Column order: Neighbor Spk AS MsgRcvd MsgSent TblVer InQ OutQ Up/Down St/PfxRcd
var iosxrSummaryLineRegexp = regexp.MustCompile(
	`^\s*(\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3})\s+\d+\s+\d+\s+\d+\s+\d+\s+\d+\s+\d+\s+\d+\s+(\S+)`,
)

var iosxrNeighborRegexp = regexp.MustCompile(
	`(?ms)BGP neighbor is (?P<neighbor_ip>\S+).*?` +
		`Remote AS (?P<remote_as>\d+).*?` +
		`Description: (?P<description>.*?)\n.*?` +
		`BGP state = (?P<bgp_state>\w+)(?:, up for (?P<uptime>\S+))?.*?` +
		`(?P<accepted_prefixes>\d+) accepted prefixes, (?P<best_paths>\d+) are bestpaths.*?` +
		`Prefix advertised (?P<prefix_advertised>\d+), suppressed \d+, withdrawn (?P<prefix_withdrawn>\d+)`,
)

// ParseSummaryIOSXR parses "show bgp summary" for IOS-XR.
// Returns a map of neighbor IP → duration in current state (seconds).
// This is used to fill in downtime for non-Established sessions, since
// "show bgp neighbor" does not include a "down for X" field.
func (c *bgpCollector) ParseSummaryIOSXR(output string) map[string]float64 {
	result := make(map[string]float64)
	for _, line := range strings.Split(output, "\n") {
		m := iosxrSummaryLineRegexp.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		result[m[1]] = parseUptime(m[2])
	}
	return result
}

// Parse2 parses cli output and tries to find bgp sessions with related data for IOSXR
func (c *bgpCollector) Parse2(ostype string, output string) ([]BgpSession2, error) {
	if ostype != rpc.IOSXR {
		return nil, errors.New("'show bgp neighbor' is not implemented for " + ostype)
	}
	items := []BgpSession2{}

	matches := iosxrNeighborRegexp.FindAllStringSubmatch(output, -1)
	names := iosxrNeighborRegexp.SubexpNames()

	for _, match := range matches {
		result := make(map[string]string)
		for i, name := range names {
			if i != 0 && name != "" {
				result[name] = strings.TrimSpace(match[i])
			}
		}
		up := strings.TrimSpace(result["bgp_state"]) == "Established"
		advertisedExact := util.Str2float64(result["prefix_advertised"]) - util.Str2float64(result["prefix_withdrawn"])
		item := BgpSession2{
			Ip:               result["neighbor_ip"],
			Asn:              result["remote_as"],
			AcceptedPrefixes: util.Str2float64(result["accepted_prefixes"]),
			BestPath:         util.Str2float64(result["best_paths"]),
			Up:               up,
			PrefixAdvertised: advertisedExact,
			Description:      result["description"],
			UptimeSeconds:    parseUptime(result["uptime"]),
		}
		items = append(items, item)
	}
	return items, nil
}
