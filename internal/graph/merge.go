package graph

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Read reads the knowledge graph from
// <projectRoot>/.understand-anything/knowledge-graph.json.
func Read(projectRoot string) (*Graph, error) {
	path := filepath.Join(projectRoot, ".understand-anything", "knowledge-graph.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading knowledge-graph.json: %w", err)
	}
	var g Graph
	if err := json.Unmarshal(data, &g); err != nil {
		return nil, fmt.Errorf("parsing knowledge-graph.json: %w", err)
	}
	return &g, nil
}

// MergeResult counts what changed during a Merge call.
type MergeResult struct {
	NodesAdded   int
	NodesSkipped int
	EdgesAdded   int
	EdgesSkipped int
}

// Merge appends nodes and edges from fragment into existing (mutating existing
// in place). Node duplicates are detected by id; edge duplicates by (from, to,
// type) triple. Returns counts of added and skipped entries.
func Merge(existing, fragment *Graph) MergeResult {
	var res MergeResult

	nodeIDs := make(map[string]bool, len(existing.Nodes))
	for _, n := range existing.Nodes {
		nodeIDs[n.ID] = true
	}
	for _, n := range fragment.Nodes {
		if nodeIDs[n.ID] {
			res.NodesSkipped++
			continue
		}
		existing.Nodes = append(existing.Nodes, n)
		nodeIDs[n.ID] = true
		res.NodesAdded++
	}

	type edgeKey struct{ from, to, etype string }
	edgeKeys := make(map[edgeKey]bool, len(existing.Edges))
	for _, e := range existing.Edges {
		edgeKeys[edgeKey{e.From, e.To, e.Type}] = true
	}
	for _, e := range fragment.Edges {
		k := edgeKey{e.From, e.To, e.Type}
		if edgeKeys[k] {
			res.EdgesSkipped++
			continue
		}
		existing.Edges = append(existing.Edges, e)
		edgeKeys[k] = true
		res.EdgesAdded++
	}

	return res
}
