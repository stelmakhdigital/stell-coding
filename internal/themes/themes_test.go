package themes

import "testing"

func TestDarkThemeHasRequiredTokens(t *testing.T) {
	th := DarkTheme()
	if err := th.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestResolveVars(t *testing.T) {
	th := Theme{
		Name: "x",
		Vars: map[string]any{"primary": "#abc"},
		Colors: map[string]string{
			"accent": "primary",
		},
	}
	for _, k := range RequiredTokens {
		if _, ok := th.Colors[k]; !ok {
			th.Colors[k] = "primary"
		}
	}
	if err := th.Resolve(); err != nil {
		t.Fatal(err)
	}
	if th.Colors["accent"] != "#abc" {
		t.Fatalf("got %q", th.Colors["accent"])
	}
}
