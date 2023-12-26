package bgp

import (
	"errors"
	"regexp"
	"strings"

	"github.com/lwlcom/cisco_exporter/rpc"
	"github.com/lwlcom/cisco_exporter/util"
)

// Parse parses cli output and tries to find bgp sessions with related data. This Parse2 func added for IOSXR
func (c *bgpCollector) Parse2(ostype string, output string) ([]BgpSession2, error) {
	if ostype != rpc.IOSXR {
		return nil, errors.New("'show bgp neighbor' is not implemented for " + ostype)
	}
	items := []BgpSession2{}

//------------------------------------------------------
	// Define the regex pattern
	pattern := `(?ms)BGP neighbor is (?P<neighbor_ip>\S+).*?` +
	`Remote AS (?P<remote_as>\d+).*?` +
	`Description: (?P<description>.*?)\n.*?` +
	`BGP state = (?P<bgp_state>\w+).*?` +
	`(?P<accepted_prefixes>\d+) accepted prefixes, (?P<best_paths>\d+) are bestpaths.*?` +
	`Prefix advertised (?P<prefix_advertised>\d+),`

	// Compile the regex
	r, _ := regexp.Compile(pattern)

	// Find all matches
	matches := r.FindAllStringSubmatch(output, -1)
	names := r.SubexpNames()

	// Iterate over matches
	for _, match := range matches {
		result := make(map[string]string)
		for i, name := range names {
			if i != 0 && name != "" {
				result[name] = strings.TrimSpace(match[i])
			}
		}
		up := true
		if strings.TrimSpace(result["bgp_state"]) != "Established" {
			up = false
		}
		item := BgpSession2{
			Ip:               strings.TrimSpace(result["neighbor_ip"]),
			Asn:              strings.TrimSpace(result["remote_as"]),
			AcceptedPrefixes: util.Str2float64(strings.TrimSpace(result["accepted_prefixes"])),
			BestPath:         util.Str2float64(strings.TrimSpace(result["best_paths"])),
			Up:               up,
			PrefixAdvertised: util.Str2float64(strings.TrimSpace(result["prefix_advertised"])),
			Description:      strings.TrimSpace(result["description"]),
		}
		items = append(items, item)
	}
	return items, nil
}
