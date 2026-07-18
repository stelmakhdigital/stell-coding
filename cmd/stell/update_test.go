package main

import (
	"testing"
)

func TestParseUpdateArgsDefaultSelf(t *testing.T) {
	opts, err := parseUpdateArgs(nil)
	if err != nil {
		t.Fatal(err)
	}
	if opts.target != targetSelf || !opts.showExtensionsHint {
		t.Fatalf("opts: %+v", opts)
	}
}

func TestParseUpdateArgsAll(t *testing.T) {
	opts, err := parseUpdateArgs([]string{"--all"})
	if err != nil {
		t.Fatal(err)
	}
	if opts.target != targetAll {
		t.Fatalf("target: %v", opts.target)
	}
}

func TestParseUpdateArgsConflict(t *testing.T) {
	_, err := parseUpdateArgs([]string{"--all", "--self"})
	if err == nil {
		t.Fatal("expected conflict")
	}
}

func TestParseUpdateArgsExtensions(t *testing.T) {
	opts, err := parseUpdateArgs([]string{"--extensions"})
	if err != nil {
		t.Fatal(err)
	}
	if opts.target != targetExtensions {
		t.Fatalf("target: %v", opts.target)
	}
}

func TestParseUpdateArgsPositionalSelf(t *testing.T) {
	opts, err := parseUpdateArgs([]string{"self"})
	if err != nil {
		t.Fatal(err)
	}
	if opts.target != targetSelf {
		t.Fatalf("target: %v", opts.target)
	}
}

func TestHasOfflineFlag(t *testing.T) {
	if !hasOfflineFlag([]string{"update", "--offline"}) {
		t.Fatal("expected offline")
	}
}
