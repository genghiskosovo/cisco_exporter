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

// STPInstance is the per-instance header "show spanning-tree" prints above the
// port rows: the root bridge ID, this bridge's own ID, and the cost to the
// root. The port rows alone cannot answer "who is root" — a switch whose only
// port in a VLAN is designated looks identical to the real root bridge — so
// the device's own root-vs-self comparison is read here instead of inferred.
type STPInstance struct {
	// Vlan is the VLAN for PVST/rapid-PVST, or the instance id under MST.
	Vlan     string
	Protocol string

	RootPriority    float64
	HasRootPriority bool
	RootAddress     string
	RootCost        float64

	BridgePriority    float64
	HasBridgePriority bool
	BridgeAddress     string

	// ThisIsRoot records the explicit "This bridge is the root" line, which
	// IOS and NX-OS print in place of the root cost when there is nothing
	// upstream.
	ThisIsRoot bool
}

// IsRoot reports whether this device is the root bridge of the instance.
func (i STPInstance) IsRoot() bool {
	if i.ThisIsRoot {
		return true
	}
	if i.RootAddress == "" || i.BridgeAddress == "" {
		return false
	}
	return i.RootAddress == i.BridgeAddress && i.RootPriority == i.BridgePriority
}

var (
	// instanceRe matches a per-instance header: "VLAN0010" under
	// PVST/rapid-PVST, "MST0" under MST. The captured number becomes the
	// `vlan` label either way, which is how junos_exporter labels an MSTP
	// instance too.
	instanceRe  = regexp.MustCompile(`^(?:VLAN|MST)(\d+)`)
	rowRe       = regexp.MustCompile(`^(\S+)\s+([A-Za-z]+)\s+([A-Za-z]+)\s+(\d+)\s+[\d.]+\s+(.+)$`)
	typeAnnotRe = regexp.MustCompile(`\([^)]+\)\s*`)

	protocolRe   = regexp.MustCompile(`^Spanning tree enabled protocol\s+(\S+)`)
	rootIDRe     = regexp.MustCompile(`^Root ID\s+Priority\s+(\d+)`)
	bridgeIDRe   = regexp.MustCompile(`^Bridge ID\s+Priority\s+(\d+)`)
	addressRe    = regexp.MustCompile(`^Address\s+(\S+)`)
	rootCostRe   = regexp.MustCompile(`^Cost\s+(\d+)`)
	thisIsRootRe = regexp.MustCompile(`^This bridge is the root`)
)

func supported(ostype string) error {
	switch ostype {
	case rpc.IOS, rpc.IOSXE, rpc.NXOS:
		return nil
	}
	return errors.New("'show spanning-tree' is not implemented for " + ostype)
}

func (c *stpCollector) Parse(ostype string, output string) ([]STPInterface, error) {
	if err := supported(ostype); err != nil {
		return nil, err
	}

	var items []STPInterface
	currentVlan := ""

	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "---") || strings.HasPrefix(line, "Interface") {
			continue
		}

		if m := instanceRe.FindStringSubmatch(line); m != nil {
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

// ParseVlans reads the per-instance headers out of the same output Parse walks,
// so root-bridge detection costs no extra command.
func (c *stpCollector) ParseVlans(ostype string, output string) ([]STPInstance, error) {
	if err := supported(ostype); err != nil {
		return nil, err
	}

	var items []STPInstance
	var current *STPInstance
	// Which ID block the indented "Address" / "Cost" lines belong to. They
	// carry no label of their own, only their position under "Root ID" or
	// "Bridge ID".
	section := ""

	flush := func() {
		if current != nil {
			items = append(items, *current)
			current = nil
		}
	}

	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		if m := instanceRe.FindStringSubmatch(line); m != nil {
			flush()
			n, _ := strconv.Atoi(m[1])
			current = &STPInstance{Vlan: strconv.Itoa(n)}
			section = ""
			continue
		}
		if current == nil {
			continue
		}

		switch {
		case protocolRe.MatchString(line):
			current.Protocol = protocolRe.FindStringSubmatch(line)[1]
		case rootIDRe.MatchString(line):
			current.RootPriority = util.Str2float64(rootIDRe.FindStringSubmatch(line)[1])
			current.HasRootPriority = true
			section = "root"
		case bridgeIDRe.MatchString(line):
			current.BridgePriority = util.Str2float64(bridgeIDRe.FindStringSubmatch(line)[1])
			current.HasBridgePriority = true
			section = "bridge"
		case addressRe.MatchString(line):
			addr := strings.ToLower(addressRe.FindStringSubmatch(line)[1])
			switch section {
			case "root":
				current.RootAddress = addr
			case "bridge":
				current.BridgeAddress = addr
			}
		case thisIsRootRe.MatchString(line):
			current.ThisIsRoot = true
		case section == "root" && rootCostRe.MatchString(line):
			current.RootCost = util.Str2float64(rootCostRe.FindStringSubmatch(line)[1])
		}
	}
	flush()

	return items, nil
}
