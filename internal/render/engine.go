package render

import (
	"bytes"
	"io"
	"text/template"
)

// Render renders a named template from its source string with the given context.
// Callers are responsible for converting snake_case variable keys to PascalCase
// before passing ctx — use ToTemplateContext for that.
// Missing keys produce an error rather than silently expanding to "<no value>".
func Render(name, source string, ctx map[string]any) (string, error) {
	tmpl, err := template.New(name).
		Option("missingkey=error").
		Funcs(forgeFuncMap).
		Parse(source)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, ctx); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// RenderFile reads a template from r and delegates to Render.
func RenderFile(name string, r io.Reader, ctx map[string]any) (string, error) {
	src, err := io.ReadAll(r)
	if err != nil {
		return "", err
	}
	return Render(name, string(src), ctx)
}

// forgeFuncMap holds the custom functions available in all forge templates.
var forgeFuncMap = template.FuncMap{
	// seq returns [start, start+1, ..., end] inclusive, useful in graph.yaml for_each loops.
	"seq": func(start, end int) []int {
		if end < start {
			return nil
		}
		out := make([]int, end-start+1)
		for i := range out {
			out[i] = start + i
		}
		return out
	},
}
