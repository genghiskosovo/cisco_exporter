package optics

type Optics struct {
	RxPower float64
	TxPower float64

	TxHighAlarm float64
	TxHighWarn  float64
	TxLowWarn   float64
	TxLowAlarm  float64
	RxHighAlarm float64
	RxHighWarn  float64
	RxLowWarn   float64
	RxLowAlarm  float64

	HasThresholds bool
}
