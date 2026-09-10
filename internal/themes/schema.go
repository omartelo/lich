package themes

import (
	"encoding/json"
	"fmt"
	"maps"
	"slices"
)

// SchemaPath is where the generated schema lives in this repository, and the
// tail of the raw URL a theme author points "$schema" at.
const SchemaPath = "themes/lich-theme.schema.json"

const schemaID = "https://raw.githubusercontent.com/omartelo/lich/main/" + SchemaPath

// Schema renders the JSON Schema a theme file is checked against in an editor.
// It is generated from the same token sets and patterns validateTheme reads, so
// a token added to the bundled theme cannot leave the schema behind; the test
// beside this file fails when the checked-in copy has drifted.
func Schema() ([]byte, error) {
	schema := map[string]any{
		"$schema":              "https://json-schema.org/draft/2020-12/schema",
		"$id":                  schemaID,
		"title":                "lich theme",
		"description":          "A color theme for the lich harness.",
		"type":                 "object",
		"additionalProperties": false,
		"required":             []string{"id", "name", "scheme", "app", "terminal"},
		"properties": map[string]any{
			"$schema": map[string]any{"type": "string"},
			"id": map[string]any{
				"description": "Names the file the theme is stored as, so reserved ids and Windows device names are out.",
				"type":        "string",
				"pattern":     idPattern.String(),
				"maxLength":   themeIDMaxLength,
				"not":         map[string]any{"enum": rejectedIDs()},
			},
			"name": map[string]any{
				"type":      "string",
				"maxLength": themeNameMaxLength,
				// Anchorless, so it only asks for one non-space rune anywhere:
				// the name is rejected when it trims to nothing, not when it
				// is short.
				"pattern": `\S`,
			},
			"scheme": map[string]any{"enum": []string{SchemeLight, SchemeDark}},
			"origin": map[string]any{
				"description": "Written by lich on import; a hand-written value is ignored.",
				"enum":        []string{OriginBundled, OriginCustom},
			},
			"source": map[string]any{
				"description":          "Written by lich for a theme installed from a repository.",
				"type":                 "object",
				"additionalProperties": false,
				"required":             []string{"url", "version"},
				"properties": map[string]any{
					"url":     map[string]any{"type": "string"},
					"version": map[string]any{"type": "string", "pattern": `^v?\d+\.\d+\.\d+$`},
				},
			},
			"app":      colorObject(appTokens, "#/$defs/appColor", sortedKeys(appTokens)),
			"terminal": colorObject(terminalTokens, "#/$defs/terminalColor", []string{"background", "foreground"}),
		},
		"$defs": map[string]any{
			"appColor":      colorValue(appColorPattern),
			"terminalColor": colorValue(terminalColorPattern),
		},
	}
	data, err := json.MarshalIndent(schema, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode theme schema: %w", err)
	}
	return append(data, '\n'), nil
}

func colorObject(tokens map[string]struct{}, ref string, required []string) map[string]any {
	properties := make(map[string]any, len(tokens))
	for token := range tokens {
		properties[token] = map[string]any{"$ref": ref}
	}
	return map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"required":             required,
		"properties":           properties,
	}
}

func colorValue(pattern string) map[string]any {
	return map[string]any{
		"type":      "string",
		"pattern":   pattern,
		"maxLength": colorValueMaxLength,
	}
}

// rejectedIDs is every id validateID turns down for what it means rather than
// for its spelling.
func rejectedIDs() []string {
	all := make(map[string]struct{}, len(reserved)+len(windowsDeviceNames))
	maps.Copy(all, reserved)
	maps.Copy(all, windowsDeviceNames)
	return sortedKeys(all)
}

func sortedKeys(tokens map[string]struct{}) []string {
	return slices.Sorted(maps.Keys(tokens))
}
