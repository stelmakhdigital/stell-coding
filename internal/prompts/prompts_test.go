package prompts

import "testing"

func TestSubstitute(t *testing.T) {
	body := "Hello $1, all: $ARGUMENTS, rest: ${@:2}"
	got := Substitute(body, []string{"world", "foo", "bar"})
	want := "Hello world, all: world foo bar, rest: foo bar"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestSubstituteDefault(t *testing.T) {
	body := "Summarize in ${1:-7} points"
	got := Substitute(body, nil)
	if got != "Summarize in 7 points" {
		t.Fatalf("got %q", got)
	}
}

func TestSubstituteSliceLength(t *testing.T) {
	body := "Items: ${@:2:1}"
	got := Substitute(body, []string{"a", "b", "c"})
	if got != "Items: b" {
		t.Fatalf("got %q", got)
	}
}

func TestExpandCommand(t *testing.T) {
	reg := NewRegistry()
	reg.byName["review"] = &Template{Name: "review", Body: "Review $1"}
	got := ExpandCommand(reg, `/review staged`)
	if got != "Review staged" {
		t.Fatalf("got %q", got)
	}
}

func TestParseCommandArgs(t *testing.T) {
	args := ParseCommandArgs(`one "two three" four`)
	if len(args) != 3 || args[1] != "two three" {
		t.Fatalf("args = %v", args)
	}
}
