package graph_test

import (
	"testing"

	"github.com/kanukuntla-r/forge/internal/graph"
)

func TestMergeNoCollisions(t *testing.T) {
	existing := &graph.Graph{
		Nodes: []graph.Node{{ID: "a", Type: "page", Label: "A"}},
		Edges: []graph.Edge{{From: "a", To: "b", Type: "calls"}},
	}
	fragment := &graph.Graph{
		Nodes: []graph.Node{{ID: "b", Type: "route", Label: "B"}},
		Edges: []graph.Edge{{From: "b", To: "c", Type: "calls"}},
	}

	res := graph.Merge(existing, fragment)

	if res.NodesAdded != 1 || res.NodesSkipped != 0 {
		t.Errorf("nodes: added=%d skipped=%d, want added=1 skipped=0", res.NodesAdded, res.NodesSkipped)
	}
	if res.EdgesAdded != 1 || res.EdgesSkipped != 0 {
		t.Errorf("edges: added=%d skipped=%d, want added=1 skipped=0", res.EdgesAdded, res.EdgesSkipped)
	}
	if len(existing.Nodes) != 2 {
		t.Errorf("existing.Nodes len = %d, want 2", len(existing.Nodes))
	}
	if len(existing.Edges) != 2 {
		t.Errorf("existing.Edges len = %d, want 2", len(existing.Edges))
	}
}

func TestMergeWithCollisions(t *testing.T) {
	existing := &graph.Graph{
		Nodes: []graph.Node{{ID: "foo", Type: "page", Label: "Foo"}},
		Edges: []graph.Edge{{From: "foo", To: "bar", Type: "calls"}},
	}
	fragment := &graph.Graph{
		Nodes: []graph.Node{
			{ID: "foo", Type: "page", Label: "Foo duplicate"},
			{ID: "new", Type: "route", Label: "New"},
		},
		Edges: []graph.Edge{
			{From: "foo", To: "bar", Type: "calls"},
			{From: "new", To: "foo", Type: "calls"},
		},
	}

	res := graph.Merge(existing, fragment)

	if res.NodesAdded != 1 || res.NodesSkipped != 1 {
		t.Errorf("nodes: added=%d skipped=%d, want added=1 skipped=1", res.NodesAdded, res.NodesSkipped)
	}
	if res.EdgesAdded != 1 || res.EdgesSkipped != 1 {
		t.Errorf("edges: added=%d skipped=%d, want added=1 skipped=1", res.EdgesAdded, res.EdgesSkipped)
	}
	if len(existing.Nodes) != 2 {
		t.Errorf("existing.Nodes len = %d, want 2", len(existing.Nodes))
	}
	if len(existing.Edges) != 2 {
		t.Errorf("existing.Edges len = %d, want 2", len(existing.Edges))
	}
}

func TestMergeEmptyFragment(t *testing.T) {
	existing := &graph.Graph{
		Nodes: []graph.Node{{ID: "x", Type: "page", Label: "X"}},
		Edges: []graph.Edge{{From: "x", To: "y", Type: "calls"}},
	}
	fragment := &graph.Graph{}

	res := graph.Merge(existing, fragment)

	if res.NodesAdded != 0 || res.NodesSkipped != 0 || res.EdgesAdded != 0 || res.EdgesSkipped != 0 {
		t.Errorf("empty fragment should produce all-zero result, got %+v", res)
	}
	if len(existing.Nodes) != 1 || len(existing.Edges) != 1 {
		t.Error("existing should be unchanged after merging empty fragment")
	}
}
