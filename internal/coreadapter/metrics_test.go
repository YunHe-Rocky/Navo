package coreadapter

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClashMetricsReader(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if got := request.Header.Get("Authorization"); got != "Bearer secret" {
			t.Errorf("Authorization = %q", got)
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"uploadTotal":1024,"downloadTotal":4096,"connections":[{},{}]}`))
	}))
	defer server.Close()

	reader := newClashMetricsReader(RuntimeInfo{
		ControllerURL: server.URL, ControllerSecret: "secret",
	})
	got, err := reader.Read(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got.UploadBytes != 1024 || got.DownloadBytes != 4096 || got.Connections != 2 {
		t.Fatalf("metrics = %#v", got)
	}
}
