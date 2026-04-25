package spanningtree

import (
	"github.com/lwlcom/cisco_exporter/collector"
	"github.com/lwlcom/cisco_exporter/rpc"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
)

const prefix string = "cisco_stp_"

var (
	stpStatusDesc *prometheus.Desc
	stpRoleDesc   *prometheus.Desc
	stpCostDesc   *prometheus.Desc
)

func init() {
	l := []string{"target", "vlan", "interface", "type"}

	stpStatusDesc = prometheus.NewDesc(prefix+"interface_status",
		"STP port state (1=FWD, 2=BLK, 3=LRN, 4=LST, 5=DIS, 0=other)", l, nil)
	stpRoleDesc = prometheus.NewDesc(prefix+"interface_role",
		"STP port role (1=Root, 2=Desg, 3=Altn, 4=Bak, 0=other)", l, nil)
	stpCostDesc = prometheus.NewDesc(prefix+"interface_cost",
		"STP interface path cost", l, nil)
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

	for _, item := range items {
		l := append(labelValues, item.Vlan, item.Interface, item.Type)
		ch <- prometheus.MustNewConstMetric(stpStatusDesc, prometheus.GaugeValue, mapStatus(item.Status), l...)
		ch <- prometheus.MustNewConstMetric(stpRoleDesc, prometheus.GaugeValue, mapRole(item.Role), l...)
		ch <- prometheus.MustNewConstMetric(stpCostDesc, prometheus.GaugeValue, item.Cost, l...)
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
