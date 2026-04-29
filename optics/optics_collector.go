package optics

import (
	"regexp"

	"github.com/lwlcom/cisco_exporter/collector"
	"github.com/lwlcom/cisco_exporter/rpc"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
)

const prefix string = "cisco_optics_"

var (
	opticsTXDesc          *prometheus.Desc
	opticsRXDesc          *prometheus.Desc
	opticsTXThresholdDesc *prometheus.Desc
	opticsRXThresholdDesc *prometheus.Desc
)

func init() {
	l := []string{"target", "interface"}
	lt := []string{"target", "interface", "level"}
	opticsTXDesc = prometheus.NewDesc(prefix+"tx", "Transceiver Tx power (dBm)", l, nil)
	opticsRXDesc = prometheus.NewDesc(prefix+"rx", "Transceiver Rx power (dBm)", l, nil)
	opticsTXThresholdDesc = prometheus.NewDesc(prefix+"tx_threshold", "Transceiver Tx power threshold (dBm)", lt, nil)
	opticsRXThresholdDesc = prometheus.NewDesc(prefix+"rx_threshold", "Transceiver Rx power threshold (dBm)", lt, nil)
}

type opticsCollector struct {
}

// NewCollector creates a new collector
func NewCollector() collector.RPCCollector {
	return &opticsCollector{}
}

// Name returns the name of the collector
func (*opticsCollector) Name() string {
	return "Optics"
}

// Describe describes the metrics
func (*opticsCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- opticsTXDesc
	ch <- opticsRXDesc
	ch <- opticsTXThresholdDesc
	ch <- opticsRXThresholdDesc
}

// emitOptics sends TX/RX current power and threshold metrics for one interface.
func emitOptics(ch chan<- prometheus.Metric, labelValues []string, iface string, o Optics) {
	l := append(labelValues, iface)
	ch <- prometheus.MustNewConstMetric(opticsTXDesc, prometheus.GaugeValue, o.TxPower, l...)
	ch <- prometheus.MustNewConstMetric(opticsRXDesc, prometheus.GaugeValue, o.RxPower, l...)
	if !o.HasThresholds {
		return
	}
	for _, entry := range []struct {
		level  string
		tx, rx float64
	}{
		{"high_alarm", o.TxHighAlarm, o.RxHighAlarm},
		{"high_warn", o.TxHighWarn, o.RxHighWarn},
		{"low_warn", o.TxLowWarn, o.RxLowWarn},
		{"low_alarm", o.TxLowAlarm, o.RxLowAlarm},
	} {
		// Each lt must be an independent allocation to avoid backing-array aliasing.
		lt := append(append([]string{}, l...), entry.level)
		ch <- prometheus.MustNewConstMetric(opticsTXThresholdDesc, prometheus.GaugeValue, entry.tx, lt...)
		ch <- prometheus.MustNewConstMetric(opticsRXThresholdDesc, prometheus.GaugeValue, entry.rx, lt...)
	}
}

// Collect collects metrics from Cisco
func (c *opticsCollector) Collect(client *rpc.Client, ch chan<- prometheus.Metric, labelValues []string) error {
	switch client.OSType {
	case rpc.NXOS:
		return c.collectNXOS(client, ch, labelValues)
	case rpc.IOS:
		return c.collectIOS(client, ch, labelValues)
	case rpc.IOSXR:
		return c.collectIOSXR(client, ch, labelValues)
	}

	// IOS-XE: per-port via hw-module subslot command
	if client.OSType != rpc.IOSXE {
		return nil
	}
	out, err := client.RunCommand("show interfaces stats | exclude disabled")
	if err != nil {
		return err
	}
	interfaces, err := c.ParseInterfaces(client.OSType, out)
	if err != nil {
		if client.Debug {
			log.Printf("ParseInterfaces for %s: %s\n", labelValues[0], err.Error())
		}
		return nil
	}

	xeDev := regexp.MustCompile(`\S(\d+)/(\d+)/(\d+)`)

	for _, i := range interfaces {
		matches := xeDev.FindStringSubmatch(i)
		if matches == nil {
			continue
		}
		out, err = client.RunCommand("show hw-module subslot " + matches[1] + "/" + matches[2] + " transceiver " + matches[3] + " status")
		if err != nil {
			if client.Debug {
				log.Printf("Transceiver command on %s: %s\n", labelValues[0], err.Error())
			}
			continue
		}
		optic, err := c.ParseTransceiver(client.OSType, out)
		if err != nil {
			if client.Debug {
				log.Printf("Transceiver data for %s: %s\n", labelValues[0], err.Error())
			}
			continue
		}
		emitOptics(ch, labelValues, i, optic)
	}

	return nil
}

// collectNXOS fetches all transceiver data in a single command with thresholds
func (c *opticsCollector) collectNXOS(client *rpc.Client, ch chan<- prometheus.Metric, labelValues []string) error {
	out, err := client.RunCommand("show interface transceiver details")
	if err != nil {
		return err
	}
	items, err := c.ParseTransceiverBulk(out)
	if err != nil {
		if client.Debug {
			log.Printf("ParseTransceiverBulk for %s: %s\n", labelValues[0], err.Error())
		}
		return nil
	}
	for iface, optic := range items {
		emitOptics(ch, labelValues, iface, optic)
	}
	return nil
}

// collectIOS fetches all transceiver data for IOS in a single bulk command with thresholds
func (c *opticsCollector) collectIOS(client *rpc.Client, ch chan<- prometheus.Metric, labelValues []string) error {
	out, err := client.RunCommand("show interfaces transceiver detail")
	if err != nil {
		return err
	}
	items, err := c.ParseTransceiverDetailIOS(out)
	if err != nil {
		if client.Debug {
			log.Printf("ParseTransceiverDetailIOS for %s: %s\n", labelValues[0], err.Error())
		}
		return nil
	}
	for iface, optic := range items {
		emitOptics(ch, labelValues, iface, optic)
	}
	return nil
}

// collectIOSXR fetches optics per physical interface via "show controllers optics <loc>".
// NOTE: each "show controllers optics" round-trip takes ~0.5s on ASR9001, so adding this
// collector increases the ASR9001 scrape duration from ~4s to ~8s depending on port count.
// Disable with optics.enabled=false per-device if scrape timeout becomes an issue.
func (c *opticsCollector) collectIOSXR(client *rpc.Client, ch chan<- prometheus.Metric, labelValues []string) error {
	// Use "show interfaces | include is" to get just the first line of each interface
	// block (e.g. "TenGigE0/0/0/0 is up, line protocol is up") without the verbose body.
	out, err := client.RunCommand(`show interfaces | include " is "`)
	if err != nil {
		return err
	}
	interfaces, err := c.ParseInterfaces(rpc.IOSXR, out)
	if err != nil {
		if client.Debug {
			log.Printf("ParseInterfaces (IOS-XR) for %s: %s\n", labelValues[0], err.Error())
		}
		return nil
	}

	// Extract rack/slot/instance/port location from full interface name, e.g.
	// TenGigE0/0/0/1 → 0/0/0/1. Sub-interfaces (TenGigE0/0/0/1.100) are
	// excluded because the trailing dot means the regex won't match at $.
	locRegexp := regexp.MustCompile(`(\d+/\d+/\d+/\d+)$`)

	for _, iface := range interfaces {
		m := locRegexp.FindStringSubmatch(iface)
		if m == nil {
			continue
		}
		loc := m[1]
		out, err = client.RunCommand("show controllers optics " + loc)
		if err != nil {
			if client.Debug {
				log.Printf("show controllers optics %s on %s: %s\n", loc, labelValues[0], err.Error())
			}
			continue
		}
		optic, err := c.ParseControllerOpticsIOSXR(out)
		if err != nil {
			if client.Debug {
				log.Printf("ParseControllerOpticsIOSXR %s on %s: %s\n", loc, labelValues[0], err.Error())
			}
			continue
		}
		emitOptics(ch, labelValues, iface, optic)
	}
	return nil
}
