package bgp

import (
	"errors"
	"regexp"
	"strings"

	"github.com/lwlcom/cisco_exporter/rpc"
	"github.com/lwlcom/cisco_exporter/util"
)

// neighborRegexp captures: IP, AS, MsgRcvd, MsgSent, Up/Down duration, State/PfxRcd
var neighborRegexp = regexp.MustCompile(`(\S+)\s+\d\s+(\d+)\s+(\d+)\s+(\d+)\s+\d+\s+\d+\s+\d+\s+(\S+)\s+(\S+)\s*`)

var uptimeDurationRegexp = regexp.MustCompile(`(\d+)([wdhm])`)

// parseUptime converts Cisco BGP duration strings to seconds.
// Handles: "3d16h", "1w2d", "00:05:30", "never".
func parseUptime(s string) float64 {
	if s == "" || s == "never" {
		return 0
	}
	if strings.Contains(s, ":") {
		parts := strings.Split(s, ":")
		if len(parts) == 3 {
			return util.Str2float64(parts[0])*3600 +
				util.Str2float64(parts[1])*60 +
				util.Str2float64(parts[2])
		}
	}
	var total float64
	for _, m := range uptimeDurationRegexp.FindAllStringSubmatch(s, -1) {
		v := util.Str2float64(m[1])
		switch m[2] {
		case "w":
			total += v * 604800
		case "d":
			total += v * 86400
		case "h":
			total += v * 3600
		case "m":
			total += v * 60
		}
	}
	return total
}

// Parse parses cli output and tries to find bgp sessions with related data
func (c *bgpCollector) Parse(ostype string, output string) ([]BgpSession, error) {
	if ostype != rpc.IOSXE && ostype != rpc.NXOS {
		return nil, errors.New("'show bgp all summary' is not implemented for " + ostype)
	}
	items := []BgpSession{}

	matches := neighborRegexp.FindAllStringSubmatch(output, -1)
	for _, match := range matches {
		// match[6] is State/PfxRcd: a number means Established, a word means down
		pref := util.Str2float64(match[6])
		up := true
		if pref < 0 {
			pref = 0
			up = false
		}

		item := BgpSession{
			IP:               match[1],
			Asn:              match[2],
			InputMessages:    util.Str2float64(match[3]),
			OutputMessages:   util.Str2float64(match[4]),
			Up:               up,
			ReceivedPrefixes: pref,
			// match[5] is the Up/Down column — duration in current state regardless of whether up or down
			UptimeSeconds: parseUptime(match[5]),
		}
		items = append(items, item)
	}
	return items, nil
}
