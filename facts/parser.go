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
	versionRegexp := make(map[string]*regexp.Regexp)
	versionRegexp[rpc.IOSXE], _ = regexp.Compile(`^.*, Version (.+) -.*$`)
	versionRegexp[rpc.IOS], _ = regexp.Compile(`^.*, Version (.+),.*$`)
	versionRegexp[rpc.NXOS], _ = regexp.Compile(`^\s+NXOS: version (.*)$`)
    versionRegexp[rpc.IOSXR], _ = regexp.Compile(`^.*IOS XR Software, Version(.*)\[.*$`)
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		matches := versionRegexp[ostype].FindStringSubmatch(line)
		if matches == nil {
			continue
		}
		return VersionFact{Version: ostype + "-" + matches[1]}, nil
	}
	return VersionFact{}, errors.New("Version string not found")
}

// ParseMemory parses cli output and tries to find current memory usage
func (c *factsCollector) ParseMemory(ostype string, output string) ([]MemoryFact, error) {
	if ostype != rpc.IOSXE && ostype != rpc.IOS && ostype != rpc.IOSXR {
		return nil, errors.New("'show process memory' is not implemented for " + ostype)
	}	
	items := []MemoryFact{}
	lines := strings.Split(output, "\n")
	if ostype == rpc.IOSXR {
		memoryRegexp, _ := regexp.Compile(`^Physical Memory:\s*(\d+)M total\s*\((\d+)M available\)$`)

		for _, line := range lines {
			matches := memoryRegexp.FindStringSubmatch(line)
			if matches == nil {
				continue
			}
			item := MemoryFact{
				Type:  "Physical Memory",
				Total: (util.Str2float64(matches[1]))*1000000, // *1000000 to get memory in bytes
				Free:  (util.Str2float64(matches[2]))*1000000,
				Used:  (util.Str2float64(matches[1]) - util.Str2float64(matches[2]))*1000000,

			}
			items = append(items, item)
		}
	} else {
		memoryRegexp, _ := regexp.Compile(`^\s*(\S*) Pool Total:\s*(\d+) Used:\s*(\d+) Free:\s*(\d+)\s*$`)
		for _, line := range lines {
			matches := memoryRegexp.FindStringSubmatch(line)
			if matches == nil {
				continue
			}
			item := MemoryFact{
				Type:  matches[1],
				Total: util.Str2float64(matches[2]),
				Used:  util.Str2float64(matches[3]),
				Free:  util.Str2float64(matches[4]),
			}
			items = append(items, item)
		}
	}

	return items, nil
}

// ParseCPU parses cli output and tries to find current CPU utilization
func (c *factsCollector) ParseCPU(ostype string, output string) (CPUFact, error) {
	if ostype != rpc.IOSXE && ostype != rpc.IOS && ostype != rpc.IOSXR {
		return CPUFact{}, errors.New("'show process cpu' is not implemented for " + ostype)
	}
	if ostype == rpc.IOSXR {
		memoryRegexp, _ := regexp.Compile(`^\s*CPU utilization for one minute: (\d+)%;\s*five minutes: (\d+)%; fifteen minutes: (\d+)%.*$`)
	
		lines := strings.Split(output, "\n")
		for _, line := range lines {
			matches := memoryRegexp.FindStringSubmatch(line)
			if matches == nil {
				continue
			}
			return CPUFact{
				OneMinute:   util.Str2float64(matches[1]),
				FiveMinutes: util.Str2float64(matches[2]),
			}, nil
		}	
	} else {

	    memoryRegexp, _ := regexp.Compile(`^\s*CPU utilization for five seconds: (\d+)%\/(\d+)%; one minute: (\d+)%; five minutes: (\d+)%.*$`)
    
	    lines := strings.Split(output, "\n")
	    for _, line := range lines {
	    	matches := memoryRegexp.FindStringSubmatch(line)
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
	return CPUFact{}, errors.New("Version string not found")
}
