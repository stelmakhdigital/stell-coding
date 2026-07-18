package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"stell/agent/hooks"
	"stell/agent/session"
	"stell/agent/tools"
)

// OpenSession загружает файл сессии в service.
func (s *Service) OpenSession(path string) error {
	if s.IsStreaming() {
		return ErrStreaming
	}
	m, err := session.Open(path)
	if err != nil {
		return err
	}
	s.mu.Lock()
	s.Sessions = m
	s.SessPath = path
	s.mu.Unlock()
	return nil
}

// ForkSession ветвит сессию в entryID внутри текущего файла.
func (s *Service) ForkSession(entryID string) (string, error) {
	if s.IsStreaming() {
		return "", ErrStreaming
	}
	if s.Hooks != nil {
		ev, err := s.Hooks.EmitNamed(context.Background(), hooks.SessionBeforeFork, s.Sessions.Header.ID, map[string]any{
			"entryId": entryID,
		})
		if err != nil {
			return "", err
		}
		if ev.Cancel {
			return "", fmt.Errorf("fork cancelled by extension")
		}
	}
	_ = s.MaybeBranchSummary(context.Background())
	if err := s.Sessions.ForkAt(entryID); err != nil {
		return "", err
	}
	if s.SessPath != "" {
		if err := s.Sessions.Save(s.SessPath); err != nil {
			return "", err
		}
	}
	return s.Sessions.LeafID(), nil
}

// CloneSession клонирует текущую сессию в новый файл.
func (s *Service) CloneSession() (string, error) {
	if s.IsStreaming() {
		return "", ErrStreaming
	}
	clone := s.Sessions.Clone()
	path := session.NewSessionPath(s.Config.SessionsRoot(), s.Config.Workspace)
	if err := clone.Save(path); err != nil {
		return "", err
	}
	s.mu.Lock()
	s.Sessions = clone
	s.Sessions.Path = path
	s.SessPath = path
	s.mu.Unlock()
	return path, nil
}

// ListSessions перечисляет файлы сессий workspace.
func (s *Service) ListSessions() ([]session.FileInfo, error) {
	return session.ListFiles(s.Config.SessionsRoot(), s.Config.Workspace)
}

// ExportSession записывает сессию в dest (должен быть внутри workspace).
func (s *Service) ExportSession(dest string) error {
	abs, _, err := tools.ResolveOutputPath(s.Config.Workspace, dest, "session-export.jsonl")
	if err != nil {
		return err
	}
	return s.Sessions.Export(abs)
}

// ResumeLatest открывает самый свежий файл сессии workspace.
func (s *Service) ResumeLatest() (string, error) {
	files, err := s.ListSessions()
	if err != nil {
		return "", err
	}
	if len(files) == 0 {
		return "", fmt.Errorf("no sessions found")
	}
	if err := s.OpenSession(files[0].Path); err != nil {
		return "", err
	}
	return files[0].Path, nil
}

// ContinueSession открывает последнюю сессию или создаёт новую, если сессий нет.
func (s *Service) ContinueSession() (string, error) {
	files, err := s.ListSessions()
	if err != nil {
		return "", err
	}
	if len(files) == 0 {
		ws := s.Config.Workspace
		s.mu.Lock()
		s.Sessions = session.NewManager(ws)
		s.SessPath = session.NewSessionPath(s.Config.SessionsRoot(), ws)
		path := s.SessPath
		s.mu.Unlock()
		return path, nil
	}
	if err := s.OpenSession(files[0].Path); err != nil {
		return "", err
	}
	return files[0].Path, nil
}

// CycleModel переходит к следующей настроенной модели.
func (s *Service) CycleModel() (string, error) {
	models := s.Config.Models
	if len(models) == 0 {
		return "", fmt.Errorf("no models configured")
	}
	s.mu.Lock()
	idx := 0
	for i, m := range models {
		if m.Name == s.Model.Name {
			idx = (i + 1) % len(models)
			break
		}
	}
	s.Model = models[idx]
	name := s.Model.Name
	s.mu.Unlock()
	s.SetModelRecord(s.Model)
	s.emitAgentHook(context.Background(), hooks.ModelSelect, map[string]any{
		"model": name, "sessionId": s.Sessions.Header.ID,
	})
	return name, nil
}

// SwitchToEntry переходит к записи без ветвления.
func (s *Service) SwitchToEntry(ctx context.Context, entryID string) error {
	if s.IsStreaming() {
		return ErrStreaming
	}
	_ = s.MaybeBranchSummary(ctx)
	if err := s.Sessions.SwitchToEntry(entryID); err != nil {
		return err
	}
	if s.SessPath != "" {
		return s.Sessions.Save(s.SessPath)
	}
	return nil
}

// AbortBash отменяет выполняющуюся bash-команду, запущенную через RunBash.
func (s *Service) AbortBash() {
	s.bashMu.Lock()
	cancel := s.bashCancel
	s.bashMu.Unlock()
	if cancel != nil {
		cancel()
	}
}

// DeleteSession удаляет файл сессии с диска.
func (s *Service) DeleteSession(path string) error {
	if s.IsStreaming() && s.SessPath == path {
		return fmt.Errorf("cannot delete active session while streaming")
	}
	if err := os.Remove(path); err != nil {
		return err
	}
	if s.SessPath == path {
		ws := s.Config.Workspace
		s.mu.Lock()
		s.Sessions = session.NewManager(ws)
		s.SessPath = session.NewSessionPath(s.Config.SessionsRoot(), ws)
		s.mu.Unlock()
	}
	return nil
}

func (s *Service) saveSessionIfNeeded() {
	if s.SessPath == "" || s.Sessions == nil {
		return
	}
	reportSessionSaveError(s.SessPath, s.Sessions.Save(s.SessPath))
}

// AppendSessionInfo записывает entry session_info.
func (s *Service) AppendSessionInfo(text string) (string, error) {
	id, err := s.Sessions.AppendSessionInfo(text)
	if err != nil {
		return "", err
	}
	s.saveSessionIfNeeded()
	return id, nil
}

// AppendCustomEntry записывает custom entry.
func (s *Service) AppendCustomEntry(text string) (string, error) {
	id, err := s.Sessions.AppendCustomEntry(text)
	if err != nil {
		return "", err
	}
	s.saveSessionIfNeeded()
	return id, nil
}

// AppendCustomMessage записывает entry custom_message (RPC append_custom_message).
func (s *Service) AppendCustomMessage(text string) (string, error) {
	id, err := s.Sessions.AppendCustomMessage(text)
	if err != nil {
		return "", err
	}
	s.saveSessionIfNeeded()
	return id, nil
}

func (s *Service) AppendTypedCustomMessage(customType, text string, data json.RawMessage) (string, error) {
	id, err := s.Sessions.AppendTypedCustomMessage(customType, text, data)
	if err != nil {
		return "", err
	}
	s.saveSessionIfNeeded()
	return id, nil
}

func (s *Service) AppendTypedCustomEntry(customType, text string, data json.RawMessage) (string, error) {
	id, err := s.Sessions.AppendTypedCustomEntry(customType, text, data)
	if err != nil {
		return "", err
	}
	s.saveSessionIfNeeded()
	return id, nil
}

// AppendLabel записывает label entry.
func (s *Service) AppendLabel(text string) (string, error) {
	id, err := s.Sessions.AppendLabel(text)
	if err != nil {
		return "", err
	}
	s.saveSessionIfNeeded()
	return id, nil
}
