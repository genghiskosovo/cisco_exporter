package spanningtree

import (
	"testing"

	"github.com/lwlcom/cisco_exporter/rpc"
)

// Shape of "show spanning-tree" on a Nexus running rapid-PVST: one block per
// VLAN, a Root ID and a Bridge ID header, then the port rows. VLAN0010 has an
// upstream root; VLAN0020 is rooted here and says so; VLAN0030 is rooted here
// but only the matching bridge IDs show it.
const nxosOutput = `
VLAN0010
  Spanning tree enabled protocol rstp
  Root ID    Priority    24586
             Address     0023.04ee.be01
             Cost        3
             Port        4101 (port-channel5)
             Hello Time   2  sec  Max Age 20 sec  Forward Delay 15 sec

  Bridge ID  Priority    32778  (priority 32768 sys-id-ext 10)
             Address     547f.ee1d.a1c1
             Hello Time   2  sec  Max Age 20 sec  Forward Delay 15 sec

Interface        Role Sts Cost      Prio.Nbr Type
---------------- ---- --- --------- -------- --------------------------------
Po5              Root FWD 1         128.4101 (vPC) P2p
Eth1/51          Altn BLK 2         128.51   P2p
Eth1/8           Desg FWD 2         128.8    Edge P2p

VLAN0020
  Spanning tree enabled protocol rstp
  Root ID    Priority    24596
             Address     547f.ee1d.a1c1
             This bridge is the root
             Hello Time   2  sec  Max Age 20 sec  Forward Delay 15 sec

  Bridge ID  Priority    24596  (priority 24576 sys-id-ext 20)
             Address     547f.ee1d.a1c1
             Hello Time   2  sec  Max Age 20 sec  Forward Delay 15 sec

Interface        Role Sts Cost      Prio.Nbr Type
---------------- ---- --- --------- -------- --------------------------------
Eth1/8           Desg FWD 2         128.8    Edge P2p

VLAN0030
  Spanning tree enabled protocol rstp
  Root ID    Priority    24606
             Address     547f.ee1d.a1c1
             Hello Time   2  sec  Max Age 20 sec  Forward Delay 15 sec

  Bridge ID  Priority    24606  (priority 24576 sys-id-ext 30)
             Address     547f.ee1d.a1c1

Interface        Role Sts Cost      Prio.Nbr Type
---------------- ---- --- --------- -------- --------------------------------
Eth1/9           Desg FWD 2         128.9    P2p
`

const mstOutput = `
MST0
  Spanning tree enabled protocol mstp
  Root ID    Priority    32768
             Address     0023.04ee.be01
             Cost        0
             Port        128 (Ethernet1/1)

  Bridge ID  Priority    32768  (priority 32768 sys-id-ext 0)
             Address     547f.ee1d.a1c1

Interface        Role Sts Cost      Prio.Nbr Type
---------------- ---- --- --------- -------- --------------------------------
Eth1/1           Root FWD 2000      128.1    P2p
`

func TestParsePorts(t *testing.T) {
	c := &stpCollector{}
	items, err := c.Parse(rpc.NXOS, nxosOutput)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(items) != 5 {
		t.Fatalf("got %d ports, want 5: %+v", len(items), items)
	}

	want := STPInterface{Vlan: "10", Interface: "Po5", Role: "Root", Status: "FWD", Cost: 1, Type: "P2p"}
	if items[0] != want {
		t.Errorf("port 0 = %+v, want %+v", items[0], want)
	}
	// The blocked port is what the per-VLAN view exists to show.
	blocked := STPInterface{Vlan: "10", Interface: "Eth1/51", Role: "Altn", Status: "BLK", Cost: 2, Type: "P2p"}
	if items[1] != blocked {
		t.Errorf("port 1 = %+v, want %+v", items[1], blocked)
	}
	if items[2].Type != "Edge P2p" {
		t.Errorf("port 2 type = %q, want %q", items[2].Type, "Edge P2p")
	}
	if items[3].Vlan != "20" || items[4].Vlan != "30" {
		t.Errorf("later VLANs = %q, %q; want 20, 30", items[3].Vlan, items[4].Vlan)
	}
}

func TestParseVlansRootBridge(t *testing.T) {
	c := &stpCollector{}
	instances, err := c.ParseVlans(rpc.NXOS, nxosOutput)
	if err != nil {
		t.Fatalf("ParseVlans: %v", err)
	}
	if len(instances) != 3 {
		t.Fatalf("got %d instances, want 3: %+v", len(instances), instances)
	}

	v10 := instances[0]
	if v10.Vlan != "10" || v10.Protocol != "rstp" {
		t.Errorf("vlan 10 header = %q/%q", v10.Vlan, v10.Protocol)
	}
	if v10.IsRoot() {
		t.Error("vlan 10: root is upstream, must not report this bridge as root")
	}
	if v10.RootAddress != "0023.04ee.be01" || v10.BridgeAddress != "547f.ee1d.a1c1" {
		t.Errorf("vlan 10 addresses = %q / %q", v10.RootAddress, v10.BridgeAddress)
	}
	if v10.RootPriority != 24586 || v10.BridgePriority != 32778 || v10.RootCost != 3 {
		t.Errorf("vlan 10 numbers = %v / %v / %v", v10.RootPriority, v10.BridgePriority, v10.RootCost)
	}

	if !instances[1].IsRoot() {
		t.Error(`vlan 20: "This bridge is the root" must be honoured`)
	}
	if !instances[2].IsRoot() {
		t.Error("vlan 30: matching root and bridge IDs mean this bridge is root")
	}
}

func TestParseMstInstance(t *testing.T) {
	c := &stpCollector{}
	items, err := c.Parse(rpc.NXOS, mstOutput)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(items) != 1 || items[0].Vlan != "0" {
		t.Fatalf("MST ports = %+v, want one port in instance 0", items)
	}
	instances, err := c.ParseVlans(rpc.NXOS, mstOutput)
	if err != nil {
		t.Fatalf("ParseVlans: %v", err)
	}
	if len(instances) != 1 || instances[0].Vlan != "0" || instances[0].IsRoot() {
		t.Fatalf("MST instances = %+v, want one non-root instance 0", instances)
	}
}

func TestParseUnsupportedOS(t *testing.T) {
	c := &stpCollector{}
	if _, err := c.Parse(rpc.IOSXR, nxosOutput); err == nil {
		t.Error("Parse on IOS XR: want error")
	}
	if _, err := c.ParseVlans(rpc.IOSXR, nxosOutput); err == nil {
		t.Error("ParseVlans on IOS XR: want error")
	}
}
