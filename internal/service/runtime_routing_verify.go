package service

import "time"

type RuntimeRoutingVerification struct {
	Verified   bool                        `json:"verified"`
	Sites      map[string]SiteVerification `json:"sites,omitempty"`
	VerifiedAt time.Time                   `json:"verified_at"`
}
