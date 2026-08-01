// Package ipc defines the inter-process communication protocol for Navo.
// Phase 1 uses JSON encoding; protobuf will be added in Phase 2.
package ipc

import (
	"encoding/json"
	"fmt"
	"sync/atomic"
	"time"
)

// MessageType indicates the kind of IPC message.
type MessageType string

const (
	TypeRequest  MessageType = "REQUEST"
	TypeResponse MessageType = "RESPONSE"
	TypeError    MessageType = "ERROR"
	TypeEvent    MessageType = "EVENT"
)

// Envelope wraps all IPC messages with routing metadata.
type Envelope struct {
	RequestID string          `json:"request_id"`
	Method    string          `json:"method"`
	Payload   json.RawMessage `json:"payload,omitempty"`
	Timestamp int64           `json:"timestamp"`
	Type      MessageType     `json:"type"`
}

// NewEnvelope creates a new message envelope.
func NewEnvelope(method string, payload interface{}, msgType MessageType) (*Envelope, error) {
	var raw json.RawMessage
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("marshal payload: %w", err)
		}
		raw = data
	}

	return &Envelope{
		RequestID: newRequestID(),
		Method:    method,
		Payload:   raw,
		Timestamp: time.Now().UnixMilli(),
		Type:      msgType,
	}, nil
}

// NewRequest creates a REQUEST envelope.
func NewRequest(method string, payload interface{}) (*Envelope, error) {
	return NewEnvelope(method, payload, TypeRequest)
}

// NewResponse creates a RESPONSE envelope for a given request.
func NewResponse(requestID string, payload interface{}) (*Envelope, error) {
	var raw json.RawMessage
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("marshal payload: %w", err)
		}
		raw = data
	}

	return &Envelope{
		RequestID: requestID,
		Timestamp: time.Now().UnixMilli(),
		Type:      TypeResponse,
		Payload:   raw,
	}, nil
}

// NewError creates an ERROR envelope for a given request.
func NewError(requestID string, code string, message string) (*Envelope, error) {
	errPayload := IPCError{
		Code:    code,
		Message: message,
	}
	raw, _ := json.Marshal(errPayload)

	return &Envelope{
		RequestID: requestID,
		Timestamp: time.Now().UnixMilli(),
		Type:      TypeError,
		Payload:   raw,
	}, nil
}

// NewEvent creates an EVENT envelope (server push, no request ID needed).
func NewEvent(method string, payload interface{}) (*Envelope, error) {
	var raw json.RawMessage
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("marshal payload: %w", err)
		}
		raw = data
	}

	return &Envelope{
		RequestID: "", // events don't have request correlation
		Method:    method,
		Payload:   raw,
		Timestamp: time.Now().UnixMilli(),
		Type:      TypeEvent,
	}, nil
}

// Marshal serializes the envelope to JSON bytes.
func Marshal(e *Envelope) ([]byte, error) {
	data, err := json.Marshal(e)
	if err != nil {
		return nil, fmt.Errorf("marshal envelope: %w", err)
	}
	return data, nil
}

// Unmarshal deserializes an envelope from JSON bytes.
func Unmarshal(data []byte) (*Envelope, error) {
	var e Envelope
	if err := json.Unmarshal(data, &e); err != nil {
		return nil, fmt.Errorf("unmarshal envelope: %w", err)
	}
	return &e, nil
}

// UnmarshalPayload deserializes the payload into the given type.
func (e *Envelope) UnmarshalPayload(v interface{}) error {
	if len(e.Payload) == 0 {
		return nil
	}
	return json.Unmarshal(e.Payload, v)
}

// IPCError is the standard error response body.
type IPCError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

var requestCounter atomic.Int64

func newRequestID() string {
	return fmt.Sprintf("req-%d-%d", time.Now().UnixNano(), requestCounter.Add(1))
}

// ── Method Constants ──

// Core control methods.
const (
	MethodCoreStatus     = "core.status"
	MethodCoreHealth     = "core.health"
	MethodCoreSwapConfig = "core.swap_config"
)

// Outbound management methods.
const (
	MethodOutboundList   = "outbound.list"
	MethodOutboundCreate = "outbound.create"
	MethodOutboundUpdate = "outbound.update"
	MethodOutboundDelete = "outbound.delete"
	MethodOutboundTest   = "outbound.test"
)

// Rule management methods.
const (
	MethodRuleList     = "rule.list"
	MethodRuleCreate   = "rule.create"
	MethodRuleUpdate   = "rule.update"
	MethodRuleDelete   = "rule.delete"
	MethodRuleSimulate = "rule.simulate"
)

// Config management methods.
const (
	MethodConfigCompile   = "config.compile"
	MethodConfigApply     = "config.apply"
	MethodConfigRollback  = "config.rollback"
	MethodConfigRevisions = "config.revisions"
)

// TUN control methods.
const (
	MethodTUNEnable  = "tun.enable"
	MethodTUNDisable = "tun.disable"
	MethodTUNStatus  = "tun.status"
	MethodTUNConfig  = "tun.config"
)

// Event methods (server → client push).
const (
	EventStateChanged    = "event.state_changed"
	EventMetricUpdate    = "event.metric_update"
	EventIPChanged       = "event.ip_changed"
	EventCoreCrashed     = "event.core_crashed"
	EventRuleHit         = "event.rule_hit"
	EventTUNStateChanged = "event.tun_state_changed"
	EventTUNError        = "event.tun_error"
)

// Subscription management methods.
const (
	MethodSubAdd     = "subscription.add"
	MethodSubUpdate  = "subscription.update"
	MethodSubRemove  = "subscription.remove"
	MethodSubList    = "subscription.list"
	MethodSubRefresh = "subscription.refresh"
)

// Metrics methods.
const (
	MethodMetricsCurrent = "metrics.current"
	MethodMetricsHistory = "metrics.history"
)

// IP detection methods.
const (
	MethodIPCheck    = "ip.check"
	MethodIPCheckAll = "ip.check_all"
)
