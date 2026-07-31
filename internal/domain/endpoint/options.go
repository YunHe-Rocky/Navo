package endpoint

import (
	"net"
	"net/url"
	"strings"

	"navo/internal/domain/apperror"
)

type TLSOptions struct {
	Enabled    bool            `json:"enabled"`
	ServerName string          `json:"serverName,omitempty"`
	ALPN       []string        `json:"alpn,omitempty"`
	Insecure   bool            `json:"insecure"`
	Reality    *RealityOptions `json:"reality,omitempty"`
}

type RealityOptions struct {
	PublicKey string `json:"publicKey"`
	ShortID   string `json:"shortId,omitempty"`
}

type TransportType string

const (
	TransportTCP         TransportType = "tcp"
	TransportWebSocket   TransportType = "websocket"
	TransportHTTPUpgrade TransportType = "httpupgrade"
	TransportGRPC        TransportType = "grpc"
	TransportHTTP        TransportType = "http"
	TransportQUIC        TransportType = "quic"
)

type TransportOptions struct {
	Type        TransportType     `json:"type"`
	Path        string            `json:"path,omitempty"`
	Host        string            `json:"host,omitempty"`
	ServiceName string            `json:"serviceName,omitempty"`
	Headers     map[string]string `json:"headers,omitempty"`
}

type PluginOptions struct {
	Name    string            `json:"name"`
	Options map[string]string `json:"options,omitempty"`
}

type ObfsOptions struct {
	Type        string `json:"type"`
	PasswordRef string `json:"passwordRef"`
}

func (o TLSOptions) validate(field string) apperror.ValidationErrors {
	var errs apperror.ValidationErrors
	if o.Reality != nil {
		if !o.Enabled {
			errs = append(errs, invalid(field+".enabled", "must be true when Reality is configured"))
		}
		if strings.TrimSpace(o.Reality.PublicKey) == "" {
			errs = append(errs, required(field+".reality.publicKey"))
		}
	}
	return errs
}

func (o TransportOptions) validate(field string) apperror.ValidationErrors {
	var errs apperror.ValidationErrors
	switch o.Type {
	case "", TransportTCP:
	case TransportWebSocket, TransportHTTPUpgrade, TransportHTTP:
		if o.Path != "" && !strings.HasPrefix(o.Path, "/") {
			errs = append(errs, invalid(field+".path", "must start with /"))
		}
	case TransportGRPC:
		if strings.TrimSpace(o.ServiceName) == "" {
			errs = append(errs, required(field+".serviceName"))
		}
	case TransportQUIC:
	default:
		errs = append(errs, invalid(field+".type", "unsupported transport type"))
	}
	for name, value := range o.Headers {
		if strings.TrimSpace(name) == "" || strings.ContainsAny(name, "\r\n") {
			errs = append(errs, invalid(field+".headers", "header names must be non-empty single-line values"))
		}
		if strings.ContainsAny(value, "\r\n") {
			errs = append(errs, invalid(field+".headers", "header values must be single-line values"))
		}
	}
	return errs
}

func validServer(server string) bool {
	value := strings.TrimSpace(server)
	if value == "" {
		return false
	}
	if net.ParseIP(value) != nil {
		return true
	}
	parsed, err := url.Parse("dns://" + value)
	return err == nil && parsed.Hostname() == value && !strings.ContainsAny(value, " /\\")
}

func required(field string) apperror.ValidationError {
	return apperror.ValidationError{Field: field, Code: apperror.CodeRequired, Message: "is required"}
}

func invalid(field, message string) apperror.ValidationError {
	return apperror.ValidationError{Field: field, Code: apperror.CodeInvalid, Message: message}
}
