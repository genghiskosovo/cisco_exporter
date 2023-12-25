package bgp

import (
	"log"

	"github.com/lwlcom/cisco_exporter/rpc"

	"github.com/lwlcom/cisco_exporter/collector"
	"github.com/prometheus/client_golang/prometheus"
)

const prefix string = "cisco_bgp_session_"

var (
	upDesc                 *prometheus.Desc
	receivedPrefixesDesc   *prometheus.Desc
	advertisedPrefixesDesc *prometheus.Desc
	bestPathsDesc          *prometheus.Desc
	bgpcmd                 string
)

func init() {
	l := []string{"target", "asn", "ip", "description"}
	upDesc = prometheus.NewDesc(prefix+"up", "Session is up (1 = Established)", l, nil)
	receivedPrefixesDesc = prometheus.NewDesc(prefix+"prefixes_received_count", "Number of received prefixes", l, nil)
	advertisedPrefixesDesc = prometheus.NewDesc(prefix+"prefixes_advertised_count", "Number of advertised prefixes", l, nil)
	bestPathsDesc = prometheus.NewDesc(prefix+"best_path_count", "Number of best paths from peer", l, nil)
}

type bgpCollector struct {
}

// NewCollector creates a new collector
func NewCollector() collector.RPCCollector {
	return &bgpCollector{}
}

// Name returns the name of the collector
func (*bgpCollector) Name() string {
	return "BGP"
}

// Describe describes the metrics
func (*bgpCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- upDesc
	ch <- receivedPrefixesDesc
	ch <- advertisedPrefixesDesc
	ch <- bestPathsDesc
}

// Collect collects metrics from Cisco
func (c *bgpCollector) Collect(client *rpc.Client, ch chan<- prometheus.Metric, labelValues []string) error {
	if client.OSType == "IOSXR" {
		bgpcmd = "show bgp neighbor"
		out, err := client.RunCommand(bgpcmd)
		if err != nil {
			return err
		}
		items, err := c.Parse2(client.OSType, out)
		if err != nil {
			if client.Debug {
				log.Printf("Parse bgp sessions for %s: %s\n", labelValues[0], err.Error())
			}
			return nil
		}
	
		for _, item := range items {
			l := append(labelValues, item.Asn, item.Ip, item.Description)
	
			up := 0
			if item.Up {
				up = 1
			}
	
			ch <- prometheus.MustNewConstMetric(upDesc, prometheus.GaugeValue, float64(up), l...)
			ch <- prometheus.MustNewConstMetric(receivedPrefixesDesc, prometheus.GaugeValue, float64(item.AcceptedPrefixes), l...)
			ch <- prometheus.MustNewConstMetric(advertisedPrefixesDesc, prometheus.GaugeValue, float64(item.PrefixAdvertised), l...)
			ch <- prometheus.MustNewConstMetric(bestPathsDesc, prometheus.GaugeValue, float64(item.BestPath), l...)
		}
	
		return nil
	}  else {
		return nil
	}
	// 	bgpcmd = "show bgp all summary"
	// 	items, err := c.Parse(client.OSType, out)
	// 	out, err := client.RunCommand(bgpcmd)
	// 	if err != nil {
	// 		return err
	// 	}
	
	// 	if err != nil {
	// 		if client.Debug {
	// 			log.Printf("Parse bgp sessions for %s: %s\n", labelValues[0], err.Error())
	// 		}
	// 		return nil
	// 	}
	
	// 	for _, item := range items {
	// 		l := append(labelValues, item.Asn, item.Ip)
	
	// 		up := 0
	// 		if item.Up {
	// 			up = 1
	// 		}
	
	// 		ch <- prometheus.MustNewConstMetric(upDesc, prometheus.GaugeValue, float64(up), l...)
	// 		ch <- prometheus.MustNewConstMetric(receivedPrefixesDesc, prometheus.GaugeValue, float64(item.ReceivedPrefixes), l...)
	// 		ch <- prometheus.MustNewConstMetric(inputMessagesDesc, prometheus.GaugeValue, float64(item.InputMessages), l...)
	// 		ch <- prometheus.MustNewConstMetric(outputMessagesDesc, prometheus.GaugeValue, float64(item.OutputMessages), l...)
	// 	}
	
	// 	return nil
	// }
}
