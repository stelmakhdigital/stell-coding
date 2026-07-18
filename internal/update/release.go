package update

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"runtime"
	"strings"
	"time"

	"github.com/stelmakhdigital/stell-coding/internal/telemetry"
)

const defaultVersionCheckTimeout = 10 * time.Second

// LatestRelease — ответ API stell.dev latest-version.
type LatestRelease struct {
	Version    string `json:"version"`
	ModulePath string `json:"modulePath,omitempty"`
	Note       string `json:"note,omitempty"`
}

// CheckForNewRelease возвращает релиз, если доступна более новая версия.
func CheckForNewRelease(ctx context.Context, currentVersion string) (*LatestRelease, error) {
	if SkipVersionCheck() {
		return nil, nil
	}
	release, err := GetLatestRelease(ctx, currentVersion)
	if err != nil || release == nil {
		return nil, err
	}
	if IsNewer(release.Version, currentVersion) {
		return release, nil
	}
	return nil, nil
}

// GetLatestRelease загружает метаданные последнего релиза.
func GetLatestRelease(ctx context.Context, currentVersion string) (*LatestRelease, error) {
	if SkipVersionCheck() {
		return nil, nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, telemetry.UpdateURL(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", userAgent(currentVersion))

	client := &http.Client{Timeout: defaultVersionCheckTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("latest-version: HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
	if err != nil {
		return nil, err
	}
	var raw struct {
		Version    string `json:"version"`
		ModulePath string `json:"modulePath,omitempty"`
		Note       string `json:"note,omitempty"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, err
	}
	ver := strings.TrimSpace(raw.Version)
	if ver == "" {
		ver = strings.TrimSpace(string(body))
	}
	if ver == "" {
		return nil, fmt.Errorf("latest-version: empty version")
	}
	out := &LatestRelease{Version: ver}
	if mp := strings.TrimSpace(raw.ModulePath); mp != "" {
		out.ModulePath = mp
	}
	if note := strings.TrimSpace(raw.Note); note != "" {
		out.Note = note
	}
	return out, nil
}

func userAgent(currentVersion string) string {
	return fmt.Sprintf("stell/%s (%s; go/%s; %s)", currentVersion, runtime.GOOS, runtime.Version(), runtime.GOARCH)
}
