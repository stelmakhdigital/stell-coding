package packages

import (
	"context"
	"os"
	"strings"
	"sync"

	"github.com/stelmakhdigital/stell-coding/internal/update"
)

// PackageUpdate описывает установленный пакет с доступной более новой версией.
type PackageUpdate struct {
	Name        string
	Source      string
	DisplayName string
}

// CheckForAvailableUpdates сканирует установленные пакеты на обновления.
func (m *Manager) CheckForAvailableUpdates(ctx context.Context) ([]PackageUpdate, error) {
	if update.Offline() {
		return nil, nil
	}
	recs, err := m.List()
	if err != nil {
		return nil, err
	}
	var out []PackageUpdate
	var mu sync.Mutex
	sem := make(chan struct{}, 4)
	var wg sync.WaitGroup
	for _, rec := range recs {
		rec := rec
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			u, ok := m.checkRecordUpdate(ctx, rec)
			if !ok {
				return
			}
			mu.Lock()
			out = append(out, u)
			mu.Unlock()
		}()
	}
	wg.Wait()
	return out, nil
}

func (m *Manager) checkRecordUpdate(ctx context.Context, rec Record) (PackageUpdate, bool) {
	src, err := ParseSource(rec.Source)
	if err != nil {
		return PackageUpdate{}, false
	}
	if src.Kind == "local" {
		return PackageUpdate{}, false
	}
	if src.Kind == "git" && src.Ref != "" {
		return PackageUpdate{}, false
	}
	switch src.Kind {
	case "git":
		if _, err := os.Stat(rec.InstallPath); err != nil {
			return PackageUpdate{}, false
		}
		has, err := gitHasAvailableUpdate(ctx, rec.InstallPath)
		if err != nil || !has {
			return PackageUpdate{}, false
		}
		display := rec.Name
		if u := gitDisplayName(src.Path); u != "" {
			display = u
		}
		return PackageUpdate{Name: rec.Name, Source: rec.Source, DisplayName: display}, true
	default:
		return PackageUpdate{}, false
	}
}

func gitDisplayName(url string) string {
	url = strings.TrimPrefix(url, "https://")
	url = strings.TrimPrefix(url, "http://")
	url = strings.TrimSuffix(url, ".git")
	return url
}

// CheckAllScopes возвращает обновления из global и project хранилищ пакетов.
func CheckAllScopes(ctx context.Context, globalDir, projectDir string) ([]PackageUpdate, error) {
	if update.Offline() {
		return nil, nil
	}
	seen := map[string]bool{}
	var out []PackageUpdate
	for _, scope := range []string{"global", "project"} {
		mgr := NewManager(globalDir, projectDir, scope)
		updates, err := mgr.CheckForAvailableUpdates(ctx)
		if err != nil {
			return nil, err
		}
		for _, u := range updates {
			if seen[u.Name] {
				continue
			}
			seen[u.Name] = true
			out = append(out, u)
		}
	}
	return out, nil
}
