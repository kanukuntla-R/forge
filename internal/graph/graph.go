package graph

// Graph is the canonical knowledge-graph format emitted as JSON.
type Graph struct {
	Version string  `json:"version" yaml:"version"`
	Project Project `json:"project" yaml:"project"`
	Nodes   []Node  `json:"nodes" yaml:"nodes"`
	Edges   []Edge  `json:"edges" yaml:"edges"`
}

type Project struct {
	Name        string `json:"name" yaml:"name"`
	Description string `json:"description" yaml:"description"`
}

type Node struct {
	ID    string `json:"id" yaml:"id"`
	Type  string `json:"type" yaml:"type"`
	Label string `json:"label" yaml:"label"`
	Path  string `json:"path" yaml:"path"`
}

type Edge struct {
	From string `json:"from" yaml:"from"`
	To   string `json:"to" yaml:"to"`
	Type string `json:"type" yaml:"type"`
}
