// Package manifest defines the schema for blueprint manifest.yaml files
// and provides parsing and validation.
package manifest

// Kind classifies what kind of project a blueprint produces.
type Kind string

const (
	KindAgentSkill Kind = "agent-skill"
	KindCLITool    Kind = "cli-tool"
	KindService    Kind = "service"
	KindApp        Kind = "app"
	KindLibrary    Kind = "library"
	KindPipeline   Kind = "pipeline"
)

// VarType is the data type of a blueprint variable.
type VarType string

const (
	VarTypeString VarType = "string"
	VarTypeBool   VarType = "bool"
	VarTypeInt    VarType = "int"
	VarTypeChoice VarType = "choice"
	VarTypePath   VarType = "path"
)

// Stack holds language/runtime/framework metadata used by forge list.
type Stack struct {
	Language  string   `yaml:"language"`
	Runtime   string   `yaml:"runtime"`
	Framework string   `yaml:"framework"`
	Tags      []string `yaml:"tags"`
}

// Variable is one user-configurable input in a blueprint.
type Variable struct {
	Name    string   `yaml:"name"`
	Prompt  string   `yaml:"prompt"`
	Type    VarType  `yaml:"type"`
	Default any      `yaml:"default"`
	Choices []string `yaml:"choices,omitempty"`
	Pattern string   `yaml:"pattern,omitempty"`
	Min     *int     `yaml:"min,omitempty"`
	Max     *int     `yaml:"max,omitempty"`
}

// Target describes where the scaffolded project is written.
type Target struct {
	DefaultPath string `yaml:"default_path"`
}

// Hook is a shell command run after the project is scaffolded.
type Hook struct {
	Name     string `yaml:"name"`
	Shell    string `yaml:"shell"`
	Optional bool   `yaml:"optional"`
}

// ExtensionArg is a parameter for a forge add extension.
type ExtensionArg struct {
	Name    string `yaml:"name"`
	Prompt  string `yaml:"prompt"`
	Pattern string `yaml:"pattern,omitempty"`
}

// Extension defines a component that forge add can append to an existing project.
type Extension struct {
	Name          string         `yaml:"name"`
	Description   string         `yaml:"description"`
	Template      string         `yaml:"template"`
	Args          []ExtensionArg `yaml:"args,omitempty"`
	GraphFragment string         `yaml:"graph_fragment,omitempty"`
}

// Manifest is the parsed representation of a blueprint's manifest.yaml.
type Manifest struct {
	Name        string      `yaml:"name"`
	DisplayName string      `yaml:"display_name"`
	Description string      `yaml:"description"`
	Version     string      `yaml:"version"`
	Authors     []string    `yaml:"authors"`
	Kind        Kind        `yaml:"kind"`
	Stack       Stack       `yaml:"stack"`
	Variables   []Variable  `yaml:"variables"`
	Target      Target      `yaml:"target"`
	PostCreate  []Hook      `yaml:"post_create"`
	Extensions  []Extension `yaml:"extensions"`
}
