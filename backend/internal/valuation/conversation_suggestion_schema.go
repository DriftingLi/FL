package valuation

import "encoding/json"

var valuationSuggestionSchemaJSON = buildValuationSuggestionSchema()

func buildValuationSuggestionSchema() string {
	fields := make([]string, 0, len(draftFieldSpecs))
	rangeFields := make([]string, 0, len(draftFieldSpecs))
	for _, spec := range draftFieldSpecs {
		fields = append(fields, spec.name)
		if spec.allowRange {
			rangeFields = append(rangeFields, spec.name)
		}
	}
	root := map[string]any{
		"$schema":              "https://json-schema.org/draft/2020-12/schema",
		"type":                 "object",
		"additionalProperties": false,
		"required":             []string{"schema_version", "patches", "supplements", "questions"},
		"properties": map[string]any{
			"schema_version": map[string]any{"type": "integer", "const": SuggestionSchemaVersion},
			"patches": map[string]any{
				"type": "array", "maxItems": len(draftFieldSpecs),
				"items": map[string]any{
					"type": "object", "additionalProperties": false,
					"required": []string{"field", "state", "value", "range", "candidate_keys", "evidence"},
					"properties": map[string]any{
						"field":          map[string]any{"type": "string", "enum": fields},
						"state":          map[string]any{"type": "string", "enum": []string{"missing", "unknown", "value", "range", "candidate"}},
						"value":          map[string]any{},
						"range":          map[string]any{"type": []string{"object", "null"}, "additionalProperties": false, "required": []string{"min", "max"}, "properties": map[string]any{"min": map[string]any{"type": "number"}, "max": map[string]any{"type": "number"}}},
						"candidate_keys": map[string]any{"type": "array", "maxItems": 8, "uniqueItems": true, "items": map[string]any{"type": "string", "minLength": 1, "maxLength": 160}},
						"evidence":       map[string]any{"type": "array", "maxItems": 8, "items": map[string]any{"type": "string", "minLength": 1, "maxLength": 300}},
					},
					"allOf": []any{map[string]any{
						"if": map[string]any{
							"properties": map[string]any{
								"state": map[string]any{"const": string(DraftFieldRange)},
								"field": map[string]any{"enum": rangeFields},
							},
							"required": []string{"state", "field"},
						},
						"then": map[string]any{"properties": map[string]any{"range": map[string]any{"type": "object"}}},
					}},
				},
			},
			"supplements": map[string]any{
				"type": "object", "maxProperties": 20,
				"propertyNames":        map[string]any{"pattern": "^[a-z][a-z0-9_]{0,31}$"},
				"additionalProperties": map[string]any{"type": "string", "maxLength": 500},
			},
			"questions": map[string]any{"type": "array", "maxItems": 5, "uniqueItems": true, "items": map[string]any{"type": "string", "minLength": 1, "maxLength": 300}},
		},
	}
	encoded, err := json.Marshal(root)
	if err != nil {
		panic(err)
	}
	return string(encoded)
}
