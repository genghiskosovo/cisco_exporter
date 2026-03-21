package bgp

import (
	"errors"
	"regexp"
	"strings"

	"github.com/lwlcom/cisco_exporter/rpc"
	"github.com/lwlcom/cisco_exporter/util"
)

var iosxrNeighborRegexp = regexp.MustCompile(
	`(?ms)BGP neighbor is (?P<neighbor_ip>\S+).*?` +
		`Remote AS (?P<remote_as>\d+).*?` +
		`Description: (?P<description>.*?)\n.*?` +
		`BGP state = (?P<bgp_state>\w+).*?` +
		`(?P<accepted_prefixes>\d+) accepted prefixes, (?P<best_paths>\d+) are bestpaths.*?` +
		`Prefix advertised (?P<prefix_advertised>\d+), suppressed \d+, withdrawn (?P<prefix_withdrawn>\d+)`,
)

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
		}
		items = append(items, item)
	}
	return items, nil
}
