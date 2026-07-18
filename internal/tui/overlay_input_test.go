package tui

import (
	"strings"
	"testing"

	"github.com/stelmakhdigital/stell-coding/internal/config"
)

func TestOverlayActive(t *testing.T) {
	m := &Model{}
	if m.overlayActive() {
		t.Fatal("empty model should not be overlay-active")
	}

	m.overlayMode = overlayInlineAt
	m.overlay = ""
	if !m.overlayActive() {
		t.Fatal("inline @-picker with overlayMode=overlayInlineAt should be active")
	}

	m = &Model{overlay: "text"}
	if !m.overlayActive() {
		t.Fatal("non-empty overlay string should be active")
	}

	list := NewSelectList([]string{"a"}, nil)
	m = &Model{overlayComp: list}
	if !m.overlayActive() {
		t.Fatal("overlayComp should make overlay active")
	}

	m = &Model{overlayMode: overlayModel, overlayComp: list}
	if !m.overlayActive() {
		t.Fatal("overlayModel with comp should be active")
	}
}

func TestDismissPopups(t *testing.T) {
	m := &Model{
		slashMenu:    &slashMenuState{items: []slashCommand{{name: "/model"}}},
		autocomplete: &acState{items: []acItem{{value: "/model"}}},
	}
	m.dismissPopups()
	if m.slashMenu != nil || m.autocomplete != nil {
		t.Fatal("dismissPopups should clear slash menu and autocomplete")
	}
}

func TestPushOverlayDismissesPopups(t *testing.T) {
	m := &Model{
		slashMenu:    &slashMenuState{items: []slashCommand{{name: "/model"}}},
		autocomplete: &acState{items: []acItem{{value: "/model"}}},
	}
	m.pushOverlayFrame(overlayFrame{mode: overlayTree, text: "tree"})
	if m.slashMenu != nil || m.autocomplete != nil {
		t.Fatal("pushOverlayFrame should dismiss popups")
	}
}

func TestCloseOverlayDismissesPopups(t *testing.T) {
	m := &Model{
		overlayMode: overlayTree,
		overlay:     "tree",
		slashMenu:   &slashMenuState{items: []slashCommand{{name: "/help"}}},
	}
	m.closeOverlay()
	if m.overlayActive() {
		t.Fatal("overlay should be closed")
	}
	if m.slashMenu != nil {
		t.Fatal("closeOverlay should dismiss slash menu")
	}
}

func TestEscClosesModelOverlayAfterSlashModel(t *testing.T) {
	m, _ := testModel(t)
	m.width, m.height = 100, 40

	m.composer.SetValue("/model")
	m.updateSlashMenu()
	if m.slashMenu == nil {
		t.Fatal("expected slash menu before opening model overlay")
	}

	m.handleSlash("/model")
	if !m.overlayActive() {
		t.Fatal("expected model overlay open")
	}
	if m.slashMenu != nil {
		t.Fatal("slash menu should be dismissed when overlay opens")
	}

	updated, _ := m.Update(KeyMsg{Type: KeyEsc, raw: "esc"})
	m2 := updated
	if m2.overlayActive() {
		t.Fatal("Esc should close model overlay")
	}
	if m2.slashMenu != nil {
		t.Fatal("slash menu should stay dismissed after Esc")
	}
	if m2.autocomplete != nil {
		t.Fatal("autocomplete should stay nil after Esc")
	}
}

func TestEscDismissesAutocompletePopup(t *testing.T) {
	m, _ := testModel(t)
	m.composer.SetValue("/mod")
	m.updateAutocomplete()
	if m.autocomplete == nil {
		t.Fatal("expected autocomplete for /mod")
	}

	updated, _ := m.Update(KeyMsg{Type: KeyEsc, raw: "esc"})
	m2 := updated
	if m2.autocomplete != nil {
		t.Fatal("Esc should dismiss autocomplete")
	}
	if m2.slashMenu != nil {
		t.Fatal("Esc should dismiss slash menu synced with autocomplete")
	}
}

func TestEscClosesScopedModelsOverlay(t *testing.T) {
	m, cfg := testModel(t)
	cfg.Models = []config.ModelConfig{
		{Name: "alpha", Provider: "mock"},
		{Name: "beta", Provider: "mock"},
	}
	m.cfg = cfg

	m.openScopedModelsOverlay()
	if !m.overlayActive() {
		t.Fatal("expected scoped models overlay open")
	}

	updated, _ := m.Update(KeyMsg{Type: KeyEsc, raw: "esc"})
	m2 := updated
	if m2.overlayActive() {
		t.Fatal("Esc should close scoped models overlay")
	}
}

func TestSelectListEscClearsFilterBeforeClose(t *testing.T) {
	m, _ := testModel(t)
	m.openModelOverlay()
	if !m.overlayActive() {
		t.Fatal("expected model overlay")
	}

	updated, _ := m.Update(KeyMsg{Type: KeyRunes, Runes: []rune{'m'}})
	m2 := updated
	sl, ok := m2.overlayComp.(*SelectList)
	if !ok || sl.Query == "" {
		t.Fatal("expected filter query after typing in model overlay")
	}

	updated2, _ := m2.Update(KeyMsg{Type: KeyEsc, raw: "esc"})
	m3 := updated2
	if !m3.overlayActive() {
		t.Fatal("first Esc should clear filter, not close overlay")
	}
	sl2, ok := m3.overlayComp.(*SelectList)
	if !ok || sl2.Query != "" {
		t.Fatal("first Esc should clear filter query")
	}

	updated3, _ := m3.Update(KeyMsg{Type: KeyEsc, raw: "esc"})
	m4 := updated3
	if m4.overlayActive() {
		t.Fatal("second Esc should close overlay")
	}
}

func TestEscClosesModelOverlayWithModifyOtherKeys(t *testing.T) {
	m, _ := testModel(t)
	m.width, m.height = 100, 40
	m.handleSlash("/model")
	if !m.overlayActive() {
		t.Fatal("expected model overlay open")
	}
	updated, _ := m.Update(parseKey("\x1b[27;1;27~"))
	m2 := updated
	if m2.overlayActive() {
		t.Fatal("modifyOtherKeys Esc should close model overlay")
	}
	if m2.slashMenu != nil {
		t.Fatal("slash menu should stay dismissed")
	}
}

func TestEscClosesSettingsOverlayAfterSlashSettings(t *testing.T) {
	m, _ := testModel(t)
	m.width, m.height = 100, 40

	m.composer.SetValue("/settings")
	m.updateSlashMenu()
	if m.slashMenu == nil {
		t.Fatal("expected slash menu before opening settings overlay")
	}

	m.handleSlash("/settings")
	if !m.overlayActive() {
		t.Fatal("expected settings overlay open")
	}
	if m.overlayMode != overlaySettings {
		t.Fatalf("overlayMode=%v want overlaySettings", m.overlayMode)
	}
	if m.slashMenu != nil {
		t.Fatal("slash menu should be dismissed when overlay opens")
	}

	updated, _ := m.Update(KeyMsg{Type: KeyEsc, raw: "esc"})
	m2 := updated
	if m2.overlayActive() {
		t.Fatal("Esc should close settings overlay")
	}
	if m2.slashMenu != nil {
		t.Fatal("slash menu should stay dismissed after Esc")
	}
}

func TestEnterSelectsModelAndClosesOverlay(t *testing.T) {
	m, cfg := testModel(t)
	cfg.Models = []config.ModelConfig{
		{Name: "alpha", Provider: "mock"},
		{Name: "beta", Provider: "mock"},
	}
	m.cfg = cfg
	// Avoid ReloadModels overwriting test models from workspace discovery.
	m.cfg.Workspace = t.TempDir()

	m.openModelOverlay()
	if !m.overlayActive() {
		t.Fatal("expected model overlay open")
	}
	sl, ok := m.overlayComp.(*SelectList)
	if !ok || len(sl.Items) < 2 {
		t.Fatalf("expected SelectList with >=2 items, got %v", m.overlayComp)
	}

	updated, _ := m.Update(KeyMsg{Type: KeyDown, raw: "down"})
	m2 := updated
	updated2, _ := m2.Update(KeyMsg{Type: KeyEnter, raw: "enter"})
	m3 := updated2
	if m3.overlayActive() {
		t.Fatal("Enter should close model overlay after select")
	}
	want := strings.Split(sl.Items[1], " [")[0]
	if got := m3.svc.GetState().ModelName; got != want {
		t.Fatalf("model=%q want %q", got, want)
	}
}

func TestAtPickerSinglePopupNoFullOverlay(t *testing.T) {
	m, _ := testModel(t)
	m.width, m.height = 80, 40
	m.showStartup = false
	m.fileIndex = []string{
		"main.go", "cmd/main.go", "internal/foo.go", "internal/bar.go",
		"pkg/a.go", "pkg/b.go", "README.md", "go.mod",
	}
	m.composer.SetValue("@")
	m.updateAutocomplete()
	m.resizeViewport()

	if m.overlayMode != overlayInlineAt {
		t.Fatalf("overlayMode=%v want overlayInlineAt", m.overlayMode)
	}
	if m.overlay != "" {
		t.Fatalf("inline @ picker must keep overlay empty, got %q", m.overlay)
	}
	if popup := m.autocompletePopup(); popup != "" {
		t.Fatalf("acFile must not render autocompletePopup, got %q", popup)
	}
	if popup := m.composerPopup(); !strings.Contains(popup, "@ picker") {
		t.Fatalf("expected @ picker popup, got %q", popup)
	}

	updated, _ := m.Update(KeyMsg{Type: KeyDown, raw: "down"})
	m2 := updated
	view := stripANSIForTest(m2.View())
	if strings.Count(view, "@ picker") != 1 {
		t.Fatalf("want exactly one @ picker header, view:\n%s", view)
	}
	if strings.Contains(view, "(esc to close)") {
		t.Fatalf("down must not open full overlay with (esc to close), view:\n%s", view)
	}
	if strings.Contains(view, "files (↑/↓ enter)") {
		t.Fatalf("must not show duplicate files autocomplete popup, view:\n%s", view)
	}
	if strings.Contains(view, "file picker (") {
		t.Fatalf("must not materialize full file picker overlay, view:\n%s", view)
	}
	if m2.overlay != "" {
		t.Fatalf("after down overlay should stay empty, got %q", m2.overlay)
	}
	if m2.overlayCursor != 1 {
		t.Fatalf("overlayCursor=%d want 1 after down", m2.overlayCursor)
	}
}
