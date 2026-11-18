package server

import (
	"context"
	"log/slog"
	"reflect"
	"slices"
	"strings"
	"testing"

	"bw-mcp/internal/bw"
)

func TestItemToolSchemasTrackPublicModel(t *testing.T) {
	session := connectServer(t, New(slog.New(slog.DiscardHandler), nil))
	listedTools, err := session.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}

	toolSchemas := make(map[string]any, len(listedTools.Tools))
	for _, tool := range listedTools.Tools {
		toolSchemas[tool.Name] = tool.OutputSchema
	}

	want := publicItemJSONFields()
	listFields := listItemSchemaFields(t, toolSchemas["list_items"])
	getFields := objectSchemaFields(t, toolSchemas["get_item"])
	if !slices.Equal(listFields, want) {
		t.Errorf("list_items fields = %v, want public Item fields %v", listFields, want)
	}
	if !slices.Equal(getFields, want) {
		t.Errorf("get_item fields = %v, want public Item fields %v", getFields, want)
	}
}

func publicItemJSONFields() []string {
	typ := reflect.TypeOf(bw.Item{})
	fields := make([]string, 0, typ.NumField())
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		if field.PkgPath != "" {
			continue
		}
		name := strings.Split(field.Tag.Get("json"), ",")[0]
		if name == "-" {
			continue
		}
		if name == "" {
			name = field.Name
		}
		fields = append(fields, name)
	}
	slices.Sort(fields)
	return fields
}

func listItemSchemaFields(t *testing.T, raw any) []string {
	t.Helper()
	properties := objectSchemaProperties(t, raw)
	items, ok := properties["items"].(map[string]any)
	if !ok {
		t.Fatalf("list_items.items schema has type %T", properties["items"])
	}
	branches, ok := items["anyOf"].([]any)
	if !ok || len(branches) != 2 {
		t.Fatalf("list_items.items anyOf = %#v", items["anyOf"])
	}
	array, ok := branches[1].(map[string]any)
	if !ok {
		t.Fatalf("list_items.items array branch has type %T", branches[1])
	}
	item, ok := array["items"].(map[string]any)
	if !ok {
		t.Fatalf("list_items.items[]. schema has type %T", array["items"])
	}
	return schemaFieldNames(t, item)
}

func objectSchemaFields(t *testing.T, raw any) []string {
	t.Helper()
	return schemaFieldNames(t, raw)
}

func schemaFieldNames(t *testing.T, raw any) []string {
	t.Helper()
	properties := objectSchemaProperties(t, raw)
	fields := make([]string, 0, len(properties))
	for name := range properties {
		fields = append(fields, name)
	}
	slices.Sort(fields)
	return fields
}

func objectSchemaProperties(t *testing.T, raw any) map[string]any {
	t.Helper()
	schema, ok := raw.(map[string]any)
	if !ok {
		t.Fatalf("tool output schema has type %T", raw)
	}
	properties, ok := schema["properties"].(map[string]any)
	if !ok {
		t.Fatalf("tool schema properties have type %T", schema["properties"])
	}
	return properties
}
