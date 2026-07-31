package source

type Type string

const (
	TypeAirportSubscription Type = "airport_subscription"
	TypeUpstreamProxy       Type = "upstream_proxy"
)

func (t Type) Valid() bool {
	switch t {
	case TypeAirportSubscription, TypeUpstreamProxy:
		return true
	default:
		return false
	}
}

func (t Type) String() string {
	return string(t)
}
