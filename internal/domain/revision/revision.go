package revision

import (
	"context"
	"time"

	"navo/internal/domain/core"
	"navo/internal/domain/source"
)

type Status string

const (
	StatusCandidate Status = "candidate"
	StatusActive    Status = "active"
	StatusFailed    Status = "failed"
)

type Revision struct {
	ID                string
	CoreType          core.Type
	SourceType        source.Type
	EndpointReference string
	ConfigHash        string
	ConfigPath        string
	CreatedAt         time.Time
	ValidationStatus  Status
	StartupStatus     Status
	HealthStatus      Status
}

type Repository interface {
	SaveCandidate(ctx context.Context, value Revision) error
	MarkActive(ctx context.Context, revisionID string) error
	MarkFailed(ctx context.Context, revisionID, stage string) error
}
