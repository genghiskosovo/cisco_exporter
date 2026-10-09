package lag

import (
	"github.com/lwlcom/cisco_exporter/collector"
	"github.com/lwlcom/cisco_exporter/rpc"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
)

// The metric belongs to the interface family, and mirrors
// junos_interface_lag_member label for label.
const prefix = "cisco_interface_"

var lagMemberDesc *prometheus.Desc

func init() {
	lagMemberDesc = prometheus.NewDesc(
		prefix+"lag_member",
		"Physical interface bundled into an aggregate (always 1; read the labels)",
		[]string{"target", "aggregate", "member"}, nil,
	)
}

type lagCollector struct{}

// NewCollector creates a new LAG membership collector.
func NewCollector() collector.RPCCollector {
	return &lagCollector{}
}

func (*lagCollector) Name() string { return "LAG" }

func (*lagCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- lagMemberDesc
}

func (c *lagCollector) Collect(client *rpc.Client, ch chan<- prometheus.Metric, labelValues []string) error {
	switch client.OSType {
	case rpc.IOS, rpc.IOSXE, rpc.NXOS:
	default:
		return nil
	}

	out, err := client.RunCommand(command(client.OSType))
	if err != nil {
		return err
	}

	members, err := c.Parse(client.OSType, out)
	if err != nil {
		if client.Debug {
			log.Printf("Parse LAG members for %s: %s\n", labelValues[0], err.Error())
		}
		return nil
	}

	for _, m := range members {
		l := append(append([]string{}, labelValues...), m.Aggregate, m.Member)
		ch <- prometheus.MustNewConstMetric(lagMemberDesc, prometheus.GaugeValue, 1, l...)
	}

	return nil
}
