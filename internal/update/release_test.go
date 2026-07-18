package update

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetLatestRelease(t *testing.T) {
	t.Setenv("STELL_OFFLINE", "")
	t.Setenv("STELL_SKIP_VERSION_CHECK", "")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{
			"version":    "1.2.3",
			"modulePath": "example.com/stell/cmd",
		})
	}))
	defer srv.Close()
	t.Setenv("STELL_UPDATE_URL", srv.URL)
	t.Setenv("STELL_SKIP_VERSION_CHECK", "")

	release, err := GetLatestRelease(context.Background(), "1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	if release == nil {
		t.Fatal("expected release")
	}
	if release.Version != "1.2.3" || release.ModulePath != "example.com/stell/cmd" {
		t.Fatalf("release: %+v", release)
	}
}

func TestCheckForNewReleaseSkipsWhenCurrent(t *testing.T) {
	t.Setenv("STELL_OFFLINE", "")
	t.Setenv("STELL_SKIP_VERSION_CHECK", "")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"version": "1.0.0"})
	}))
	defer srv.Close()
	t.Setenv("STELL_UPDATE_URL", srv.URL)
	t.Setenv("STELL_SKIP_VERSION_CHECK", "")

	release, err := CheckForNewRelease(context.Background(), "1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	if release != nil {
		t.Fatalf("expected nil, got %+v", release)
	}
}
