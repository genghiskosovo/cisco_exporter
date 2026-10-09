package lag

import (
	"errors"
	"regexp"
	"strings"

	"github.com/lwlcom/cisco_exporter/rpc"
)

var (
	// groupRe matches a table row: a channel-group number, then the
	// port-channel with its state flags in parentheses, then the rest of the
	// row. NX-OS has a Type column that IOS lacks, so everything after the
	// port-channel is handed to memberRe rather than being matched by column.
	//
	//   1     Po1(SU)     Eth      LACP      Eth1/1(P)    Eth1/2(P)   (NX-OS)
	//   1     Po1(SU)         LACP      Gi0/1(P)    Gi0/2(P)          (IOS)
	groupRe = regexp.MustCompile(`^(\d+)\s+(\S+?)\(([^)]*)\)\s*(.*)$`)

	// groupNoFlagsRe is the same row without the state flags, which some
	// releases omit. The port-channel is matched by name here, because a bare
	// "<number> <word>" is too loose to tell a table row from prose.
	groupNoFlagsRe = regexp.MustCompile(`^(\d+)\s+((?i:Po|port-channel)\d+)\s+(.*)$`)

	// memberRe picks the member ports out of a row. A member always carries
	// its state flag immediately after the name -- "Eth1/1(P)", "Gi0/2(D)" --
	// which is what separates it from the Type and Protocol columns.
	memberRe = regexp.MustCompile(`(\S+?)\(([A-Za-z]*)\)`)
)

// Parse reads "show port-channel summary" (NX-OS) or "show etherchannel
// summary" (IOS / IOS XE).
//
// Every member the device lists is returned, whatever its state flag: this is
// the configured membership of the bundle, and whether a member is up is
// already answered by cisco_interface_up for that interface.
func (c *lagCollector) Parse(ostype string, output string) ([]LagMember, error) {
	switch ostype {
	case rpc.IOS, rpc.IOSXE, rpc.NXOS:
	default:
		return nil, errors.New("port-channel summary is not implemented for " + ostype)
	}

	var members []LagMember
	current := ""

	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "---") {
			continue
		}

		rest := ""
		switch {
		case groupRe.MatchString(line):
			m := groupRe.FindStringSubmatch(line)
			current, rest = m[2], m[4]
		case groupNoFlagsRe.MatchString(line):
			m := groupNoFlagsRe.FindStringSubmatch(line)
			current, rest = m[2], m[3]
		default:
			// A continuation line: a wide bundle wraps its member list onto
			// the next line with no group number. Before the first row there
			// is no bundle to attach members to, which is what keeps the flag
			// legend from being read as data.
			if current == "" {
				continue
			}
			rest = line
		}

		for _, m := range memberRe.FindAllStringSubmatch(rest, -1) {
			name := strings.TrimSpace(m[1])
			if name == "" {
				continue
			}
			members = append(members, LagMember{Aggregate: current, Member: name})
		}
	}

	return members, nil
}

// command returns the right summary command for the OS.
func command(ostype string) string {
	if ostype == rpc.NXOS {
		return "show port-channel summary"
	}
	return "show etherchannel summary"
}
