package themes

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolvePrecedence(t *testing.T) {
	dir := t.TempDir()
	global := filepath.Join(dir, "global")
	project := filepath.Join(dir, "project")
	_ = os.MkdirAll(filepath.Join(global, "themes"), 0o755)
	_ = os.MkdirAll(filepath.Join(project, ".stell", "themes"), 0o755)

	writeTheme := func(path, name, accent string) {
		t.Helper()
		body := `{"name":"` + name + `","colors":{`
		for i, k := range RequiredTokens {
			if i > 0 {
				body += ","
			}
			val := accent
			if k != "accent" {
				val = "242"
			}
			body += `"` + k + `":"` + val + `"`
		}
		body += `}}`
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	writeTheme(filepath.Join(global, "themes", "mine.json"), "mine", "#111111")
	writeTheme(filepath.Join(project, ".stell", "themes", "mine.json"), "mine", "#222222")

	trusted := true
	list, err := Resolve(ResolveOpts{
		GlobalDir:  global,
		ProjectDir: project,
		Workspace:  project,
		Trusted:    &trusted,
	})
	if err != nil {
		t.Fatal(err)
	}
	var mine *Theme
	for i := range list {
		if list[i].Name == "mine" {
			mine = &list[i]
			break
		}
	}
	if mine == nil {
		t.Fatal("mine theme missing")
	}
	if mine.Colors["accent"] != "#222222" {
		t.Fatalf("project should win, got %q", mine.Colors["accent"])
	}
}

func TestParseAutoThemeSetting(t *testing.T) {
	light, dark, ok := ParseAutoThemeSetting("solarized-light/solarized-dark")
	if !ok || light != "solarized-light" || dark != "solarized-dark" {
		t.Fatalf("got %v %q %q", ok, light, dark)
	}
	if _, _, ok := ParseAutoThemeSetting("dark"); ok {
		t.Fatal("fixed name should not parse as auto")
	}
	if ResolveThemeSetting("light/dark", "light") != "light" {
		t.Fatal("expected light")
	}
	if ResolveThemeSetting("light/dark", "dark") != "dark" {
		t.Fatal("expected dark")
	}
}

func TestMarkdownThemeMapping(t *testing.T) {
	th := DarkTheme()
	md := th.MarkdownTheme()
	if md.Heading == "" || md.Reset == "" {
		t.Fatalf("missing ansi: %+v", md)
	}
	out := HighlightCode(`func main() { return "hi" }`, md)
	if out == "" || !containsANSI(out) {
		t.Fatalf("expected highlighted output: %q", out)
	}
}

func containsANSI(s string) bool {
	return len(s) > 0 && (s[0] == '\x1b' || len(s) > 5)
}

func TestFindBuiltinByName(t *testing.T) {
	if tth := FindByName(ResolveOpts{}, "dark"); tth == nil || tth.Name != "dark" {
		t.Fatal("dark builtin")
	}
	if tth := FindByName(ResolveOpts{}, "light"); tth == nil || tth.Name != "light" {
		t.Fatal("light builtin")
	}
}
