package tui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"stell/coding-agent/internal/extensions"
	tuilib "stell/tui"
)

const extActionPrefix = "ext:"

const (
	actionSubmit              = "submit"
	actionFollowUp            = "followUp"
	actionClear               = "clear"
	actionScrollUp            = "scrollUp"
	actionScrollDown          = "scrollDown"
	actionModelCycle          = "modelCycle"
	actionModelSelect         = "modelSelect"
	actionTreeOpen            = "treeOpen"
	actionThinkingCycle       = "thinkingCycle"
	actionSessionResume       = "sessionResume"
	actionSessionNew          = "sessionNew"
	actionMessageCopy         = "messageCopy"
	actionMessageDequeue      = "messageDequeue"
	actionPasteClipboard      = "pasteClipboard"
	actionThinkingToggle      = "thinkingToggle"
	actionExternalEditor      = "externalEditor"
	actionMarkdownPreview     = "markdownPreview"
	actionScopedModels        = "scopedModels"
	actionModelCycleBack      = "modelCycleBack"
	actionInterrupt           = "interrupt"
	actionDeleteForward       = "deleteForward"
	actionSuspend             = "suspend"
	actionSessionFork         = "sessionFork"
	actionTreeFilterCycle     = "treeFilterCycle"
	actionTreeFilterCycleBack = "treeFilterCycleBack"
	actionTreeEditLabel       = "treeEditLabel"
	actionTreeFold            = "treeFold"
	actionTreeUnfold          = "treeUnfold"
	actionSettingsOpen        = "settingsOpen"
	actionEditorYank          = "editorYank"
	actionEditorPageUp        = "editorPageUp"
	actionEditorPageDown      = "editorPageDown"
	actionCardFocus           = "cardFocus"
	actionCardUp              = "cardUp"
	actionCardDown            = "cardDown"
)

// Алиасы с пространством имён сопоставляются внутренним действиям.
// Перемещение в редакторе (tui.editor.move*) обрабатывается в редакторе, не через app keybindings.
var actionAliases = map[string]string{
	"tui.editor.submit":             actionSubmit,
	"tui.editor.followUp":           actionFollowUp,
	"tui.editor.abort":              actionInterrupt,
	"tui.viewport.scrollUp":         actionScrollUp,
	"tui.viewport.scrollDown":       actionScrollDown,
	"app.model.cycle":               actionModelCycle,
	"app.model.cycleForward":        actionModelCycle,
	"app.model.select":              actionModelSelect,
	"app.session.tree":              actionTreeOpen,
	"app.thinking.cycle":            actionThinkingCycle,
	"app.session.resume":            actionSessionResume,
	"app.session.new":               actionSessionNew,
	"app.message.copy":              actionMessageCopy,
	"app.message.dequeue":           actionMessageDequeue,
	"app.message.followUp":          actionFollowUp,
	"app.clipboard.pasteImage":      actionPasteClipboard,
	"app.thinking.toggle":           actionThinkingToggle,
	"toggleThinking":                actionThinkingToggle,
	"app.editor.external":           actionExternalEditor,
	"app.markdown.preview":          actionMarkdownPreview,
	"app.models.save":               actionScopedModels,
	"app.models.enableAll":          actionScopedModels,
	"app.model.cycleBackward":       actionModelCycleBack,
	"app.interrupt":                 actionInterrupt,
	"app.clear":                     actionClear,
	"app.exit":                      actionDeleteForward, // legacy alias
	"app.deleteForward":             actionDeleteForward,
	"exit":                          actionDeleteForward, // legacy binding id
	"app.suspend":                   actionSuspend,
	"app.session.fork":              actionSessionFork,
	"app.tree.filter.cycleForward":  actionTreeFilterCycle,
	"app.tree.filter.cycleBackward": actionTreeFilterCycleBack,
	"app.tree.editLabel":            actionTreeEditLabel,
	"app.tree.foldOrUp":             actionTreeFold,
	"app.tree.unfoldOrDown":         actionTreeUnfold,
	"app.settings.open":             actionSettingsOpen,
	"tui.editor.pageUp":             actionEditorPageUp,
	"tui.editor.pageDown":           actionEditorPageDown,
	"tui.editor.yank":               actionEditorYank,
	"tui.models.scoped":             actionScopedModels,
	"tui.models.providerToggle":     actionModelCycle,
	// Устаревший алиас из ранних сборок stell.
	"abort": actionClear,
}

type Keybindings struct {
	Bindings map[string]string `json:"bindings"`
	Rules    []struct {
		Key    string `json:"key"`
		Action string `json:"action"`
	} `json:"-"`
	// Map — библиотечный KeyMap (chord→action), синхронизирован с Bindings.
	Map *KeyMap `json:"-"`
}

func DefaultKeybindings() Keybindings {
	kb := Keybindings{Bindings: map[string]string{
		actionSubmit:         "enter",
		actionFollowUp:       "alt+enter",
		actionInterrupt:      "esc",
		actionClear:          "ctrl+c",
		actionDeleteForward:  "ctrl+d",
		actionSuspend:        "ctrl+z",
		actionScrollUp:       "alt+pgup",
		actionScrollDown:     "alt+pgdown",
		actionModelCycle:     "ctrl+p",
		actionModelCycleBack: "shift+ctrl+p",
		actionModelSelect:    "ctrl+l",
		// Дерево открывается через /tree (без аккорда по умолчанию — Ctrl+T переключает thinking).
		actionThinkingCycle:  "shift+tab",
		actionMessageCopy:    "ctrl+x",
		actionMessageDequeue: "alt+up",
		actionPasteClipboard: "ctrl+v,super+v,meta+v,shift+insert,alt+v",
		actionThinkingToggle: "ctrl+t",
		actionExternalEditor: "ctrl+g",
		actionMarkdownPreview: "ctrl+shift+v",
		actionScopedModels:   "ctrl+shift+m",
		actionTreeEditLabel:  "shift+l",
		actionTreeFold:       "ctrl+left,alt+left",
		actionTreeUnfold:     "ctrl+right,alt+right",
		actionEditorPageUp:   "pgup",
		actionEditorPageDown: "pgdown",
		actionEditorYank:     "ctrl+y",
		actionSettingsOpen:   "ctrl+,",
		actionCardFocus:      "tab",
		actionCardUp:         "up,ctrl+up",
		actionCardDown:       "down,ctrl+down",
	}}
	for action, keys := range overlayActionDefaults() {
		// Дефолты оверлеев живут в Bindings для /hotkeys и user JSON.
		// syncMap даёт app-действиям приоритет на общих аккордах; оверлеи используют overlayKeys.
		if _, exists := kb.Bindings[action]; !exists {
			kb.Bindings[action] = keys
		}
	}
	kb.syncMap()
	return kb
}

// syncMap пересобирает KeyMap из Bindings (action→keys инвертируется в key→action).
// Действия оверлеев (tree/session) биндятся первыми; app-действия перезаписывают общие
// аккорды (ctrl+d, ctrl+t, …). Оверлеи резолвят их через overlayKeys, не через Map.
func (k *Keybindings) syncMap() {
	if k == nil {
		return
	}
	m := NewKeyMap()
	overlay := overlayActionDefaults()
	bind := func(onlyOverlay bool) {
		for action, bound := range k.Bindings {
			_, isOverlay := overlay[action]
			if isOverlay != onlyOverlay {
				continue
			}
			for _, key := range splitBindingKeys(bound) {
				if key != "" {
					m.Bind(key, action)
				}
			}
		}
	}
	bind(true)
	bind(false)
	k.Map = m
}

func KeybindingsPath(globalDir string) string {
	return filepath.Join(globalDir, "keybindings.json")
}

func LoadKeybindings(globalDir string) Keybindings {
	def := DefaultKeybindings()
	path := KeybindingsPath(globalDir)
	data, err := os.ReadFile(path)
	if err != nil {
		return def
	}
	var raw struct {
		Bindings map[string]string `json:"bindings"`
		Rules    []struct {
			Key    string `json:"key"`
			Action string `json:"action"`
		} `json:"rules"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		var rules []struct {
			Key    string `json:"key"`
			Action string `json:"action"`
		}
		if err2 := json.Unmarshal(data, &rules); err2 == nil {
			raw.Rules = rules
		} else {
			var actionKeysFormat map[string]json.RawMessage
			if err3 := json.Unmarshal(data, &actionKeysFormat); err3 == nil {
				for action, keysRaw := range actionKeysFormat {
					var keys []string
					if err4 := json.Unmarshal(keysRaw, &keys); err4 == nil && len(keys) > 0 {
						def.Bindings[normalizeAction(action)] = keys[0]
					} else {
						var one string
						if err5 := json.Unmarshal(keysRaw, &one); err5 == nil {
							def.Bindings[normalizeAction(action)] = one
						}
					}
				}
				def.syncMap()
				return def
			}
			return def
		}
	}
	for k, v := range raw.Bindings {
		def.Bindings[normalizeAction(k)] = v
	}
	for _, r := range raw.Rules {
		if r.Key != "" && r.Action != "" {
			def.Bindings[normalizeAction(r.Action)] = r.Key
		}
	}
	def.syncMap()
	return def
}

func LoadKeybindingsWithExtensions(globalDir string, ext *extensions.Supervisor) Keybindings {
	kb := LoadKeybindings(globalDir)
	if ext != nil {
		kb.MergeExtensionShortcuts(ext.Shortcuts())
	}
	return kb
}

// LoadUserKeyOverrides возвращает сырую карту id→keys из keybindings.json (формат id→keys + bindings).
func LoadUserKeyOverrides(globalDir string) map[string]string {
	path := KeybindingsPath(globalDir)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	out := map[string]string{}
	var actionKeysFormat map[string]json.RawMessage
	if err := json.Unmarshal(data, &actionKeysFormat); err == nil {
		for action, keysRaw := range actionKeysFormat {
			if action == "bindings" || action == "rules" {
				continue
			}
			var keys []string
			if err := json.Unmarshal(keysRaw, &keys); err == nil && len(keys) > 0 {
				out[action] = strings.Join(keys, ",")
				continue
			}
			var one string
			if err := json.Unmarshal(keysRaw, &one); err == nil {
				out[action] = one
			}
		}
	}
	var wrapped struct {
		Bindings map[string]string `json:"bindings"`
	}
	if err := json.Unmarshal(data, &wrapped); err == nil {
		for k, v := range wrapped.Bindings {
			out[k] = v
		}
	}
	return out
}

// SaveKeybindings пишет текущие bindings в JSON-формате id→keys[].
// Неизвестные user id из предыдущего файла сохраняются при mergeExtras=true.
func SaveKeybindings(globalDir string, kb Keybindings, mergeExtras bool) error {
	path := KeybindingsPath(globalDir)
	out := map[string]any{}
	if mergeExtras {
		if prev := LoadUserKeyOverrides(globalDir); prev != nil {
			for id, keys := range prev {
				out[id] = splitBindingKeys(keys)
			}
		}
	}
	for action, bound := range kb.Bindings {
		keys := splitBindingKeys(bound)
		if len(keys) == 0 {
			continue
		}
		// Предпочитаем namespaced id для известных алиасов.
		id := action
		for alias, a := range actionAliases {
			if a == action && (strings.HasPrefix(alias, "app.") || strings.HasPrefix(alias, "tui.")) {
				id = alias
				break
			}
		}
		out[id] = keys
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append(data, '\n'), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func ExtAction(action string) string {
	return extActionPrefix + action
}

func IsExtAction(action string) bool {
	return strings.HasPrefix(action, extActionPrefix)
}

func ExtActionName(action string) string {
	return strings.TrimPrefix(action, extActionPrefix)
}

// MergeExtensionShortcuts регистрирует keybindings расширений как записи ext:<action>.
func (k *Keybindings) MergeExtensionShortcuts(shortcuts []extensions.ShortcutDef) {
	for action := range k.Bindings {
		if strings.HasPrefix(action, extActionPrefix) {
			delete(k.Bindings, action)
		}
	}
	for _, sc := range shortcuts {
		if sc.Key == "" || sc.Action == "" {
			continue
		}
		k.Bindings[ExtAction(sc.Action)] = sc.Key
	}
	k.syncMap()
}

func normalizeAction(action string) string {
	if a, ok := actionAliases[action]; ok {
		return a
	}
	return action
}

func (k Keybindings) Matches(action, keyStr string) bool {
	b, ok := k.Bindings[action]
	if !ok {
		return false
	}
	keyStr = tuilib.NormalizeKeyChord(keyStr)
	for _, key := range splitBindingKeys(b) {
		if tuilib.NormalizeKeyChord(key) == keyStr {
			return true
		}
	}
	return false
}

func (k Keybindings) ActionForKey(keyStr string) (string, bool) {
	keyStr = tuilib.NormalizeKeyChord(keyStr)
	if k.Map != nil {
		if a, ok := k.Map.Lookup(keyStr); ok {
			return a, true
		}
	}
	for action, bound := range k.Bindings {
		for _, key := range splitBindingKeys(bound) {
			if tuilib.NormalizeKeyChord(key) == keyStr {
				return action, true
			}
		}
	}
	return "", false
}

func (k Keybindings) Help(action string) []string {
	return splitBindingKeys(k.Bindings[action])
}

func (k Keybindings) boundKeys() map[string]bool {
	out := make(map[string]bool)
	for _, v := range k.Bindings {
		for _, key := range splitBindingKeys(v) {
			if key != "" {
				out[key] = true
			}
		}
	}
	return out
}

func splitBindingKeys(binding string) []string {
	parts := strings.Split(binding, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func isPasteKey(keys Keybindings, keyStr string) bool {
	action, ok := keys.ActionForKey(keyStr)
	return ok && action == actionPasteClipboard
}

func isBoundKey(keys Keybindings, keyStr string) bool {
	return keys.boundKeys()[keyStr]
}

func isEditorKey(keyStr string) bool {
	switch keyStr {
	case "enter", "alt+enter", "shift+enter", "ctrl+j", "tab", "shift+tab":
		return true
	}
	return strings.HasPrefix(keyStr, "ctrl+") || strings.HasPrefix(keyStr, "alt+")
}
