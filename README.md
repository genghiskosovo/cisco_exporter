# cisco_exporter
Exporter for metrics from devices running Cisco (NX-OS/IOS XE/IOS) (via SSH) https://prometheus.io/

The basic structure is based on https://github.com/czerwonk/junos_exporter

# flags
Name     | Description | Default
---------|-------------|---------
version | Print version information. |
web.listen-address | Address on which to expose metrics and web interface. | :9362
web.telemetry-path | Path under which to expose metrics. | /metrics
ssh.targets | Comma seperated list of hosts to scrape |
ssh.user | Username to use for SSH connection | cisco_exporter
ssh.keyfile | Key file to use for SSH connection | cisco_exporter
ssh.timeout | Timeout in seconds to use for SSH connection | 5
debug | Show verbose debug output | false
legacy.ciphers | Allow insecure legacy ciphers: aes128-cbc 3des-cbc aes192-cbc aes256-cbc | false
config.file | Path to config file |

# metrics

All metrics are enabled by default. To disable something pass a flag `--<name>.enabled=false`, where `<name>` is the name of the metric.

Name     | Description | OS
---------|-------------|----
bgp | BGP (message count, prefix counts per peer, session state) | IOS XE/NX-OS
cdp | CDP neighbours (local interface, neighbour, neighbour interface) | NX-OS/IOS XE/IOS/IOS XR
environment | Environment (temperatures, state of power supply) | NX-OS/IOS XE/IOS
facts | System informations (OS Version, memory: total/used/free, cpu: 5s/1m/5m/interrupts) | IOS XE/IOS
interfaces | Interfaces (transmitted/received: bytes/errors/drops, admin/oper state) | NX-OS (*_drops is always 0)/IOS XE/IOS
lag | Port-channel membership (which physical ports are bundled into which aggregate) | NX-OS/IOS XE/IOS
optics | Optical signals (tx/rx) | NX-OS/IOS XE/IOS
stp | Spanning tree (per VLAN: port role, state, path cost, root bridge, bridge priority) | NX-OS/IOS XE/IOS

## Topology metrics

These three families exist to let a consumer reconstruct the network graph, and
their names and label sets mirror [junos_exporter](../junos_exporter) one for
one so a Cisco series and a Juniper series are interchangeable to a query.

```
cisco_cdp_neighbor{target, local_interface, neighbor, neighbor_interface}

cisco_interface_lag_member{target, aggregate, member}

cisco_stp_interface_role{target, vlan, interface, type}    1=Root 2=Desg 3=Altn 4=Bak 0=other
cisco_stp_interface_status{target, vlan, interface, type}  1=FWD 2=BLK 3=LRN 4=LST 5=DIS 0=other
cisco_stp_interface_cost{target, vlan, interface, type}
cisco_stp_root_bridge{target, vlan}
cisco_stp_bridge_priority{target, vlan}
```

**Spanning tree is per VLAN.** `show spanning-tree` prints one block per VLAN
under PVST / rapid-PVST, so every series carries the VLAN it belongs to, and
the `vlan` label holds the MST instance id on a device running MST instead.
Which VLANs a trunk carries needs no separate metric: a port appears in a
VLAN's block only while that VLAN is active on it.

`cisco_stp_root_bridge` is read from the device's own Root ID / Bridge ID
header rather than inferred from port roles. Inference — "has designated ports
but no root port" — is wrong for any switch whose only trunk for a VLAN is
down: it is then root of its own island and reports as root of the VLAN. It
survives only as a fallback for an instance whose header does not parse.

`cisco_interface_lag_member` is what maps a spanning-tree port named `Po5` onto
a physical link, since CDP and interface descriptions only ever name physical
ports. Every member the device lists is reported whatever its state flag;
whether a member is up is already answered by `cisco_interface_up`.

## Install
```bash
go get -u github.com/lwlcom/cisco_exporter
```

## Usage

### Binary
```bash
./cisco_exporter -ssh.targets="host1.example.com,host2.example.com:2233,172.16.0.1" -ssh.keyfile=cisco_exporter
```

```bash
./cisco_exporter -config.file=config.yml
```

## Config file
The exporter can be configured with a YAML based config file:

```yaml
---
debug: false
legacy_ciphers: false
# default values
timeout: 5
batch_size: 10000
username: default-username
password: default-password
key_file: /path/to/key

devices:
  - host: host1.example.com
    key_file: /path/to/key
    timeout: 5
    batch_size: 10000
    features: # enable/disable per host
      bgp: false
  - host: host2.example.com:2233
    username: exporter
    password: secret

features:
  bgp: true
  cdp: true
  environment: true
  facts: true
  interfaces: true
  lag: true
  optics: true
  stp: true

```

## Third Party Components
This software uses components of the following projects
* Prometheus Go client library (https://github.com/prometheus/client_golang)

## License
(c) Martin Poppen, 2018. Licensed under [MIT](LICENSE) license.

## Prometheus
see https://prometheus.io/
