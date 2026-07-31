package endpoint

import (
	"strings"
	"time"

	"navo/internal/domain/apperror"
)

type UpstreamProtocol string

const (
	UpstreamHTTP   UpstreamProtocol = "http"
	UpstreamHTTPS  UpstreamProtocol = "https"
	UpstreamSOCKS5 UpstreamProtocol = "socks5"
)

type UDPPolicy string

const (
	UDPPolicyDisabled UDPPolicy = "disabled"
	UDPPolicyPrefer   UDPPolicy = "prefer"
	UDPPolicyRequire  UDPPolicy = "require"
)

type UpstreamProxy struct {
	ID          string           `json:"id"`
	Name        string           `json:"name"`
	Protocol    UpstreamProtocol `json:"protocol"`
	Server      string           `json:"server"`
	Port        uint16           `json:"port"`
	UsernameRef *string          `json:"usernameRef,omitempty"`
	PasswordRef *string          `json:"passwordRef,omitempty"`
	TLS         bool             `json:"tls"`
	UDPPolicy   UDPPolicy        `json:"udpPolicy"`
	Enabled     bool             `json:"enabled"`
	CreatedAt   time.Time        `json:"createdAt"`
	UpdatedAt   time.Time        `json:"updatedAt"`
}

func (p UpstreamProxy) Validate() error {
	var errs apperror.ValidationErrors
	errs = requireNonBlank(errs, "id", p.ID)
	errs = requireNonBlank(errs, "name", p.Name)
	if !validServer(p.Server) {
		errs = append(errs, invalid("server", "must be a valid IP address or hostname"))
	}
	if p.Port == 0 {
		errs = append(errs, apperror.ValidationError{
			Field: "port", Code: apperror.CodeOutOfRange, Message: "must be between 1 and 65535",
		})
	}
	switch p.Protocol {
	case UpstreamHTTP:
		if p.TLS {
			errs = append(errs, invalid("tls", "must be false for the http protocol"))
		}
	case UpstreamHTTPS:
		if !p.TLS {
			errs = append(errs, invalid("tls", "must be true for the https protocol"))
		}
	case UpstreamSOCKS5:
	default:
		errs = append(errs, invalid("protocol", "must be http, https or socks5"))
	}
	switch p.UDPPolicy {
	case UDPPolicyDisabled, UDPPolicyPrefer, UDPPolicyRequire:
	default:
		errs = append(errs, invalid("udpPolicy", "must be disabled, prefer or require"))
	}
	if p.Protocol != UpstreamSOCKS5 && p.UDPPolicy != UDPPolicyDisabled {
		errs = append(errs, invalid("udpPolicy", "HTTP upstream proxies cannot carry UDP"))
	}
	if (present(p.UsernameRef) && !present(p.PasswordRef)) || (!present(p.UsernameRef) && present(p.PasswordRef)) {
		errs = append(errs, apperror.ValidationError{
			Field: "credentials", Code: apperror.CodeMutuallyExclusive,
			Message: "usernameRef and passwordRef must either both be set or both be empty",
		})
	}
	if present(p.UsernameRef) && strings.ContainsAny(*p.UsernameRef, "\r\n") {
		errs = append(errs, invalid("usernameRef", "must be a single-line credential reference"))
	}
	return errs.Err()
}
