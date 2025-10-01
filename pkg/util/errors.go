package util

// SoftErrorMode allows for skipping certain non-fatal errors from breaking execution
type SoftErrorMode struct {
	Enabled bool
}

var _softErrorMode *SoftErrorMode

func InitSoftErrorMode() {
	_softErrorMode = &SoftErrorMode{false}
}

func SetSoftErrorMode(newValue bool) {
	_softErrorMode.Enabled = newValue
}

func IsSoftErrorOn() bool {
	return _softErrorMode.Enabled
}
