package edit_test

import (
	"encoding/json/v2"
	"slices"
	"testing"

	"github.com/MarkRosemaker/openapi"
	edit "github.com/MarkRosemaker/openapi-edit"
)

func load(t *testing.T, schemas string) *openapi.Document {
	t.Helper()

	doc, err := openapi.LoadFromDataJSON([]byte(`{
  "openapi": "3.1.0",
  "info": {"title": "t", "version": "1"},
  "components": {"schemas": ` + schemas + `}
}`))
	if err != nil {
		t.Fatal(err)
	}

	return doc
}

func jsonOf(t *testing.T, s *openapi.Schema) string {
	t.Helper()

	b, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}

	return string(b)
}

func TestMergeUnions_Variants(t *testing.T) {
	doc := load(t, `{
    "Block": {"anyOf": [{"$ref": "#/components/schemas/Paragraph"}, {"$ref": "#/components/schemas/Divider"}]},
    "Paragraph": {"type": "object", "properties": {
      "id": {"type": "string"}, "type": {"type": "string", "const": "paragraph"}, "paragraph": {"type": "string"}
    }, "required": ["id", "type", "paragraph"]},
    "Divider": {"type": "object", "properties": {
      "id": {"type": "string"}, "type": {"type": "string", "const": "divider"}, "divider": {"type": "object"}
    }, "required": ["id", "type", "divider"]}
  }`)

	if n := edit.MergeUnions(doc, "type"); n != 1 {
		t.Fatalf("merged %d unions, want 1", n)
	}

	want := `{"type":"object","properties":{"id":{"type":"string"},"type":{"type":"string","enum":["paragraph","divider"]},` +
		`"paragraph":{"type":"string"},"divider":{"type":"object"}},"required":["id","type"]}`
	if got := jsonOf(t, doc.Components.Schemas["Block"]); got != want {
		t.Errorf("got  %s\nwant %s", got, want)
	}

	if len(doc.Components.Schemas) != 1 {
		t.Error("the variants nothing refers to anymore are still there")
	}
}

func TestMergeUnions_CommonPartsAndNestedUnions(t *testing.T) {
	doc := load(t, `{
    "Value": {"allOf": [
      {"$ref": "#/components/schemas/ID"},
      {"oneOf": [
        {"type": "object", "properties": {"type": {"const": "number"}, "number": {"type": "number"}}},
        {"oneOf": [{"type": "object", "properties": {"type": {"const": "title"}, "title": {"type": "string"}}}]}
      ]}
    ]},
    "ID": {"type": "object", "properties": {"id": {"type": "string"}}, "required": ["id"]},
    "Holder": {"type": "object", "properties": {"id": {"$ref": "#/components/schemas/ID"}}}
  }`)

	if n := edit.MergeUnions(doc, "type"); n != 1 {
		t.Fatalf("merged %d unions, want 1", n)
	}

	want := `{"type":"object","properties":{"id":{"type":"string"},"type":{"type":"string","enum":["number","title"]},` +
		`"number":{"type":"number"},"title":{"type":"string"}},"required":["id"]}`
	if got := jsonOf(t, doc.Components.Schemas["Value"]); got != want {
		t.Errorf("got  %s\nwant %s", got, want)
	}

	if _, ok := doc.Components.Schemas["ID"]; !ok {
		t.Error("a common part something else refers to was removed")
	}
}

func TestMergeUnions_LeavesWhatItCannotMerge(t *testing.T) {
	for name, union := range map[string]string{
		"nullable":      `{"oneOf": [{"type": "object", "properties": {"type": {"const": "a"}}}, {"type": "null"}]}`,
		"no tag":        `{"oneOf": [{"type": "object", "properties": {"type": {"const": "a"}}}, {"type": "object"}]}`,
		"conflict":      `{"oneOf": [{"type": "object", "properties": {"type": {"const": "a"}, "x": {"type": "string"}}}, {"type": "object", "properties": {"type": {"const": "b"}, "x": {"type": "number"}}}]}`,
		"not an object": `{"oneOf": [{"type": "string"}, {"type": "number"}]}`,
	} {
		t.Run(name, func(t *testing.T) {
			doc := load(t, `{"U": `+union+`}`)
			before := jsonOf(t, doc.Components.Schemas["U"])

			if n := edit.MergeUnions(doc, "type"); n != 0 {
				t.Fatalf("merged %d unions, want none", n)
			}

			if got := jsonOf(t, doc.Components.Schemas["U"]); got != before {
				t.Errorf("the union changed to %s", got)
			}
		})
	}
}

func TestMergeUnion_ReturnsWhatItTookIn(t *testing.T) {
	doc := load(t, `{
    "U": {"oneOf": [{"$ref": "#/components/schemas/A"}, {"$ref": "#/components/schemas/Inner"}]},
    "Inner": {"oneOf": [{"$ref": "#/components/schemas/B"}]},
    "A": {"type": "object", "properties": {"type": {"const": "a"}}},
    "B": {"type": "object", "properties": {"type": {"const": "b"}}}
  }`)

	names, err := edit.MergeUnion(doc.Components.Schemas["U"], "type")
	if err != nil {
		t.Fatal(err)
	}

	slices.Sort(names)

	if want := []string{"A", "B", "Inner"}; !slices.Equal(names, want) {
		t.Errorf("got %v, want %v", names, want)
	}

	edit.RemoveUnreferenced(doc, names...)

	if len(doc.Components.Schemas) != 1 {
		t.Errorf("got schemas %v, want only U", doc.Components.Schemas)
	}
}

func TestMergeUnion_FailsWithoutChanging(t *testing.T) {
	doc := load(t, `{"U": {"allOf": [
    {"oneOf": [{"type": "object", "properties": {"type": {"const": "a"}}}]},
    {"oneOf": [{"type": "object", "properties": {"type": {"const": "b"}}}]}
  ]}}`)
	before := jsonOf(t, doc.Components.Schemas["U"])

	if _, err := edit.MergeUnion(doc.Components.Schemas["U"], "type"); err == nil {
		t.Fatal("merged an allOf of two unions")
	}

	if got := jsonOf(t, doc.Components.Schemas["U"]); got != before {
		t.Errorf("the union changed to %s", got)
	}
}

func TestMergeUnions_DescriptionsDiffer(t *testing.T) {
	doc := load(t, `{"Parent": {"oneOf": [
    {"type": "object", "properties": {"type": {"const": "database_id"}, "database_id": {"type": "string", "description": "The database."}}},
    {"type": "object", "properties": {"type": {"const": "data_source_id"}, "database_id": {"type": "string", "description": "The data source's database."}}}
  ]}}`)

	if n := edit.MergeUnions(doc, "type"); n != 1 {
		t.Fatalf("merged %d unions, want 1", n)
	}

	if d := doc.Components.Schemas["Parent"].Properties["database_id"].Description; d != "" {
		t.Errorf("the merged property is described as %q, true of only one variant", d)
	}
}
