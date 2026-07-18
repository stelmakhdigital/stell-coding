package telemetry

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stelmakhdigital/stell-coding/internal/config"
)

func TestIsInstallTelemetryEnvOverride(t *testing.T) {
	t.Setenv("STELL_TELEMETRY", "0")
	if IsInstallTelemetryEnabled(config.Settings{}) {
		t.Fatal("expected disabled via env")
	}
	t.Setenv("STELL_TELEMETRY", "1")
	off := false
	if !IsInstallTelemetryEnabled(config.Settings{EnableInstallTelemetry: &off}) {
		t.Fatal("env should override settings")
	}
}

func TestReportInstall(t *testing.T) {
	var got map[string]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&got)
		w.WriteHeader(204)
	}))
	defer srv.Close()
	t.Setenv("STELL_TELEMETRY_URL", srv.URL)
	if err := ReportInstall(context.Background(), "0.1.0", "abc"); err != nil {
		t.Fatal(err)
	}
	if got["version"] != "0.1.0" || got["trackingId"] != "abc" {
		t.Fatalf("payload=%v", got)
	}
}

func TestEnsureTrackingID(t *testing.T) {
	on := true
	s := config.Settings{EnableAnalytics: &on}
	if !EnsureTrackingID(&s) || s.TrackingID == "" {
		t.Fatal("expected new tracking id")
	}
	prev := s.TrackingID
	if EnsureTrackingID(&s) || s.TrackingID != prev {
		t.Fatal("should not regenerate")
	}
}
