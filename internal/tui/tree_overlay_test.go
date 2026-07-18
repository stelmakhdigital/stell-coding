package tui

import (
	"strings"
	"testing"

	"github.com/stelmakhdigital/ai"
	"stell/agent/session"
)

func TestBuildTreeItemsLinearFlat(t *testing.T) {
	m := session.NewManager(t.TempDir())
	for i := 0; i < 5; i++ {
		_, _ = m.AppendMessage(ai.Message{Role: ai.RoleUser, Content: "u"})
		_, _ = m.AppendMessage(ai.Message{Role: ai.RoleAssistant, Content: "a"})
	}
	items := buildTreeItems(m, "all", "", nil)
	if len(items) < 10 {
		t.Fatalf("want >=10 items, got %d", len(items))
	}
	for _, it := range items {
		if it.depth != 0 {
			t.Fatalf("linear chain should be flat depth=0, got depth=%d for %q", it.depth, it.label)
		}
	}
	out := renderTreeOverlay(items, 0, "default", "", nil, false)
	// No staircase: only root-level lines, no ├─ cascade
	if strings.Count(out, "├─") > 0 {
		t.Fatalf("linear tree should not show branch indent markers:\n%s", out)
	}
}

func TestBuildTreeItemsForkIndent(t *testing.T) {
	m := session.NewManager(t.TempDir())
	id1, _ := m.AppendMessage(ai.Message{Role: ai.RoleUser, Content: "root"})
	_, _ = m.AppendMessage(ai.Message{Role: ai.RoleAssistant, Content: "first"})
	if err := m.ForkAt(id1); err != nil {
		t.Fatal(err)
	}
	_, _ = m.AppendMessage(ai.Message{Role: ai.RoleUser, Content: "forked"})
	_, _ = m.AppendMessage(ai.Message{Role: ai.RoleAssistant, Content: "second"})

	items := buildTreeItems(m, "all", "", nil)
	byLabel := map[string]int{}
	for _, it := range items {
		byLabel[it.label] = it.depth
	}
	// Root user stays at 0; siblings under it deepen.
	rootDepth := -1
	for _, it := range items {
		if strings.Contains(it.label, "user: root") {
			rootDepth = it.depth
			break
		}
	}
	if rootDepth != 0 {
		t.Fatalf("root depth=%d want 0", rootDepth)
	}
	var branchDepths []int
	for _, it := range items {
		if it.depth > 0 {
			branchDepths = append(branchDepths, it.depth)
		}
	}
	if len(branchDepths) == 0 {
		t.Fatalf("fork should deepen some nodes, items=%v", items)
	}
	for _, d := range branchDepths {
		if d < 1 {
			t.Fatalf("branch depth=%d want >=1", d)
		}
	}
}

func TestBuildTreeItemsSearchAndFold(t *testing.T) {
	m := session.NewManager(t.TempDir())
	_, _ = m.AppendMessage(ai.Message{Role: ai.RoleUser, Content: "hello world"})
	_, _ = m.AppendMessage(ai.Message{Role: ai.RoleAssistant, Content: "hi there"})
	items := buildTreeItems(m, "default", "hello", nil)
	if len(items) == 0 {
		t.Fatal("expected search hits")
	}
	all := buildTreeItems(m, "all", "", nil)
	if len(all) < 2 {
		t.Fatalf("expected >=2 nodes, got %d", len(all))
	}
	// Parent first node with kids if forked — just ensure fold map doesn't panic.
	folded := map[string]bool{all[0].id: true}
	_ = buildTreeItems(m, "all", "", folded)
}

func TestRenderTreeHasAscii(t *testing.T) {
	items := []treeItem{{id: "a", label: "root", depth: 0, hasKids: true}, {id: "b", label: "child", depth: 1, parentID: "a"}}
	out := renderTreeOverlay(items, 1, "default", "hi", map[string]bool{"a": true}, false)
	if !strings.Contains(out, "search:") {
		t.Fatal("search display missing")
	}
	if !strings.Contains(out, "▸") && !strings.Contains(out, "▾") {
		t.Fatal("fold marks missing")
	}
}

func TestRenderTreeShowTs(t *testing.T) {
	items := []treeItem{{
		id: "a", label: "root", depth: 0, hasKids: true,
		timestamp: "2026-07-15T12:34:56.789Z",
	}}
	out := renderTreeOverlay(items, 0, "default", "", nil, true)
	if !strings.Contains(out, "timestamps: on") {
		t.Fatal("expected timestamps indicator")
	}
	if !strings.Contains(out, formatTreeTs(items[0].timestamp)) {
		t.Fatalf("expected formatted timestamp in output: %q", out)
	}
	hidden := renderTreeOverlay(items, 0, "default", "", nil, false)
	if strings.Contains(hidden, "timestamps: on") {
		t.Fatal("timestamps should be off")
	}
}
