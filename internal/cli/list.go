package cli

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/kanukuntla-r/forge/internal/blueprint"
)

var listJSON bool

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List available blueprints",
	Long: `List all blueprints available to forge: built-in (embedded in the binary),
user-installed (~/.config/forge/blueprints/), and project-local (.forge/blueprints/).`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		r, err := blueprint.NewRegistry()
		if err != nil {
			return fmt.Errorf("loading blueprints: %w", err)
		}
		return runList(cmd.OutOrStdout(), r.List(), listJSON)
	},
}

func init() {
	listCmd.Flags().BoolVar(&listJSON, "json", false, "Output as a JSON array")
	rootCmd.AddCommand(listCmd)
}

type blueprintListItem struct {
	Name        string `json:"name"`
	Kind        string `json:"kind"`
	Description string `json:"description"`
	Version     string `json:"version"`
	Source      string `json:"source"`
}

func runList(w io.Writer, blueprints []*blueprint.Blueprint, asJSON bool) error {
	if asJSON {
		items := make([]blueprintListItem, len(blueprints))
		for i, bp := range blueprints {
			items[i] = blueprintListItem{
				Name:        bp.Manifest.Name,
				Kind:        string(bp.Manifest.Kind),
				Description: bp.Manifest.Description,
				Version:     bp.Manifest.Version,
				Source:      sourceString(bp.Source),
			}
		}
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(items)
	}
	for _, bp := range blueprints {
		fmt.Fprintf(w, "%s  %s\n", bp.Manifest.Name, bp.Manifest.Description)
	}
	return nil
}

func sourceString(s blueprint.Source) string {
	switch s {
	case blueprint.SourceEmbedded:
		return "embedded"
	case blueprint.SourceUserInstalled:
		return "user-installed"
	case blueprint.SourceProjectLocal:
		return "project-local"
	default:
		return "unknown"
	}
}
