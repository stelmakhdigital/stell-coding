package catalog

import "testing"

func TestParseFrontmatter(t *testing.T) {
	fields, body, err := ParseFrontmatter(`---
name: demo
description: A demo skill
---
Body here`)
	if err != nil {
		t.Fatal(err)
	}
	if fields["name"] != "demo" || fields["description"] != "A demo skill" {
		t.Fatalf("fields = %v", fields)
	}
	if body != "Body here" {
		t.Fatalf("body = %q", body)
	}
}
