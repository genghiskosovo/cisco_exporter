package spanningtree

import (
	"github.com/lwlcom/cisco_exporter/collector"
	"github.com/lwlcom/cisco_exporter/rpc"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
)

const prefix string = "cisco_stp_"

var (
	stpStatusDesc     *prometheus.Desc
	stpRoleDesc       *prometheus.Desc
	stpCostDesc       *prometheus.Desc
	stpRootBridgeDesc *prometheus.Desc
	stpBridgePrioDesc *prometheus.Desc
)

func init() {
	l := []string{"target", "vlan", "interface", "type"}

	stpStatusDesc = prometheus.NewDesc(prefix+"interface_status",
		"STP port state (1=FWD, 2=BLK, 3=LRN, 4=LST, 5=DIS, 0=other)", l, nil)
	stpRoleDesc = prometheus.NewDesc(prefix+"interface_role",
		"STP port role (1=Root, 2=Desg, 3=Altn, 4=Bak, 0=other)", l, nil)
	stpCostDesc = prometheus.NewDesc(prefix+"interface_cost",
		"STP interface path cost", l, nil)

	// Per instance rather than per port, matching junos_stp_root_bridge and
	// junos_stp_bridge_priority label for label.
	il := []string{"target", "vlan"}
	stpRootBridgeDesc = prometheus.NewDesc(prefix+"root_bridge",
		"1 when this device is the root bridge for the VLAN, 0 otherwise", il, nil)
	stpBridgePrioDesc = prometheus.NewDesc(prefix+"bridge_priority",
		"This bridge's priority in the instance", il, nil)
}

type stpCollector struct{}

func NewCollector() collector.RPCCollector {
	return &stpCollector{}
}

func (*stpCollector) Name() string {
	return "SpanningTree"
}

func (*stpCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- stpStatusDesc
	ch <- stpRoleDesc
	ch <- stpCostDesc
	ch <- stpRootBridgeDesc
	ch <- stpBridgePrioDesc
}

func (c *stpCollector) Collect(client *rpc.Client, ch chan<- prometheus.Metric, labelValues []string) error {
	switch client.OSType {
	case rpc.IOS, rpc.IOSXE, rpc.NXOS:
	default:
		return nil
	}

	out, err := client.RunCommand("show spanning-tree")
	if err != nil {
		return err
	}

	items, err := c.Parse(client.OSType, out)
	if err != nil {
		if client.Debug {
			log.Printf("Parse STP for %s: %s\n", labelValues[0], err.Error())
		}
		return nil
	}

	hasRoot := make(map[string]bool)
	hasDesg := make(map[string]bool)

	for _, item := range items {
		l := append(labelValues, item.Vlan, item.Interface, item.Type)
		role := mapRole(item.Role)
		ch <- prometheus.MustNewConstMetric(stpStatusDesc, prometheus.GaugeValue, mapStatus(item.Status), l...)
		ch <- prometheus.MustNewConstMetric(stpRoleDesc, prometheus.GaugeValue, role, l...)
		ch <- prometheus.MustNewConstMetric(stpCostDesc, prometheus.GaugeValue, item.Cost, l...)

		switch role {
		case 1:
			hasRoot[item.Vlan] = true
		case 2:
			hasDesg[item.Vlan] = true
		}
	}

	instances, err := c.ParseVlans(client.OSType, out)
	if err != nil {
		if client.Debug {
			log.Printf("Parse STP instances for %s: %s\n", labelValues[0], err.Error())
		}
		instances = nil
	}

	fromDevice := make(map[string]bool, len(instances))
	for _, inst := range instances {
		// A repeated instance header would mean two metrics with the same
		// label set, which fails the whole scrape rather than just this
		// series. One block per instance is what the devices print, so this
		// only guards against output we have not seen.
		if fromDevice[inst.Vlan] {
			continue
		}
		l := append(append([]string{}, labelValues...), inst.Vlan)
		fromDevice[inst.Vlan] = true

		root := 0.0
		if inst.IsRoot() {
			root = 1
		}
		ch <- prometheus.MustNewConstMetric(stpRootBridgeDesc, prometheus.GaugeValue, root, l...)
		if inst.HasBridgePriority {
			ch <- prometheus.MustNewConstMetric(stpBridgePrioDesc, prometheus.GaugeValue, inst.BridgePriority, l...)
		}
	}

	// Fallback for an instance whose header did not parse: a bridge holding
	// designated ports but no root port has nothing upstream of it, so it is
	// the root. Weaker than the device's own answer -- a switch whose only
	// trunk for that VLAN is down reads as root of its own island -- but it
	// keeps the metric present for every instance that has ports.
	for vlan := range hasDesg {
		if fromDevice[vlan] {
			continue
		}
		root := 0.0
		if !hasRoot[vlan] {
			root = 1
		}
		ch <- prometheus.MustNewConstMetric(stpRootBridgeDesc, prometheus.GaugeValue, root,
			append(append([]string{}, labelValues...), vlan)...)
	}

	return nil
}

func mapStatus(status string) float64 {
	switch status {
	case "FWD":
		return 1
	case "BLK":
		return 2
	case "LRN":
		return 3
	case "LST":
		return 4
	case "DIS":
		return 5
	default:
		return 0
	}
}

func mapRole(role string) float64 {
	switch role {
	case "Root":
		return 1
	case "Desg":
		return 2
	case "Altn":
		return 3
	case "Bak":
		return 4
	default:
		return 0
	}
}
