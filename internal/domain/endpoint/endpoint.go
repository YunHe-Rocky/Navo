package endpoint

import (
	"strings"
	"time"

	"navo/internal/domain/apperror"
)

type Endpoint struct {
	ID          string    `json:"id"`
	ProviderID  string    `json:"providerId"`
	Name        string    `json:"name"`
	Protocol    Protocol  `json:"protocol"`
	Server      string    `json:"server"`
	Port        uint16    `json:"port"`
	Enabled     bool      `json:"enabled"`
	SpecVersion int       `json:"specVersion"`
	Spec        Spec      `json:"-"`
	RawFormat   string    `json:"rawFormat,omitempty"`
	RawHash     string    `json:"rawHash,omitempty"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

func (e Endpoint) Validate() error {
	var errs apperror.ValidationErrors
	errs = requireNonBlank(errs, "id", e.ID)
	errs = requireNonBlank(errs, "providerId", e.ProviderID)
	errs = requireNonBlank(errs, "name", e.Name)
	if !e.Protocol.Valid() {
		errs = append(errs, invalid("protocol", "unsupported endpoint protocol"))
	}
	if !validServer(e.Server) {
		errs = append(errs, invalid("server", "must be a valid IP address or hostname"))
	}
	if e.Port == 0 {
		errs = append(errs, apperror.ValidationError{
			Field: "port", Code: apperror.CodeOutOfRange, Message: "must be between 1 and 65535",
		})
	}
	if e.SpecVersion < 1 {
		errs = append(errs, apperror.ValidationError{
			Field: "specVersion", Code: apperror.CodeOutOfRange, Message: "must be at least 1",
		})
	}
	if e.Spec == nil {
		errs = append(errs, required("spec"))
		return errs.Err()
	}
	if e.Spec.Protocol() != e.Protocol {
		errs = append(errs, invalid("spec", "protocol-specific spec does not match endpoint protocol"))
	}
	if err := e.Spec.Validate(); err != nil {
		if nested, ok := err.(apperror.ValidationErrors); ok {
			errs = append(errs, nested...)
		} else {
			errs = append(errs, invalid("spec", err.Error()))
		}
	}
	if strings.TrimSpace(e.RawHash) != "" && len(strings.TrimSpace(e.RawHash)) < 32 {
		errs = append(errs, invalid("rawHash", "must be a cryptographic hash"))
	}
	return errs.Err()
}
