package selection

import (
	"strings"
	"time"

	"navo/internal/domain/apperror"
	"navo/internal/domain/capture"
	"navo/internal/domain/core"
	"navo/internal/domain/source"
)

type ActiveSelection struct {
	CoreType        core.Type    `json:"coreType"`
	SourceType      source.Type  `json:"sourceType"`
	CaptureMode     capture.Mode `json:"captureMode"`
	SubscriptionID  *string      `json:"subscriptionId,omitempty"`
	EndpointID      *string      `json:"endpointId,omitempty"`
	UpstreamProxyID *string      `json:"upstreamProxyId,omitempty"`
	UpdatedAt       time.Time    `json:"updatedAt"`
}

func (s ActiveSelection) Validate() error {
	var errs apperror.ValidationErrors
	if !s.CoreType.Valid() {
		errs = append(errs, invalid("coreType", "must be mihomo, xray or sing-box"))
	}
	if !s.SourceType.Valid() {
		errs = append(errs, invalid("sourceType", "must be airport_subscription or upstream_proxy"))
	}
	if !s.CaptureMode.Valid() {
		errs = append(errs, invalid("captureMode", "must be off, system_proxy or tun"))
	}

	switch s.SourceType {
	case source.TypeAirportSubscription:
		errs = requireID(errs, "subscriptionId", s.SubscriptionID)
		errs = requireID(errs, "endpointId", s.EndpointID)
		if present(s.UpstreamProxyID) {
			errs = append(errs, exclusive("upstreamProxyId", "must be empty for airport_subscription"))
		}
	case source.TypeUpstreamProxy:
		errs = requireID(errs, "upstreamProxyId", s.UpstreamProxyID)
		if present(s.SubscriptionID) {
			errs = append(errs, exclusive("subscriptionId", "must be empty for upstream_proxy"))
		}
		if present(s.EndpointID) {
			errs = append(errs, exclusive("endpointId", "must be empty for upstream_proxy"))
		}
	}

	return errs.Err()
}

func present(value *string) bool {
	return value != nil && strings.TrimSpace(*value) != ""
}

func requireID(errs apperror.ValidationErrors, field string, value *string) apperror.ValidationErrors {
	if !present(value) {
		return append(errs, apperror.ValidationError{
			Field: field, Code: apperror.CodeRequired, Message: "must be a non-empty identifier",
		})
	}
	return errs
}

func invalid(field, message string) apperror.ValidationError {
	return apperror.ValidationError{Field: field, Code: apperror.CodeInvalid, Message: message}
}

func exclusive(field, message string) apperror.ValidationError {
	return apperror.ValidationError{Field: field, Code: apperror.CodeMutuallyExclusive, Message: message}
}
