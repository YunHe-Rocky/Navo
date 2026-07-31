package capture

type Mode string

const (
	ModeOff         Mode = "off"
	ModeSystemProxy Mode = "system_proxy"
	ModeTUN         Mode = "tun"
)

func (m Mode) Valid() bool {
	switch m {
	case ModeOff, ModeSystemProxy, ModeTUN:
		return true
	default:
		return false
	}
}

func (m Mode) String() string {
	return string(m)
}
