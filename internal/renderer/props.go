package renderer

// PropType mirrors one entry of scripts/types.js's "props" JSON array.
type PropType struct {
	Name     string     `json:"name"`
	Type     string     `json:"type"`
	Optional bool       `json:"optional"`
	Shape    []PropType `json:"shape"`
}

// TypeExtraction is the JSON payload emitted by scripts/types.js.
type TypeExtraction struct {
	Props    []PropType `json:"props"`
	Warnings []string   `json:"warnings"`
}

// GenerateProps builds Level 1 synthetic placeholder values (type-based,
// not schema-aware) for each extracted prop, keyed by prop name.
func GenerateProps(props []PropType) map[string]any {
	result := make(map[string]any, len(props))
	for _, p := range props {
		result[p.Name] = generatePropValue(p)
	}
	return result
}

func generatePropValue(p PropType) any {
	switch p.Type {
	case "string":
		return "Sample Text"
	case "number":
		return 42
	case "boolean":
		return false
	case "function":
		// Not a real function — JSON can't carry one. Harmless because
		// synthetic renders never invoke event handlers.
		return "() => {}"
	case "object":
		return GenerateProps(p.Shape)
	default:
		// unsupported (generics, unions, ...): fall back to a harmless placeholder
		return "Sample Text"
	}
}
