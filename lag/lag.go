package lag

// LagMember is one physical interface bundled into a port-channel.
type LagMember struct {
	// Aggregate is the port-channel as the device spells it ("Po5"), left
	// unnormalised like every other interface label this exporter emits.
	Aggregate string
	Member    string
}
