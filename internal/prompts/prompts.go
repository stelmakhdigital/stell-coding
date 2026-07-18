package prompts

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/stelmakhdigital/stell-coding/internal/catalog"
)

type Template struct {
	Name         string
	Description  string
	ArgumentHint string
	Body         string
	Source       catalog.Source
	Path         string
}

type Entry struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Source      string `json:"source"`
}

type Registry struct {
	byName map[string]*Template
}

func NewRegistry() *Registry {
	return &Registry{byName: map[string]*Template{}}
}

func (r *Registry) Scan(dirs []catalog.Dir) {
	for _, d := range dirs {
		r.scanDir(d.Path, d.Source)
	}
}

func (r *Registry) scanDir(root string, source catalog.Source) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		path := filepath.Join(root, e.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		tmpl, err := parseTemplate(string(data), path, source)
		if err != nil {
			continue
		}
		r.byName[tmpl.Name] = tmpl
	}
}

func parseTemplate(data, path string, source catalog.Source) (*Template, error) {
	fields, body, err := catalog.ParseFrontmatter(data)
	if err != nil {
		return nil, err
	}
	name := fields["name"]
	if name == "" {
		name = strings.TrimSuffix(filepath.Base(path), ".md")
	}
	desc := fields["description"]
	if desc == "" {
		// первая непустая строка body
		for _, line := range strings.Split(body, "\n") {
			line = strings.TrimSpace(line)
			if line != "" {
				desc = line
				if len(desc) > 60 {
					desc = desc[:60] + "..."
				}
				break
			}
		}
	}
	if desc == "" {
		return nil, fmt.Errorf("description required")
	}
	return &Template{
		Name: name, Description: desc, ArgumentHint: fields["argument-hint"],
		Body: body, Source: source, Path: path,
	}, nil
}

func (r *Registry) Get(name string) (*Template, bool) {
	t, ok := r.byName[name]
	return t, ok
}

func (r *Registry) Has(name string) bool {
	_, ok := r.byName[name]
	return ok
}

func (r *Registry) Render(name string, args []string) (string, error) {
	t, ok := r.Get(name)
	if !ok {
		return "", fmt.Errorf("prompt %q not found", name)
	}
	return Substitute(t.Body, args), nil
}

func (r *Registry) SlashDescription(name string) string {
	t, ok := r.Get(name)
	if !ok {
		return ""
	}
	if t.ArgumentHint != "" {
		return t.ArgumentHint + " — " + t.Description
	}
	return t.Description
}

func (r *Registry) List() []Entry {
	names := make([]string, 0, len(r.byName))
	for n := range r.byName {
		names = append(names, n)
	}
	sort.Strings(names)
	out := make([]Entry, 0, len(names))
	for _, n := range names {
		t := r.byName[n]
		out = append(out, Entry{
			Name: t.Name, Description: t.Description, Source: string(t.Source),
		})
	}
	return out
}

