package skills

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"stell/coding-agent/internal/catalog"
)

type Skill struct {
	Name                   string
	Description            string
	Body                   string
	Source                 catalog.Source
	Path                   string
	DisableModelInvocation bool
}

type Entry struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Source      string `json:"source"`
}

type Registry struct {
	byName map[string]*Skill
}

func NewRegistry() *Registry {
	return &Registry{byName: map[string]*Skill{}}
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
		if !e.IsDir() {
			continue
		}
		skillPath := filepath.Join(root, e.Name(), "SKILL.md")
		data, err := os.ReadFile(skillPath)
		if err != nil {
			continue
		}
		skill, err := parseSkill(string(data), skillPath, source)
		if err != nil {
			continue
		}
		r.byName[skill.Name] = skill
	}
}

var skillNameRe = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

func parseSkill(data, path string, source catalog.Source) (*Skill, error) {
	fields, body, err := catalog.ParseFrontmatter(data)
	if err != nil {
		return nil, err
	}
	name := fields["name"]
	if name == "" {
		name = filepath.Base(filepath.Dir(path))
	}
	if !skillNameRe.MatchString(name) {
		return nil, fmt.Errorf("invalid skill name %q", name)
	}
	desc := fields["description"]
	if desc == "" {
		return nil, fmt.Errorf("description required")
	}
	disable := strings.EqualFold(fields["disable-model-invocation"], "true")
	return &Skill{
		Name: name, Description: desc, Body: body,
		Source: source, Path: path,
		DisableModelInvocation: disable,
	}, nil
}

func (r *Registry) Get(name string) (*Skill, bool) {
	s, ok := r.byName[name]
	return s, ok
}

func (r *Registry) List() []Entry {
	names := make([]string, 0, len(r.byName))
	for n := range r.byName {
		names = append(names, n)
	}
	sort.Strings(names)
	out := make([]Entry, 0, len(names))
	for _, n := range names {
		s := r.byName[n]
		out = append(out, Entry{
			Name: s.Name, Description: s.Description, Source: string(s.Source),
		})
	}
	return out
}

func (r *Registry) PromptXML() string {
	if r == nil || len(r.byName) == 0 {
		return ""
	}
	list := make([]*Skill, 0, len(r.byName))
	for _, s := range r.byName {
		list = append(list, s)
	}
	sort.Slice(list, func(i, j int) bool { return list[i].Name < list[j].Name })
	return FormatForPrompt(list)
}
