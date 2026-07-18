package telemetry

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/stelmakhdigital/stell-coding/internal/config"
)

func isTruthy(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

// IsInstallTelemetryEnabled учитывает STELL_TELEMETRY, иначе — settings.
func IsInstallTelemetryEnabled(s config.Settings) bool {
	if v, ok := os.LookupEnv("STELL_TELEMETRY"); ok {
		return isTruthy(v)
	}
	return s.InstallTelemetryEnabled()
}

func reportURL() string {
	if u := strings.TrimSpace(os.Getenv("STELL_TELEMETRY_URL")); u != "" {
		return u
	}
	return "https://stell.dev/api/report-install"
}

func updateURL() string {
	if u := strings.TrimSpace(os.Getenv("STELL_UPDATE_URL")); u != "" {
		return u
	}
	return "https://stell.dev/api/latest-version"
}

// UpdateURL возвращает endpoint latest-version.
func UpdateURL() string {
	return updateURL()
}

func newTrackingID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

// EnsureTrackingID задаёт TrackingID, если analytics включена и ID пуст.
func EnsureTrackingID(s *config.Settings) bool {
	if s == nil || !s.AnalyticsEnabled() {
		return false
	}
	if s.TrackingID != "" {
		return false
	}
	s.TrackingID = newTrackingID()
	return true
}

// ReportInstall отправляет анонимный POST ping install/update.
func ReportInstall(ctx context.Context, version, trackingID string) error {
	payload := map[string]string{"version": version, "product": "stell"}
	if trackingID != "" {
		payload["trackingId"] = trackingID
	}
	b, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reportURL(), bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<16))
	return nil
}

// CheckLatestVersion получает строку последней опубликованной версии (GET).
func CheckLatestVersion(ctx context.Context) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, updateURL(), nil)
	if err != nil {
		return "", err
	}
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
	if err != nil {
		return "", err
	}
	var raw struct {
		Version string `json:"version"`
	}
	if json.Unmarshal(body, &raw) == nil && raw.Version != "" {
		return raw.Version, nil
	}
	return strings.TrimSpace(string(body)), nil
}
