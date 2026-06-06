package manifest

import (
	"fmt"
	"io"
	"os"
	"regexp"

	"gopkg.in/yaml.v3"
)

// Parse reads and validates a manifest from r.
func Parse(r io.Reader) (*Manifest, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("reading manifest: %w", err)
	}
	var m Manifest
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parsing manifest YAML: %w", err)
	}
	if err := validate(&m); err != nil {
		return nil, err
	}
	return &m, nil
}

// ParseFile reads and validates the manifest at path.
func ParseFile(path string) (*Manifest, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening manifest %s: %w", path, err)
	}
	defer f.Close()
	return Parse(f)
}

var validKinds = map[Kind]bool{
	KindAgentSkill: true,
	KindCLITool:    true,
	KindService:    true,
	KindApp:        true,
	KindLibrary:    true,
	KindPipeline:   true,
}

func validate(m *Manifest) error {
	if m.Name == "" {
		return fmt.Errorf("manifest missing required field: name")
	}
	if m.Kind == "" {
		return fmt.Errorf("manifest missing required field: kind")
	}
	if !validKinds[m.Kind] {
		return fmt.Errorf("manifest: invalid kind %q; must be one of: agent-skill, cli-tool, service, app, library, pipeline", m.Kind)
	}
	for i, v := range m.Variables {
		if v.Name == "" {
			return fmt.Errorf("manifest: variable[%d] missing required field: name", i)
		}
		if v.Type == VarTypeChoice && len(v.Choices) == 0 {
			return fmt.Errorf("manifest: variable %q has type 'choice' but no choices defined", v.Name)
		}
		if v.Pattern != "" {
			if _, err := regexp.Compile(v.Pattern); err != nil {
				return fmt.Errorf("manifest: variable %q has invalid pattern: %w", v.Name, err)
			}
		}
	}
	return nil
}
