package lag

import (
	"testing"

	"github.com/lwlcom/cisco_exporter/rpc"
)

// "show port-channel summary" on NX-OS. Po5 wraps its member list onto a
// second line, which is the case that makes this a state machine rather than
// a regex over each line. Po10 is down, and its member is still listed.
const nxosSummary = `
Flags:  D - Down        P - Up in port-channel (members)
        I - Individual  H - Hot-standby (LACP only)
        s - Suspended   r - Module-removed
        b - BFD Session Wait
        S - Switched    R - Routed
        U - Up (port-channel)
        p - Up in delay-lacp mode (member)
        M - Not in use. Min-links not met
--------------------------------------------------------------------------------
Group Port-       Type     Protocol  Member Ports
      Channel
--------------------------------------------------------------------------------
1     Po1(SU)     Eth      LACP      Eth1/1(P)    Eth1/2(P)
5     Po5(SU)     Eth      LACP      Eth1/31(P)   Eth1/32(P)   Eth1/33(P)
                                     Eth1/34(P)
10    Po10(SD)    Eth      NONE      Eth1/17(D)
`

// "show etherchannel summary" on IOS: no Type column, and a pair of count
// lines above the table.
const iosSummary = `
Flags:  D - down        P - bundled in port-channel
        I - stand-alone s - suspended
        H - Hot-standby (LACP only)
        R - Layer3      S - Layer2
        U - in use      f - failed to allocate aggregator

        M - not in use, minimum links not met
        u - unsuitable for bundling
        w - waiting to be aggregated
        d - default port

Number of channel-groups in use: 1
Number of aggregators:           1

Group  Port-channel  Protocol    Ports
------+-------------+-----------+-----------------------------------------------
1      Po1(SU)         LACP      Gi0/1(P)    Gi0/2(P)
`

func members(t *testing.T, ostype, output string) map[string][]string {
	t.Helper()
	c := &lagCollector{}
	items, err := c.Parse(ostype, output)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	out := map[string][]string{}
	for _, m := range items {
		out[m.Aggregate] = append(out[m.Aggregate], m.Member)
	}
	return out
}

func TestParseNxos(t *testing.T) {
	got := members(t, rpc.NXOS, nxosSummary)
	if len(got) != 3 {
		t.Fatalf("got %d aggregates, want 3: %v", len(got), got)
	}
	if want := 2; len(got["Po1"]) != want {
		t.Errorf("Po1 members = %v, want %d", got["Po1"], want)
	}
	// Four, only if the wrapped continuation line was picked up.
	if want := 4; len(got["Po5"]) != want {
		t.Errorf("Po5 members = %v, want %d", got["Po5"], want)
	}
	if got["Po5"][3] != "Eth1/34" {
		t.Errorf("Po5 last member = %q, want Eth1/34", got["Po5"][3])
	}
	// A down member is configured membership all the same; whether it is up
	// is cisco_interface_up's job.
	if len(got["Po10"]) != 1 || got["Po10"][0] != "Eth1/17" {
		t.Errorf("Po10 members = %v, want [Eth1/17]", got["Po10"])
	}
}

func TestParseIos(t *testing.T) {
	got := members(t, rpc.IOS, iosSummary)
	if len(got) != 1 {
		t.Fatalf("got %d aggregates, want 1: %v", len(got), got)
	}
	if len(got["Po1"]) != 2 || got["Po1"][0] != "Gi0/1" || got["Po1"][1] != "Gi0/2" {
		t.Errorf("Po1 members = %v, want [Gi0/1 Gi0/2]", got["Po1"])
	}
}

// The flag legend above the table is full of parenthesised text; none of it
// may be read as a member.
func TestParseIgnoresLegend(t *testing.T) {
	c := &lagCollector{}
	items, err := c.Parse(rpc.NXOS, `
Flags:  D - Down        P - Up in port-channel (members)
        H - Hot-standby (LACP only)
        U - Up (port-channel)
        p - Up in delay-lacp mode (member)
`)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("got %+v, want no members from a legend alone", items)
	}
}

func TestCommandPerOS(t *testing.T) {
	if got := command(rpc.NXOS); got != "show port-channel summary" {
		t.Errorf("NX-OS command = %q", got)
	}
	if got := command(rpc.IOS); got != "show etherchannel summary" {
		t.Errorf("IOS command = %q", got)
	}
}

func TestParseUnsupportedOS(t *testing.T) {
	c := &lagCollector{}
	if _, err := c.Parse(rpc.IOSXR, nxosSummary); err == nil {
		t.Error("Parse on IOS XR: want error")
	}
}
