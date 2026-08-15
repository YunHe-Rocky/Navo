package service

type RuntimeRoutingVerification struct {
	Verified bool                        `json:"verified"`
	Sites    map[string]SiteVerification `json:"sites,omitempty"`
}
