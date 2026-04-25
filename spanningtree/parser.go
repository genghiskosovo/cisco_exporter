package spanningtree

import (
	"errors"
	"regexp"
	"strconv"
	"strings"

	"github.com/lwlcom/cisco_exporter/rpc"
	"github.com/lwlcom/cisco_exporter/util"
)

type STPInterface struct {
	Vlan      string
	Interface string
	Role      string
	Status    string
	Cost      float64
	Type      string
}

var (
	vlanRe      = regexp.MustCompile(`^VLAN(\d+)`)
	rowRe       = regexp.MustCompile(`^(\S+)\s+([A-Za-z]+)\s+([A-Za-z]+)\s+(\d+)\s+[\d.]+\s+(.+)$`)
	typeAnnotRe = regexp.MustCompile(`\([^)]+\)\s*`)
)

func (c *stpCollector) Parse(ostype string, output string) ([]STPInterface, error) {
	switch ostype {
	case rpc.IOS, rpc.IOSXE, rpc.NXOS:
	default:
		return nil, errors.New("'show spanning-tree' is not implemented for " + ostype)
	}

	var items []STPInterface
	currentVlan := ""

	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "---") || strings.HasPrefix(line, "Interface") {
			continue
		}

		if m := vlanRe.FindStringSubmatch(line); m != nil {
			n, _ := strconv.Atoi(m[1])
			currentVlan = strconv.Itoa(n)
			continue
		}

		if currentVlan == "" {
			continue
		}

		if m := rowRe.FindStringSubmatch(line); m != nil {
			typ := strings.TrimSpace(typeAnnotRe.ReplaceAllString(m[5], ""))
			items = append(items, STPInterface{
				Vlan:      currentVlan,
				Interface: m[1],
				Role:      m[2],
				Status:    m[3],
				Cost:      util.Str2float64(m[4]),
				Type:      typ,
			})
		}
	}

	return items, nil
}
