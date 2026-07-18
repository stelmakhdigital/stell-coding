package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stelmakhdigital/stell-coding/internal/agent"
	"github.com/stelmakhdigital/stell-coding/internal/config"
	"github.com/stelmakhdigital/stell-ai/provider"
	"github.com/stelmakhdigital/stell-agent/session"
	"github.com/stelmakhdigital/stell-agent/tools"
)

func testModel(t *testing.T) (Model, *config.Config) {
	t.Helper()
	dir := t.TempDir()
	cfg := &config.Config{
		Settings:  config.DefaultSettings(),
		Models:    []config.ModelConfig{{Name: "mock", Provider: "mock"}},
		GlobalDir: dir,
		Workspace: dir,
	}
	reg := provider.NewRegistry()
	rt := tools.NewRuntime(tools.Env{Workspace: dir})
	_ = tools.RegisterBuiltins(rt)
	sess := session.NewManager(dir)
	svc := agent.NewService(cfg, reg, rt, sess, "", cfg.Models[0], nil, nil)
	m := NewModel(t.Context(), Options{Service: svc, Config: cfg})
	return m, cfg
}

func TestSubmitAllowsAttachmentsOnly(t *testing.T) {
	m, cfg := testModel(t)
	if err := writeTestPNG(filepath.Join(cfg.Workspace, "photo.png")); err != nil {
		t.Fatal(err)
	}
	m.attachments = []composerAttachment{newComposerAttachment("photo.png")}

	cmd := m.submitWithText("", false)
	if cmd == nil {
		t.Fatal("expected submit cmd with attachments-only")
	}
}

func TestAtGuardBareQueryOnly(t *testing.T) {
	m, cfg := testModel(t)
	if err := os.WriteFile(filepath.Join(cfg.Workspace, "main.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cmd := m.submitWithText("@main.go", false)
	if cmd != nil {
		t.Fatal("bare @query should open picker, not submit")
	}
	if m.overlayMode != overlayPicker {
		t.Fatalf("expected picker overlay, got %v", m.overlayMode)
	}

	m2, _ := testModel(t)
	if err := os.WriteFile(filepath.Join(cfg.Workspace, "main.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	m2.cfg.Workspace = cfg.Workspace
	m2.attachments = []composerAttachment{newComposerAttachment("main.go")}
	cmd2 := m2.submitWithText("explain this file", false)
	if cmd2 == nil {
		t.Fatal("expected submit with text and attachment")
	}
}

func TestInsertInlinePickerRemovesAtQuery(t *testing.T) {
	m, _ := testModel(t)
	m.composer.SetValue("please @mai")
	m.insertInlinePicker("main.go")
	if strings.Contains(m.composer.Value(), "@main.go") {
		t.Fatalf("composer should not contain @path, got %q", m.composer.Value())
	}
	if len(m.attachments) != 1 || m.attachments[0].Path != "main.go" {
		t.Fatalf("unexpected attachments: %+v", m.attachments)
	}
}

func TestImportImageToWorkspace(t *testing.T) {
	dir := t.TempDir()
	outside := t.TempDir()
	src := filepath.Join(outside, "photo.png")
	if err := writeTestPNG(src); err != nil {
		t.Fatal(err)
	}
	m := Model{cfg: &config.Config{Workspace: dir}}
	rel, err := m.importImageToWorkspace(src)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(rel, ".stell-attachment-photo.png") {
		t.Fatalf("unexpected rel %q", rel)
	}
	if _, err := os.Stat(filepath.Join(dir, rel)); err != nil {
		t.Fatal(err)
	}
}

func TestPrepareAttachmentsClearsStaleFiles(t *testing.T) {
	m, cfg := testModel(t)
	if err := os.WriteFile(filepath.Join(cfg.Workspace, "old.txt"), []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	m.svc.SetPendingAttachments([]string{"old.txt"})

	if err := writeTestPNG(filepath.Join(cfg.Workspace, "photo.png")); err != nil {
		t.Fatal(err)
	}
	m.attachments = []composerAttachment{newComposerAttachment("photo.png")}
	if err := m.prepareAttachments(); err != nil {
		t.Fatal(err)
	}

	m.attachments = []composerAttachment{newComposerAttachment("old.txt")}
	if err := m.prepareAttachments(); err != nil {
		t.Fatal(err)
	}
	imgPaths, filePaths := agent.SplitAttachments(m.attachmentPaths())
	if len(imgPaths) != 0 {
		t.Fatalf("expected no image paths after file-only prepare, got %v", imgPaths)
	}
	if len(filePaths) != 1 || filePaths[0] != "old.txt" {
		t.Fatalf("unexpected file paths: %v", filePaths)
	}
}

func writeTestPNG(path string) error {
	data := []byte{
		0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a,
		0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52,
		0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
		0x08, 0x06, 0x00, 0x00, 0x00, 0x1f, 0x15, 0xc4,
		0x89, 0x00, 0x00, 0x00, 0x0a, 0x49, 0x44, 0x41,
		0x54, 0x78, 0x9c, 0x63, 0x00, 0x01, 0x00, 0x00,
		0x05, 0x00, 0x01, 0x0d, 0x0a, 0x2d, 0xb4, 0x00,
		0x00, 0x00, 0x00, 0x49, 0x45, 0x4e, 0x44, 0xae,
		0x42, 0x60, 0x82,
	}
	return os.WriteFile(path, data, 0o644)
}
