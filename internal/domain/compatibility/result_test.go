package compatibility

import "testing"

func TestResultInvariants(t *testing.T) {
	t.Parallel()

	if !Supported().Valid() {
		t.Fatal("Supported result must be valid")
	}
	if !Limited(Warning{Code: "udp_limited", Message: "UDP is unavailable"}).Valid() {
		t.Fatal("Limited result with warning must be valid")
	}
	if !Unsupported(Reason{Code: "protocol_unsupported", Message: "unsupported protocol"}).Valid() {
		t.Fatal("Unsupported result with reason must be valid")
	}
	if (Result{Supported: true, Level: LevelUnsupported}).Valid() {
		t.Fatal("contradictory result must be invalid")
	}
}
