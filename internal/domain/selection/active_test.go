package selection

import (
	"errors"
	"testing"

	"navo/internal/domain/apperror"
	"navo/internal/domain/capture"
	"navo/internal/domain/core"
	"navo/internal/domain/source"
)

func TestActiveSelectionValidate(t *testing.T) {
	t.Parallel()

	subscriptionID := "subscription-1"
	endpointID := "endpoint-1"
	upstreamID := "upstream-1"

	tests := []struct {
		name      string
		selection ActiveSelection
		wantErr   bool
	}{
		{
			name: "airport",
			selection: ActiveSelection{
				CoreType: core.TypeSingBox, SourceType: source.TypeAirportSubscription,
				CaptureMode: capture.ModeSystemProxy, SubscriptionID: &subscriptionID, EndpointID: &endpointID,
			},
		},
		{
			name: "upstream",
			selection: ActiveSelection{
				CoreType: core.TypeXray, SourceType: source.TypeUpstreamProxy,
				CaptureMode: capture.ModeOff, UpstreamProxyID: &upstreamID,
			},
		},
		{
			name: "airport cannot contain upstream",
			selection: ActiveSelection{
				CoreType: core.TypeMihomo, SourceType: source.TypeAirportSubscription,
				CaptureMode: capture.ModeTUN, SubscriptionID: &subscriptionID,
				EndpointID: &endpointID, UpstreamProxyID: &upstreamID,
			},
			wantErr: true,
		},
		{
			name: "upstream cannot contain airport ids",
			selection: ActiveSelection{
				CoreType: core.TypeXray, SourceType: source.TypeUpstreamProxy,
				CaptureMode: capture.ModeSystemProxy, SubscriptionID: &subscriptionID,
				EndpointID: &endpointID, UpstreamProxyID: &upstreamID,
			},
			wantErr: true,
		},
		{
			name: "unknown dimensions",
			selection: ActiveSelection{
				CoreType: "other", SourceType: "mixed", CaptureMode: "magic",
			},
			wantErr: true,
		},
		{
			name: "blank identifiers",
			selection: ActiveSelection{
				CoreType: core.TypeSingBox, SourceType: source.TypeAirportSubscription,
				CaptureMode: capture.ModeOff, SubscriptionID: ptr(" "), EndpointID: ptr(""),
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.selection.Validate()
			if tt.wantErr && err == nil {
				t.Fatal("Validate() error = nil, want error")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("Validate() error = %v", err)
			}
			if err != nil {
				var validationErrors apperror.ValidationErrors
				if !errors.As(err, &validationErrors) {
					t.Fatalf("Validate() error type = %T, want apperror.ValidationErrors", err)
				}
			}
		})
	}
}

func ptr(value string) *string {
	return &value
}
