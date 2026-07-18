package themes

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// RequiredTokens — обязательные ключи цветов темы (51 required; thinkingMax опционален).
var RequiredTokens = []string{
	"accent", "border", "borderAccent", "borderMuted", "success", "error", "warning",
	"muted", "dim", "text", "thinkingText",
	"selectedBg", "userMessageBg", "userMessageText", "customMessageBg", "customMessageText",
	"customMessageLabel", "toolPendingBg", "toolSuccessBg", "toolErrorBg", "toolTitle", "toolOutput",
	"mdHeading", "mdLink", "mdLinkUrl", "mdCode", "mdCodeBlock", "mdCodeBlockBorder",
	"mdQuote", "mdQuoteBorder", "mdHr", "mdListBullet",
	"toolDiffAdded", "toolDiffRemoved", "toolDiffContext",
	"syntaxComment", "syntaxKeyword", "syntaxFunction", "syntaxVariable", "syntaxString",
	"syntaxNumber", "syntaxType", "syntaxOperator", "syntaxPunctuation",
	"thinkingOff", "thinkingMinimal", "thinkingLow", "thinkingMedium", "thinkingHigh", "thinkingXhigh",
	"bashMode",
}

type ExportColors struct {
	PageBg string `json:"pageBg,omitempty"`
	CardBg string `json:"cardBg,omitempty"`
	InfoBg string `json:"infoBg,omitempty"`
}

type Theme struct {
	Name    string            `json:"name"`
	Vars    map[string]any    `json:"vars,omitempty"`
	Colors  map[string]string `json:"colors"`
	Export  *ExportColors     `json:"export,omitempty"`
	path    string            `json:"-"`
}

func Load(path string) (*Theme, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var t Theme
	if err := json.Unmarshal(data, &t); err != nil {
		return nil, err
	}
	if t.Name == "" {
		t.Name = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	}
	t.path = path
	if err := t.Resolve(); err != nil {
		return nil, err
	}
	return &t, nil
}

func LoadDir(dir string) ([]Theme, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []Theme
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		t, err := Load(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		out = append(out, *t)
	}
	return out, nil
}

// Resolve раскрывает var-ссылки и заполняет thinkingMax из thinkingXhigh.
func (t *Theme) Resolve() error {
	if t.Colors == nil {
		t.Colors = map[string]string{}
	}
	vars := map[string]string{}
	for k, v := range t.Vars {
		switch x := v.(type) {
		case string:
			vars[k] = x
		case float64:
			vars[k] = fmt.Sprintf("%d", int(x))
		case int:
			vars[k] = fmt.Sprintf("%d", x)
		}
	}
	resolve := func(v string) string {
		if v == "" {
			return v
		}
		if r, ok := vars[v]; ok {
			return r
		}
		return v
	}
	for k, v := range t.Colors {
		t.Colors[k] = resolve(v)
	}
	if _, ok := t.Colors["thinkingMax"]; !ok {
		if x, ok := t.Colors["thinkingXhigh"]; ok {
			t.Colors["thinkingMax"] = x
		}
	}
	return nil
}

func (t *Theme) Validate() error {
	missing := make([]string, 0)
	for _, k := range RequiredTokens {
		if _, ok := t.Colors[k]; !ok {
			missing = append(missing, k)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("theme %q missing tokens: %s", t.Name, strings.Join(missing, ", "))
	}
	return nil
}

func (t Theme) Color(key, fallback string) string {
	if v, ok := t.Colors[key]; ok {
		return v
	}
	return fallback
}

func (t Theme) Path() string { return t.path }

func DefaultTheme() Theme {
	return DarkTheme()
}

func DarkTheme() Theme {
	t := Theme{
		Name: "dark",
		Vars: map[string]any{
			"primary":   "#00aaff",
			"secondary": 242,
		},
		Colors: map[string]string{
			"accent": "#00aaff", "border": "#00aaff", "borderAccent": "#00ffff", "borderMuted": "242",
			"success": "#00ff00", "error": "#ff5555", "warning": "#ffff00", "muted": "242", "dim": "240",
			"text": "", "thinkingText": "242",
			"selectedBg": "#2d2d30", "userMessageBg": "#2d2d30", "userMessageText": "",
			"customMessageBg": "#2d2d30", "customMessageText": "", "customMessageLabel": "#00aaff",
			"toolPendingBg": "#1e1e2e", "toolSuccessBg": "#1e2e1e", "toolErrorBg": "#2e1e1e",
			"toolTitle": "#00aaff", "toolOutput": "",
			"mdHeading": "#ffaa00", "mdLink": "#00aaff", "mdLinkUrl": "242", "mdCode": "#00ffff",
			"mdCodeBlock": "", "mdCodeBlockBorder": "242", "mdQuote": "242", "mdQuoteBorder": "242",
			"mdHr": "242", "mdListBullet": "#00ffff",
			"toolDiffAdded": "#00ff00", "toolDiffRemoved": "#ff5555", "toolDiffContext": "242",
			"syntaxComment": "242", "syntaxKeyword": "#00aaff", "syntaxFunction": "#00aaff",
			"syntaxVariable": "#ffaa00", "syntaxString": "#00ff00", "syntaxNumber": "#ff00ff",
			"syntaxType": "#00aaff", "syntaxOperator": "#00aaff", "syntaxPunctuation": "242",
			"thinkingOff": "242", "thinkingMinimal": "#00aaff", "thinkingLow": "#00aaff",
			"thinkingMedium": "#00ffff", "thinkingHigh": "#ff00ff", "thinkingXhigh": "#ff0000",
			"bashMode": "#ffaa00",
		},
		Export: &ExportColors{PageBg: "#18181e", CardBg: "#1e1e24", InfoBg: "#3c3728"},
	}
	_ = t.Resolve()
	return t
}

func LightTheme() Theme {
	t := Theme{
		Name: "light",
		Colors: map[string]string{
			"accent": "#0066cc", "border": "#0066cc", "borderAccent": "#0088aa", "borderMuted": "245",
			"success": "#008800", "error": "#cc0000", "warning": "#aa8800", "muted": "245", "dim": "250",
			"text": "", "thinkingText": "245",
			"selectedBg": "#e8e8ec", "userMessageBg": "#e8e8ec", "userMessageText": "",
			"customMessageBg": "#e8e8ec", "customMessageText": "", "customMessageLabel": "#0066cc",
			"toolPendingBg": "#f0f0f4", "toolSuccessBg": "#e8f5e8", "toolErrorBg": "#f5e8e8",
			"toolTitle": "#0066cc", "toolOutput": "",
			"mdHeading": "#aa6600", "mdLink": "#0066cc", "mdLinkUrl": "245", "mdCode": "#008888",
			"mdCodeBlock": "", "mdCodeBlockBorder": "245", "mdQuote": "245", "mdQuoteBorder": "245",
			"mdHr": "245", "mdListBullet": "#008888",
			"toolDiffAdded": "#008800", "toolDiffRemoved": "#cc0000", "toolDiffContext": "245",
			"syntaxComment": "245", "syntaxKeyword": "#0066cc", "syntaxFunction": "#0066cc",
			"syntaxVariable": "#aa6600", "syntaxString": "#008800", "syntaxNumber": "#880088",
			"syntaxType": "#0066cc", "syntaxOperator": "#0066cc", "syntaxPunctuation": "245",
			"thinkingOff": "245", "thinkingMinimal": "#0066cc", "thinkingLow": "#0066cc",
			"thinkingMedium": "#008888", "thinkingHigh": "#880088", "thinkingXhigh": "#cc0000",
			"bashMode": "#aa6600",
		},
	}
	_ = t.Resolve()
	return t
}

// DetectDefaultName возвращает dark или light по эвристике COLORFGBG.
// На Windows всегда "dark" (stub; без OSC/registry detection).
func DetectDefaultName() string {
	if isWindows() {
		return "dark"
	}
	fgbg := os.Getenv("COLORFGBG")
	if fgbg != "" {
		parts := strings.Split(fgbg, ";")
		if len(parts) >= 2 {
			bg := parts[len(parts)-1]
			if bg == "15" || bg == "7" {
				return "light"
			}
		}
	}
	return "dark"
}
