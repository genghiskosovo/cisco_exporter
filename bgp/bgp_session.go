package bgp

type BgpSession struct {
	IP               string
	Asn              string
	Up               bool
	ReceivedPrefixes float64
	InputMessages    float64
	OutputMessages   float64
}

type BgpSession2 struct {
	Ip          string
	Asn                 string
	Up                  bool
	AcceptedPrefixes    float64
	BestPath            float64
	PrefixAdvertised    float64
	Description         string
}
