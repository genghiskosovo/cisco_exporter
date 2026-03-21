package bgp

// BgpSession holds BGP session data for IOS XE and NX-OS
type BgpSession struct {
	IP               string
	Asn              string
	Up               bool
	ReceivedPrefixes float64
	InputMessages    float64
	OutputMessages   float64
}

// BgpSession2 holds BGP session data for IOS XR (show bgp neighbor)
type BgpSession2 struct {
	Ip               string
	Asn              string
	Up               bool
	AcceptedPrefixes float64
	BestPath         float64
	PrefixAdvertised float64
	Description      string
}
