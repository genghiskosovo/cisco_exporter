package cdp

import (
	"github.com/lwlcom/cisco_exporter/collector"
	"github.com/lwlcom/cisco_exporter/rpc"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
)

const prefix = "cisco_cdp_"

var neighborDesc *prometheus.Desc

func init() {
	l := []string{"target", "local_interface", "neighbor", "neighbor_interface"}
	neighborDesc = prometheus.NewDesc(
		prefix+"neighbor",
		"CDP neighbor adjacency (value is always 1; use labels for topology)",
		l, nil,
	)
}

type cdpCollector struct{}

// NewCollector creates a new CDP collector.
func NewCollector() collector.RPCCollector {
	return &cdpCollector{}
}

func (*cdpCollector) Name() string { return "CDP" }

func (*cdpCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- neighborDesc
}

func (c *cdpCollector) Collect(client *rpc.Client, ch chan<- prometheus.Metric, labelValues []string) error {
	out, err := client.RunCommand("show cdp neighbors")
	if err != nil {
		return err
	}

	items, err := c.Parse(client.OSType, out)
	if err != nil {
		if client.Debug {
			log.Printf("Parse CDP neighbors for %s: %s\n", labelValues[0], err.Error())
		}
		return nil
	}

	for _, item := range items {
		l := append(labelValues, item.LocalInterface, item.Neighbor, item.NeighborInterface)
		ch <- prometheus.MustNewConstMetric(neighborDesc, prometheus.GaugeValue, 1, l...)
	}

	return nil
}
