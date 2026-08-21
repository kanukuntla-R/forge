package renderer

import (
	"reflect"
	"testing"
)

func TestGenerateProps(t *testing.T) {
	tests := []struct {
		name  string
		props []PropType
		want  map[string]any
	}{
		{
			name:  "no props",
			props: nil,
			want:  map[string]any{},
		},
		{
			name: "primitives",
			props: []PropType{
				{Name: "title", Type: "string"},
				{Name: "count", Type: "number"},
				{Name: "active", Type: "boolean"},
			},
			want: map[string]any{
				"title":  "Sample Text",
				"count":  42,
				"active": false,
			},
		},
		{
			name:  "function placeholder",
			props: []PropType{{Name: "onClick", Type: "function", Optional: true}},
			want:  map[string]any{"onClick": "() => {}"},
		},
		{
			name: "nested object",
			props: []PropType{{
				Name: "user",
				Type: "object",
				Shape: []PropType{
					{Name: "name", Type: "string"},
					{Name: "age", Type: "number"},
				},
			}},
			want: map[string]any{
				"user": map[string]any{"name": "Sample Text", "age": 42},
			},
		},
		{
			name:  "unsupported type falls back to placeholder",
			props: []PropType{{Name: "variant", Type: "unsupported"}},
			want:  map[string]any{"variant": "Sample Text"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GenerateProps(tt.props)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GenerateProps() = %#v, want %#v", got, tt.want)
			}
		})
	}
}
