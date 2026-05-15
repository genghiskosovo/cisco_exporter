package optics

import (
	"errors"
	"regexp"
	"strings"

	"github.com/lwlcom/cisco_exporter/rpc"
	"github.com/lwlcom/cisco_exporter/util"
)

// sfpCapablePrefixes lists interface name prefixes that can physically hold an SFP/transceiver.
// FastEthernet, Vlan, Loopback, Tunnel, Port-channel etc. are excluded.
// IOS-XR uses shorter names (TenGigE, HundredGigE, FortyGigE) vs IOS/IOS-XE long forms.
var sfpCapablePrefixes = []string{
	"GigabitEthernet",
	"TenGigabitEthernet",
	"TenGigE",
	"TwentyFiveGigE",
	"FortyGigabitEthernet",
	"FortyGigE",
	"HundredGigabitEthernet",
	"HundredGigE",
	"Ethernet",
	"Eth", // NX-OS abbreviated physical ports (Eth1/1, Eth1/2, ...)
}

func hasSFPCapablePrefix(name string) bool {
	for _, prefix := range sfpCapablePrefixes {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return false
}

// ParseInterfaces parses cli output and returns list of SFP-capable interface names
func (c *opticsCollector) ParseInterfaces(ostype string, output string) ([]string, error) {
	if ostype != rpc.IOSXE && ostype != rpc.NXOS && ostype != rpc.IOS && ostype != rpc.IOSXR {
		return nil, errors.New("'show interfaces stats' is not implemented for " + ostype)
	}
	var items []string
	deviceNameRegexp := regexp.MustCompile(`^([a-zA-Z0-9\/\.-]+)\s*`)
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		matches := deviceNameRegexp.FindStringSubmatch(line)
		if matches == nil {
			continue
		}
		name := matches[1]
		// Skip interfaces that cannot physically hold an SFP transceiver.
		// This applies to all OS types — NX-OS filters sfpAbsent at the command
		// level but still returns Port-channels, VLANs, and header lines that
		// must be excluded here.
		if !hasSFPCapablePrefix(name) {
			continue
		}
		items = append(items, name)
	}
	return items, nil
}

var transceiverRegexp = map[string]*regexp.Regexp{
	rpc.IOSXE: regexp.MustCompile(`\s+Transceiver Tx power\s+= ((?:-)?\d+\.\d+).*\s*Transceiver Rx optical power\s+= ((?:-)?\d+\.\d+).*`),
}

// naSignalDBm is the sentinel value reported when a transceiver's current
// power reads "N/A" (no signal). -40 dBm sits well below typical low-alarm
// thresholds so dashboards flag it as a fault rather than a healthy reading.
const naSignalDBm = -40.0

var (
	bulkIfaceRegexp  = regexp.MustCompile(`^(Ethernet\S+)`)
	floatRegexp      = regexp.MustCompile(`-?\d+\.\d+`)
	// IOS "show interfaces transceiver detail" section row:
	// <port>  <current>  <high_alarm>  <high_warn>  <low_warn>  <low_alarm>
	iosPortRowRegexp = regexp.MustCompile(`^\s*(\S+)\s+(-?\d+\.\d+)\s+(-?\d+\.\d+)\s+(-?\d+\.\d+)\s+(-?\d+\.\d+)\s+(-?\d+\.\d+)\s*$`)
)

// ParseTransceiverBulk parses NX-OS "show interface transceiver details" output which
// contains all interfaces in one response. Returns a map of interface -> Optics.
// NX-OS Tx/Rx Power line column order: current, high_alarm, low_alarm, high_warn, low_warn
func (c *opticsCollector) ParseTransceiverBulk(output string) (map[string]Optics, error) {
	items := make(map[string]Optics)
	current := ""
	var o Optics
	hasTx, hasRx := false, false

	for _, line := range strings.Split(output, "\n") {
		if m := bulkIfaceRegexp.FindStringSubmatch(line); m != nil {
			if current != "" && hasTx && hasRx {
				items[current] = o
			}
			current = m[1]
			o = Optics{}
			hasTx, hasRx = false, false
		} else if strings.Contains(line, "Tx Power") {
			nums := floatRegexp.FindAllString(line, -1)
			// When the current measurement column is "N/A" (no signal), the
			// floatRegexp skips it and the first number becomes the high-alarm
			// threshold — which would otherwise be reported as the current power.
			naCur := strings.Contains(line, "N/A")
			thrOffset := 1
			if naCur {
				o.TxPower = naSignalDBm
				hasTx = true
				thrOffset = 0
			} else if len(nums) >= 1 {
				o.TxPower = util.Str2float64(nums[0])
				hasTx = true
			}
			if len(nums) >= thrOffset+4 {
				// NX-OS order: current, high_alarm, low_alarm, high_warn, low_warn
				o.TxHighAlarm = util.Str2float64(nums[thrOffset])
				o.TxLowAlarm = util.Str2float64(nums[thrOffset+1])
				o.TxHighWarn = util.Str2float64(nums[thrOffset+2])
				o.TxLowWarn = util.Str2float64(nums[thrOffset+3])
				o.HasThresholds = true
			}
		} else if strings.Contains(line, "Rx Power") {
			nums := floatRegexp.FindAllString(line, -1)
			naCur := strings.Contains(line, "N/A")
			thrOffset := 1
			if naCur {
				o.RxPower = naSignalDBm
				hasRx = true
				thrOffset = 0
			} else if len(nums) >= 1 {
				o.RxPower = util.Str2float64(nums[0])
				hasRx = true
			}
			if len(nums) >= thrOffset+4 {
				// NX-OS order: current, high_alarm, low_alarm, high_warn, low_warn
				o.RxHighAlarm = util.Str2float64(nums[thrOffset])
				o.RxLowAlarm = util.Str2float64(nums[thrOffset+1])
				o.RxHighWarn = util.Str2float64(nums[thrOffset+2])
				o.RxLowWarn = util.Str2float64(nums[thrOffset+3])
				o.HasThresholds = true
			}
		}
	}
	// Save last interface
	if current != "" && hasTx && hasRx {
		items[current] = o
	}
	if len(items) == 0 {
		return nil, errors.New("no transceivers found")
	}
	return items, nil
}

// ParseTransceiverDetailIOS parses IOS "show interfaces transceiver detail" bulk output.
// Returns a map of interface -> Optics with thresholds.
// IOS column order per section row: current, high_alarm, high_warn, low_warn, low_alarm
func (c *opticsCollector) ParseTransceiverDetailIOS(output string) (map[string]Optics, error) {
	items := make(map[string]Optics)
	const (
		sectionNone = iota
		sectionTx
		sectionRx
	)
	section := sectionNone

	for _, line := range strings.Split(output, "\n") {
		switch {
		case strings.Contains(line, "Transmit Power") || strings.Contains(line, "Tx Power"):
			section = sectionTx
			continue
		case strings.Contains(line, "Receive Power") || strings.Contains(line, "Rx Power"):
			section = sectionRx
			continue
		}
		if section == sectionNone {
			continue
		}
		m := iosPortRowRegexp.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		iface := m[1]
		// IOS order: current, high_alarm, high_warn, low_warn, low_alarm
		cur := util.Str2float64(m[2])
		hiAlarm := util.Str2float64(m[3])
		hiWarn := util.Str2float64(m[4])
		loWarn := util.Str2float64(m[5])
		loAlarm := util.Str2float64(m[6])

		o := items[iface]
		switch section {
		case sectionTx:
			o.TxPower = cur
			o.TxHighAlarm = hiAlarm
			o.TxHighWarn = hiWarn
			o.TxLowWarn = loWarn
			o.TxLowAlarm = loAlarm
		case sectionRx:
			o.RxPower = cur
			o.RxHighAlarm = hiAlarm
			o.RxHighWarn = hiWarn
			o.RxLowWarn = loWarn
			o.RxLowAlarm = loAlarm
		}
		o.HasThresholds = true
		items[iface] = o
	}
	if len(items) == 0 {
		return nil, errors.New("no transceivers found")
	}
	return items, nil
}

var (
	xrTxPowerRegexp      = regexp.MustCompile(`Actual TX Power\s*=\s*(-?\d+\.\d+)\s*dBm`)
	xrRxPowerRegexp      = regexp.MustCompile(`RX Power\s*=\s*(-?\d+\.\d+)\s*dBm`)
	// Threshold table rows — column order: High Alarm, Low Alarm, High Warning, Low Warning
	xrRxThresholdRegexp  = regexp.MustCompile(`Rx Power Threshold\(dBm\)\s+(-?\d+\.\d+)\s+(-?\d+\.\d+)\s+(-?\d+\.\d+)\s+(-?\d+\.\d+)`)
	xrTxThresholdRegexp  = regexp.MustCompile(`Tx Power Threshold\(dBm\)\s+(-?\d+\.\d+)\s+(-?\d+\.\d+)\s+(-?\d+\.\d+)\s+(-?\d+\.\d+)`)
)

// ParseControllerOpticsIOSXR parses IOS-XR "show controllers optics <loc>" output.
// Returns Optics with tx/rx power and thresholds when available.
func (c *opticsCollector) ParseControllerOpticsIOSXR(output string) (Optics, error) {
	txM := xrTxPowerRegexp.FindStringSubmatch(output)
	rxM := xrRxPowerRegexp.FindStringSubmatch(output)
	if txM == nil || rxM == nil {
		return Optics{}, errors.New("tx/rx power not found")
	}
	o := Optics{
		TxPower: util.Str2float64(txM[1]),
		RxPower: util.Str2float64(rxM[1]),
	}
	if rxT := xrRxThresholdRegexp.FindStringSubmatch(output); rxT != nil {
		// IOS-XR order: High Alarm, Low Alarm, High Warning, Low Warning
		o.RxHighAlarm = util.Str2float64(rxT[1])
		o.RxLowAlarm = util.Str2float64(rxT[2])
		o.RxHighWarn = util.Str2float64(rxT[3])
		o.RxLowWarn = util.Str2float64(rxT[4])
		o.HasThresholds = true
	}
	if txT := xrTxThresholdRegexp.FindStringSubmatch(output); txT != nil {
		o.TxHighAlarm = util.Str2float64(txT[1])
		o.TxLowAlarm = util.Str2float64(txT[2])
		o.TxHighWarn = util.Str2float64(txT[3])
		o.TxLowWarn = util.Str2float64(txT[4])
		o.HasThresholds = true
	}
	return o, nil
}

// ParseTransceiver parses cli output and tries to find tx/rx power for an interface (IOS-XE only)
func (c *opticsCollector) ParseTransceiver(ostype string, output string) (Optics, error) {
	re, ok := transceiverRegexp[ostype]
	if !ok {
		return Optics{}, errors.New("Transceiver data is not implemented for " + ostype)
	}
	matches := re.FindStringSubmatch(output)
	if matches == nil {
		return Optics{}, errors.New("Transceiver not found")
	}
	return Optics{
		TxPower: util.Str2float64(matches[1]),
		RxPower: util.Str2float64(matches[2]),
	}, nil
}
