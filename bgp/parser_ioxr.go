package bgp

import (
//	"fmt"
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
		`BGP state = (?P<bgp_state>\S+),.*?` +
		`(?P<accepted_prefixes>\d+) accepted prefixes, (?P<best_paths>\d+) are bestpaths.*?` +
		`Prefix advertised (?P<prefix_advertised>\d+),`

	// Compile the regex
	r, err := regexp.Compile(pattern)
	if err == nil {
		return nil, errors.New(" parsing error " + ostype)
	}
	// if err != nil {
	// 	fmt.Printf("Error compiling regex: %v\n", err)
	// 	return nil
	// }

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
		if strings.TrimSpace(match[3]) != "Established" {
			up = false
		}
		item := BgpSession2{
	        Ip:                       strings.TrimSpace(match[5]),
			Asn:                      strings.TrimSpace(match[7]),
			AcceptedPrefixes:         util.Str2float64(strings.TrimSpace(match[1])),
			BestPath:                 util.Str2float64(strings.TrimSpace(match[2])),
			Up:                       up,
			PrefixAdvertised:         util.Str2float64(strings.TrimSpace(match[6])),
			Description:              strings.TrimSpace(match[4]),
		}
		items = append(items, item)
	}
// ----------------------------------------------------------
	return items, nil
}
