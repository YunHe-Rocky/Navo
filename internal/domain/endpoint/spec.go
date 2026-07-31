package endpoint

import (
	"strings"

	"navo/internal/domain/apperror"
)

type Spec interface {
	endpointSpec()
	Protocol() Protocol
	Validate() error
}

type VLESSSpec struct {
	UUID       string           `json:"uuid"`
	Flow       string           `json:"flow,omitempty"`
	Encryption string           `json:"encryption"`
	TLS        TLSOptions       `json:"tls"`
	Transport  TransportOptions `json:"transport"`
}

func (VLESSSpec) endpointSpec()      {}
func (VLESSSpec) Protocol() Protocol { return ProtocolVLESS }
func (s VLESSSpec) Validate() error {
	errs := requireNonBlank(nil, "spec.uuid", s.UUID)
	errs = append(errs, s.TLS.validate("spec.tls")...)
	errs = append(errs, s.Transport.validate("spec.transport")...)
	return errs.Err()
}

type VMessSpec struct {
	UUID      string           `json:"uuid"`
	AlterID   int              `json:"alterId"`
	Security  string           `json:"security"`
	TLS       TLSOptions       `json:"tls"`
	Transport TransportOptions `json:"transport"`
}

func (VMessSpec) endpointSpec()      {}
func (VMessSpec) Protocol() Protocol { return ProtocolVMess }
func (s VMessSpec) Validate() error {
	errs := requireNonBlank(nil, "spec.uuid", s.UUID)
	if s.AlterID < 0 {
		errs = append(errs, apperror.ValidationError{
			Field: "spec.alterId", Code: apperror.CodeOutOfRange, Message: "must be zero or greater",
		})
	}
	errs = append(errs, s.TLS.validate("spec.tls")...)
	errs = append(errs, s.Transport.validate("spec.transport")...)
	return errs.Err()
}

type TrojanSpec struct {
	PasswordRef string           `json:"passwordRef"`
	TLS         TLSOptions       `json:"tls"`
	Transport   TransportOptions `json:"transport"`
}

func (TrojanSpec) endpointSpec()      {}
func (TrojanSpec) Protocol() Protocol { return ProtocolTrojan }
func (s TrojanSpec) Validate() error {
	errs := requireNonBlank(nil, "spec.passwordRef", s.PasswordRef)
	if !s.TLS.Enabled {
		errs = append(errs, invalid("spec.tls.enabled", "must be true for Trojan"))
	}
	errs = append(errs, s.TLS.validate("spec.tls")...)
	errs = append(errs, s.Transport.validate("spec.transport")...)
	return errs.Err()
}

type ShadowsocksSpec struct {
	Method      string         `json:"method"`
	PasswordRef string         `json:"passwordRef"`
	Plugin      *PluginOptions `json:"plugin,omitempty"`
}

func (ShadowsocksSpec) endpointSpec()      {}
func (ShadowsocksSpec) Protocol() Protocol { return ProtocolShadowsocks }
func (s ShadowsocksSpec) Validate() error {
	errs := requireNonBlank(nil, "spec.method", s.Method)
	errs = requireNonBlank(errs, "spec.passwordRef", s.PasswordRef)
	if s.Plugin != nil && strings.TrimSpace(s.Plugin.Name) == "" {
		errs = append(errs, required("spec.plugin.name"))
	}
	return errs.Err()
}

type Hysteria2Spec struct {
	PasswordRef string       `json:"passwordRef"`
	TLS         TLSOptions   `json:"tls"`
	Obfs        *ObfsOptions `json:"obfs,omitempty"`
}

func (Hysteria2Spec) endpointSpec()      {}
func (Hysteria2Spec) Protocol() Protocol { return ProtocolHysteria2 }
func (s Hysteria2Spec) Validate() error {
	errs := requireNonBlank(nil, "spec.passwordRef", s.PasswordRef)
	if !s.TLS.Enabled {
		errs = append(errs, invalid("spec.tls.enabled", "must be true for Hysteria2"))
	}
	errs = append(errs, s.TLS.validate("spec.tls")...)
	if s.Obfs != nil {
		errs = requireNonBlank(errs, "spec.obfs.type", s.Obfs.Type)
		errs = requireNonBlank(errs, "spec.obfs.passwordRef", s.Obfs.PasswordRef)
	}
	return errs.Err()
}

type TUICSpec struct {
	UUID        string     `json:"uuid"`
	PasswordRef string     `json:"passwordRef"`
	Congestion  string     `json:"congestion,omitempty"`
	TLS         TLSOptions `json:"tls"`
}

func (TUICSpec) endpointSpec()      {}
func (TUICSpec) Protocol() Protocol { return ProtocolTUIC }
func (s TUICSpec) Validate() error {
	errs := requireNonBlank(nil, "spec.uuid", s.UUID)
	errs = requireNonBlank(errs, "spec.passwordRef", s.PasswordRef)
	if !s.TLS.Enabled {
		errs = append(errs, invalid("spec.tls.enabled", "must be true for TUIC"))
	}
	errs = append(errs, s.TLS.validate("spec.tls")...)
	return errs.Err()
}

type WireGuardSpec struct {
	PrivateKeyRef   string   `json:"privateKeyRef"`
	PeerPublicKey   string   `json:"peerPublicKey"`
	PreSharedKeyRef *string  `json:"preSharedKeyRef,omitempty"`
	LocalAddresses  []string `json:"localAddresses"`
	MTU             uint16   `json:"mtu,omitempty"`
}

func (WireGuardSpec) endpointSpec()      {}
func (WireGuardSpec) Protocol() Protocol { return ProtocolWireGuard }
func (s WireGuardSpec) Validate() error {
	errs := requireNonBlank(nil, "spec.privateKeyRef", s.PrivateKeyRef)
	errs = requireNonBlank(errs, "spec.peerPublicKey", s.PeerPublicKey)
	if len(s.LocalAddresses) == 0 {
		errs = append(errs, required("spec.localAddresses"))
	}
	return errs.Err()
}

type HTTPProxySpec struct {
	TLS         bool              `json:"tls"`
	UsernameRef *string           `json:"usernameRef,omitempty"`
	PasswordRef *string           `json:"passwordRef,omitempty"`
	Headers     map[string]string `json:"headers,omitempty"`
}

func (s HTTPProxySpec) endpointSpec() {}
func (s HTTPProxySpec) Protocol() Protocol {
	if s.TLS {
		return ProtocolHTTPS
	}
	return ProtocolHTTP
}
func (s HTTPProxySpec) Validate() error {
	var errs apperror.ValidationErrors
	if (present(s.UsernameRef) && !present(s.PasswordRef)) || (!present(s.UsernameRef) && present(s.PasswordRef)) {
		errs = append(errs, apperror.ValidationError{
			Field: "spec.credentials", Code: apperror.CodeMutuallyExclusive,
			Message: "usernameRef and passwordRef must either both be set or both be empty",
		})
	}
	transport := TransportOptions{Headers: s.Headers}
	errs = append(errs, transport.validate("spec")...)
	return errs.Err()
}

type SOCKS5ProxySpec struct {
	UsernameRef  *string `json:"usernameRef,omitempty"`
	PasswordRef  *string `json:"passwordRef,omitempty"`
	UDPRequested bool    `json:"udpRequested"`
}

func (SOCKS5ProxySpec) endpointSpec()      {}
func (SOCKS5ProxySpec) Protocol() Protocol { return ProtocolSOCKS5 }
func (s SOCKS5ProxySpec) Validate() error {
	if (present(s.UsernameRef) && !present(s.PasswordRef)) || (!present(s.UsernameRef) && present(s.PasswordRef)) {
		return apperror.ValidationErrors{{
			Field: "spec.credentials", Code: apperror.CodeMutuallyExclusive,
			Message: "usernameRef and passwordRef must either both be set or both be empty",
		}}
	}
	return nil
}

func requireNonBlank(errs apperror.ValidationErrors, field, value string) apperror.ValidationErrors {
	if strings.TrimSpace(value) == "" {
		return append(errs, required(field))
	}
	return errs
}

func present(value *string) bool {
	return value != nil && strings.TrimSpace(*value) != ""
}
