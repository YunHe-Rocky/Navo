package compatibility

import (
	"fmt"

	"navo/internal/coreadapter"
	"navo/internal/domain/capture"
	domaincompat "navo/internal/domain/compatibility"
	"navo/internal/domain/core"
	"navo/internal/domain/endpoint"
)

const (
	ReasonCoreNotRegistered   = "CORE_NOT_REGISTERED"
	ReasonProtocolUnsupported = "PROTOCOL_UNSUPPORTED"
	ReasonCaptureUnsupported  = "CAPTURE_MODE_UNSUPPORTED"
	ReasonEndpointInvalid     = "ENDPOINT_INVALID"
	WarningTLSInsecure        = "TLS_CERTIFICATE_VERIFICATION_DISABLED"
	WarningSOCKSUDPUnverified = "SOCKS5_UDP_CAPABILITY_UNVERIFIED"
)

type Resolver struct {
	registry *coreadapter.Registry
}

func NewResolver(registry *coreadapter.Registry) *Resolver {
	if registry == nil {
		registry = coreadapter.NewDefaultRegistry()
	}
	return &Resolver{registry: registry}
}

func (r *Resolver) Resolve(
	coreType core.Type,
	version coreadapter.Version,
	target endpoint.Endpoint,
	captureMode capture.Mode,
) domaincompat.Result {
	if err := target.Validate(); err != nil {
		return domaincompat.Unsupported(domaincompat.Reason{
			Code: ReasonEndpointInvalid, Message: err.Error(),
		})
	}
	adapter, err := r.registry.Get(coreType)
	if err != nil {
		return domaincompat.Unsupported(domaincompat.Reason{
			Code: ReasonCoreNotRegistered, Message: fmt.Sprintf("%s is not installed", coreType),
		})
	}
	capabilities := adapter.Capabilities(version)
	var reasons []domaincompat.Reason
	if !capabilities.Protocols[target.Protocol] {
		reasons = append(reasons, domaincompat.Reason{
			Code:    ReasonProtocolUnsupported,
			Message: fmt.Sprintf("%s %s cannot compile %s endpoints", coreType, version.Raw, target.Protocol),
		})
	}
	if !capabilities.CaptureModes[captureMode] {
		reasons = append(reasons, domaincompat.Reason{
			Code:    ReasonCaptureUnsupported,
			Message: fmt.Sprintf("%s %s does not support capture mode %s", coreType, version.Raw, captureMode),
		})
	}
	if len(reasons) > 0 {
		return domaincompat.Unsupported(reasons...)
	}

	var warnings []domaincompat.Warning
	if tlsOptions, ok := endpointTLS(target.Spec); ok && tlsOptions.Insecure {
		warnings = append(warnings, domaincompat.Warning{
			Code: WarningTLSInsecure, Message: "server certificate verification is disabled for this endpoint",
		})
	}
	if spec, ok := target.Spec.(endpoint.SOCKS5ProxySpec); ok && spec.UDPRequested {
		warnings = append(warnings, domaincompat.Warning{
			Code:    WarningSOCKSUDPUnverified,
			Message: "SOCKS5 UDP support must be confirmed by a runtime capability probe",
		})
	}
	if len(warnings) > 0 {
		return domaincompat.Limited(warnings...)
	}
	return domaincompat.Supported()
}

func endpointTLS(spec endpoint.Spec) (endpoint.TLSOptions, bool) {
	switch value := spec.(type) {
	case endpoint.VLESSSpec:
		return value.TLS, value.TLS.Enabled
	case endpoint.VMessSpec:
		return value.TLS, value.TLS.Enabled
	case endpoint.TrojanSpec:
		return value.TLS, value.TLS.Enabled
	case endpoint.Hysteria2Spec:
		return value.TLS, value.TLS.Enabled
	case endpoint.TUICSpec:
		return value.TLS, value.TLS.Enabled
	default:
		return endpoint.TLSOptions{}, false
	}
}
