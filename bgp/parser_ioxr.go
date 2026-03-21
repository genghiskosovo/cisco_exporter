package bgp

import (
	"errors"
	"regexp"
	"strings"

	"github.com/lwlcom/cisco_exporter/rpc"
	"github.com/lwlcom/cisco_exporter/util"
)

// Parse2 parses IOS XR 'show bgp neighbor' output into BgpSession2 structs
func (c *bgpCollector) Parse2(ostype string, output string) ([]BgpSession2, error) {
	if ostype != rpc.IOSXR {
		return nil, errors.New("'show bgp neighbor' is not implemented for " + ostype)
	}

	pattern := `(?ms)BGP neighbor is (?P<neighbor_ip>\S+).*?` +
		`Remote AS (?P<remote_as>\d+).*?` +
		`Description: (?P<description>.*?)\n.*?` +
		`BGP state = (?P<bgp_state>\w+).*?` +
		`(?P<accepted_prefixes>\d+) accepted prefixes, (?P<best_paths>\d+) are bestpaths.*?` +
		`Prefix advertised (?P<prefix_advertised>\d+), suppressed \d+, withdrawn (?P<prefix_withdrawn>\d+)`

	r, err := regexp.Compile(pattern)
	if err != nil {
		return nil, err
	}

	matches := r.FindAllStringSubmatch(output, -1)
	if len(matches) == 0 {
		return nil, errors.New("no BGP neighbors found in output")
	}

	names := r.SubexpNames()
	items := []BgpSession2{}

	for _, match := range matches {
		result := make(map[string]string)
		for i, name := range names {
			if i != 0 && name != "" {
				result[name] = strings.TrimSpace(match[i])
			}
		}

		advertised := util.Str2float64(result["prefix_advertised"]) - util.Str2float64(result["prefix_withdrawn"])
		items = append(items, BgpSession2{
			Ip:               result["neighbor_ip"],
			Asn:              result["remote_as"],
			Up:               result["bgp_state"] == "Established",
			AcceptedPrefixes: util.Str2float64(result["accepted_prefixes"]),
			BestPath:         util.Str2float64(result["best_paths"]),
			PrefixAdvertised: advertised,
			Description:      result["description"],
		})
	}

	return items, nil
}
