package ipc

import (
	"encoding/json"
	"testing"
)

func TestNewRequest(t *testing.T) {
	payload := CoreStartRequest{
		ConfigPath: "/test/config.json",
	}

	env, err := NewRequest(MethodCoreStart, payload)
	if err != nil {
		t.Fatalf("NewRequest() error: %v", err)
	}

	if env.Method != MethodCoreStart {
		t.Errorf("Method = %s, want %s", env.Method, MethodCoreStart)
	}
	if env.Type != TypeRequest {
		t.Errorf("Type = %s, want %s", env.Type, TypeRequest)
	}
	if env.RequestID == "" {
		t.Error("RequestID is empty")
	}
	if env.Timestamp == 0 {
		t.Error("Timestamp is 0")
	}

	// Verify payload can be round-tripped
	var decoded CoreStartRequest
	if err := env.UnmarshalPayload(&decoded); err != nil {
		t.Fatalf("UnmarshalPayload() error: %v", err)
	}
	if decoded.ConfigPath != "/test/config.json" {
		t.Errorf("ConfigPath = %s", decoded.ConfigPath)
	}
}

func TestNewRequest_NilPayload(t *testing.T) {
	env, err := NewRequest("test.method", nil)
	if err != nil {
		t.Fatalf("NewRequest() error: %v", err)
	}

	if len(env.Payload) > 0 {
		t.Error("Payload should be empty for nil input")
	}
}

func TestNewResponse(t *testing.T) {
	payload := CoreStartResponse{
		PID:    12345,
		Status: "running",
	}

	env, err := NewResponse("req-123", payload)
	if err != nil {
		t.Fatalf("NewResponse() error: %v", err)
	}

	if env.Type != TypeResponse {
		t.Errorf("Type = %s, want %s", env.Type, TypeResponse)
	}
	if env.RequestID != "req-123" {
		t.Errorf("RequestID = %s, want req-123", env.RequestID)
	}

	var decoded CoreStartResponse
	env.UnmarshalPayload(&decoded)
	if decoded.PID != 12345 {
		t.Errorf("PID = %d", decoded.PID)
	}
}

func TestNewError(t *testing.T) {
	env, err := NewError("req-456", "CORE_001", "binary not found")
	if err != nil {
		t.Fatalf("NewError() error: %v", err)
	}

	if env.Type != TypeError {
		t.Errorf("Type = %s, want %s", env.Type, TypeError)
	}

	var errBody IPCError
	env.UnmarshalPayload(&errBody)
	if errBody.Code != "CORE_001" {
		t.Errorf("Code = %s, want CORE_001", errBody.Code)
	}
	if errBody.Message != "binary not found" {
		t.Errorf("Message = %s", errBody.Message)
	}
}

func TestNewEvent(t *testing.T) {
	payload := StateChangedEvent{
		From: StateChangeInfo{State: "running", Timestamp: 1000},
		To:   StateChangeInfo{State: "stopped", Timestamp: 2000},
	}

	env, err := NewEvent(EventStateChanged, payload)
	if err != nil {
		t.Fatalf("NewEvent() error: %v", err)
	}

	if env.Type != TypeEvent {
		t.Errorf("Type = %s, want %s", env.Type, TypeEvent)
	}
	if env.Method != EventStateChanged {
		t.Errorf("Method = %s", env.Method)
	}

	var decoded StateChangedEvent
	env.UnmarshalPayload(&decoded)
	if decoded.From.State != "running" {
		t.Errorf("From.State = %s", decoded.From.State)
	}
}

func TestMarshalUnmarshal(t *testing.T) {
	env, _ := NewRequest("test.ping", map[string]string{"key": "value"})

	data, err := Marshal(env)
	if err != nil {
		t.Fatalf("Marshal() error: %v", err)
	}

	parsed, err := Unmarshal(data)
	if err != nil {
		t.Fatalf("Unmarshal() error: %v", err)
	}

	if parsed.Method != env.Method {
		t.Errorf("Method mismatch after round-trip")
	}
	if parsed.RequestID != env.RequestID {
		t.Errorf("RequestID mismatch after round-trip")
	}
}

func TestUnmarshal_InvalidData(t *testing.T) {
	_, err := Unmarshal([]byte("not valid json"))
	if err == nil {
		t.Error("Unmarshal() expected error for invalid data")
	}
}

func TestUnmarshalPayload_EmptyPayload(t *testing.T) {
	env := &Envelope{}
	var v map[string]interface{}
	err := env.UnmarshalPayload(&v)
	if err != nil {
		t.Errorf("UnmarshalPayload() error on empty payload: %v", err)
	}
}

func TestMethodConstants(t *testing.T) {
	// Verify no collisions in method names
	methods := []string{
		MethodCoreStart, MethodCoreStop, MethodCoreRestart,
		MethodCoreStatus, MethodCoreHealth, MethodCoreSwapConfig,
		MethodOutboundList, MethodOutboundCreate, MethodOutboundUpdate,
		MethodOutboundDelete, MethodOutboundTest,
		MethodRuleList, MethodRuleCreate, MethodRuleUpdate,
		MethodRuleDelete, MethodRuleSimulate,
		MethodConfigCompile, MethodConfigApply, MethodConfigRollback,
		MethodConfigRevisions,
	}

	seen := make(map[string]bool)
	for _, m := range methods {
		if seen[m] {
			t.Errorf("duplicate method constant: %s", m)
		}
		seen[m] = true
	}

	// Event methods
	events := []string{
		EventStateChanged, EventMetricUpdate, EventIPChanged,
		EventCoreCrashed, EventRuleHit,
	}
	for _, e := range events {
		if seen[e] {
			t.Errorf("event/method collision: %s", e)
		}
	}
}

func TestMessageTypes(t *testing.T) {
	// Test all message structs can be marshaled/unmarshaled
	tests := []struct {
		name string
		v    interface{}
	}{
		{"CoreStartRequest", CoreStartRequest{ConfigPath: "/test"}},
		{"CoreStatusResponse", CoreStatusResponse{State: "running", PID: 12345}},
		{"CoreStopRequest", CoreStopRequest{Force: true, TimeoutSeconds: 5}},
		{"OutboundInfo", OutboundInfo{ID: "1", Name: "Test", Type: "socks5", Server: "1.2.3.4", Port: 1080}},
		{"OutboundListResponse", OutboundListResponse{Outbounds: []OutboundInfo{}}},
		{"RuleCreateRequest", RuleCreateRequest{Name: "test", Priority: 1, OutboundID: "direct"}},
		{"RuleSimulateRequest", RuleSimulateRequest{ProcessName: "chrome.exe", Domain: "google.com"}},
		{"ConfigCompileResponse", ConfigCompileResponse{ConfigHash: "abc", Validated: true}},
		{"MetricUpdateEvent", MetricUpdateEvent{OutboundID: "1", Latency: 42.5}},
		{"IPChangedEvent", IPChangedEvent{NewIP: "1.2.3.4", Country: "US"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.v)
			if err != nil {
				t.Errorf("Marshal() error: %v", err)
			}
			if len(data) == 0 {
				t.Error("Marshal() returned empty")
			}
		})
	}
}

func TestNewEnvelope_InvalidPayload(t *testing.T) {
	// Channel cannot be marshalled to JSON
	_, err := NewRequest("test", make(chan int))
	if err == nil {
		t.Error("NewRequest() expected error for unmarshallable payload")
	}
}

func TestRequestID_Uniqueness(t *testing.T) {
	ids := make(map[string]bool)
	for i := 0; i < 100; i++ {
		env, _ := NewRequest("test", nil)
		if ids[env.RequestID] {
			t.Errorf("duplicate request ID: %s", env.RequestID)
		}
		ids[env.RequestID] = true
	}
}
