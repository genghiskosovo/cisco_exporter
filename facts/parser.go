package facts

import (
	"errors"
	"regexp"
	"strings"

	"github.com/lwlcom/cisco_exporter/rpc"
	"github.com/lwlcom/cisco_exporter/util"
)

// ParseVersion parses cli output and tries to find the version number of the running OS
func (c *factsCollector) ParseVersion(ostype string, output string) (VersionFact, error) {
	if ostype != rpc.IOSXE && ostype != rpc.NXOS && ostype != rpc.IOS && ostype != rpc.IOSXR {
		return VersionFact{}, errors.New("'show version' is not implemented for " + ostype)
	}
	versionRegexp := map[string]*regexp.Regexp{
		rpc.IOSXE: regexp.MustCompile(`^.*, Version (.+) -.*$`),
		rpc.IOS:   regexp.MustCompile(`^.*, Version (.+),.*$`),
		rpc.NXOS:  regexp.MustCompile(`^\s+NXOS: version (.*)$`),
		rpc.IOSXR: regexp.MustCompile(`^.*IOS XR Software, Version(.*)\[.*$`),
	}

	for _, line := range strings.Split(output, "\n") {
		matches := versionRegexp[ostype].FindStringSubmatch(line)
		if matches == nil {
			continue
		}
		return VersionFact{Version: ostype + "-" + strings.TrimSpace(matches[1])}, nil
	}
	return VersionFact{}, errors.New("version string not found")
}

// ParseMemory parses cli output and tries to find current memory usage
func (c *factsCollector) ParseMemory(ostype string, output string) ([]MemoryFact, error) {
	if ostype != rpc.IOSXE && ostype != rpc.IOS && ostype != rpc.IOSXR {
		return nil, errors.New("'show process memory' is not implemented for " + ostype)
	}

	items := []MemoryFact{}
	lines := strings.Split(output, "\n")

	if ostype == rpc.IOSXR {
		// IOS XR: 'show memory summary' → "Physical Memory: 12288M total (8986M available)"
		re := regexp.MustCompile(`^Physical Memory:\s*(\d+)M total\s*\((\d+)M available\)$`)
		for _, line := range lines {
			matches := re.FindStringSubmatch(line)
			if matches == nil {
				continue
			}
			totalBytes := util.Str2float64(matches[1]) * 1_000_000
			freeBytes := util.Str2float64(matches[2]) * 1_000_000
			items = append(items, MemoryFact{
				Type:  "Physical Memory",
				Total: totalBytes,
				Free:  freeBytes,
				Used:  totalBytes - freeBytes,
			})
		}
	} else {
		re := regexp.MustCompile(`^\s*(\S*) Pool Total:\s*(\d+) Used:\s*(\d+) Free:\s*(\d+)\s*$`)
		for _, line := range lines {
			matches := re.FindStringSubmatch(line)
			if matches == nil {
				continue
			}
			items = append(items, MemoryFact{
				Type:  matches[1],
				Total: util.Str2float64(matches[2]),
				Used:  util.Str2float64(matches[3]),
				Free:  util.Str2float64(matches[4]),
			})
		}
	}

	return items, nil
}

// ParseCPU parses cli output and tries to find current CPU utilization
func (c *factsCollector) ParseCPU(ostype string, output string) (CPUFact, error) {
	if ostype != rpc.IOSXE && ostype != rpc.IOS && ostype != rpc.IOSXR {
		return CPUFact{}, errors.New("'show process cpu' is not implemented for " + ostype)
	}

	lines := strings.Split(output, "\n")

	if ostype == rpc.IOSXR {
		// IOS XR: "CPU utilization for one minute: 3%; five minutes: 3%; fifteen minutes: 3%"
		re := regexp.MustCompile(`^\s*CPU utilization for one minute: (\d+)%;\s*five minutes: (\d+)%; fifteen minutes: \d+%.*$`)
		for _, line := range lines {
			matches := re.FindStringSubmatch(line)
			if matches == nil {
				continue
			}
			return CPUFact{
				OneMinute:   util.Str2float64(matches[1]),
				FiveMinutes: util.Str2float64(matches[2]),
			}, nil
		}
	} else {
		// IOS / IOS XE: "CPU utilization for five seconds: 1%/0%; one minute: 1%; five minutes: 1%"
		re := regexp.MustCompile(`^\s*CPU utilization for five seconds: (\d+)%\/(\d+)%; one minute: (\d+)%; five minutes: (\d+)%.*$`)
		for _, line := range lines {
			matches := re.FindStringSubmatch(line)
			if matches == nil {
				continue
			}
			return CPUFact{
				FiveSeconds: util.Str2float64(matches[1]),
				Interrupts:  util.Str2float64(matches[2]),
				OneMinute:   util.Str2float64(matches[3]),
				FiveMinutes: util.Str2float64(matches[4]),
			}, nil
		}
	}

	return CPUFact{}, errors.New("CPU utilization string not found")
}
