package edit_test

import (
	"encoding/json/v2"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/MarkRosemaker/openapi"
	edit "github.com/MarkRosemaker/openapi-edit"
)

// parse reads a document of the paths and schemas given, as JSON.
func parse(t *testing.T, paths, schemas string) *openapi.Document {
	t.Helper()

	doc, err := openapi.LoadFromData([]byte(`{"openapi":"3.1.0","info":{"title":"t","version":"1"},"paths":` + paths +
		`,"components":{"schemas":` + schemas + `}}`))
	if err != nil {
		t.Fatal(err)
	}

	return doc
}

// written is the schema as it is written, compact.
func written(t *testing.T, s *openapi.Schema) string {
	t.Helper()

	b, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}

	return string(b)
}

func TestSchemaRef(t *testing.T) {
	doc := parse(t, `{}`, `{"Pet":{"type":"object"}}`)

	ref := edit.SchemaRef(doc, "Pet")
	if ref.Ref.Identifier != "#/components/schemas/Pet" || ref.Ref.Value != doc.Components.Schemas["Pet"] {
		t.Errorf("got %+v", ref.Ref)
	}
}

func TestMoveToComponents(t *testing.T) {
	doc := parse(t, `{}`, `{
		"Pet":{"type":"object","properties":{"owner":{"type":"object","properties":{"name":{"type":"string"}}},"kind":{"$ref":"#/components/schemas/Kind"}}},
		"Kind":{"type":"string"}
	}`)
	owner := doc.Components.Schemas["Pet"].Properties["owner"]

	moved, ok := edit.MoveToComponents(doc, owner, "Owner")
	if !ok || doc.Components.Schemas["Owner"] != moved || moved.Properties["name"] == nil {
		t.Fatalf("got %v, %+v", ok, moved)
	}

	if owner.Ref == nil || owner.Ref.Identifier != "#/components/schemas/Owner" || owner.Ref.Value != moved {
		t.Errorf("left %s in its place", written(t, owner))
	}

	for _, tc := range []struct {
		name string
		s    *openapi.Schema
		as   string
	}{
		{"a reference", doc.Components.Schemas["Pet"].Properties["kind"], "Other"},
		{"a name taken", &openapi.Schema{Type: openapi.TypeString}, "Kind"},
		{"a name no reference could have", &openapi.Schema{Type: openapi.TypeString}, "no/name"},
	} {
		if _, ok := edit.MoveToComponents(doc, tc.s, tc.as); ok {
			t.Errorf("%s: moved", tc.name)
		}
	}
}

func TestRenameSchemasFunc(t *testing.T) {
	doc := parse(t, `{}`, `{
		"PetRequest":{"type":"object","properties":{"owner":{"$ref":"#/components/schemas/OwnerRequest"}}},
		"OwnerRequest":{"type":"object"},
		"Kind":{"type":"string"}
	}`)

	if err := edit.RenameSchemasFunc(doc, func(n string) string { return strings.TrimSuffix(n, "Request") }); err != nil {
		t.Fatal(err)
	}

	if got := slices.Collect(func(yield func(string) bool) {
		for n := range doc.Components.Schemas.ByIndex() {
			if !yield(n) {
				return
			}
		}
	}); !slices.Equal(got, []string{"Pet", "Owner", "Kind"}) {
		t.Errorf("got %q", got)
	}

	if ref := doc.Components.Schemas["Pet"].Properties["owner"].Ref; ref.Identifier != "#/components/schemas/Owner" {
		t.Errorf("reference left at %s", ref.Identifier)
	}

	doc = parse(t, `{}`, `{"PetRequest":{"type":"object"},"Pet":{"type":"object"}}`)

	var exists *edit.ErrSchemaExists
	if err := edit.RenameSchemasFunc(doc, func(n string) string { return strings.TrimSuffix(n, "Request") }); !errors.As(err, &exists) {
		t.Errorf("onto a name taken: got %v", err)
	}
}

func TestMergeSchemas(t *testing.T) {
	const schemas = `{
		"Holder":{"type":"object","properties":{"a":{"$ref":"#/components/schemas/SizeA"},"b":{"$ref":"#/components/schemas/SizeB"}}},
		"SizeA":{"type":"integer","maximum":10},
		"SizeB":{"type":"integer","maximum":20}
	}`

	t.Run("under a new name", func(t *testing.T) {
		doc := parse(t, `{}`, schemas)
		merged := &openapi.Schema{Type: openapi.TypeInteger}

		if err := edit.MergeSchemas(doc, "Size", merged, "SizeA", "SizeB"); err != nil {
			t.Fatal(err)
		}

		holder := doc.Components.Schemas["Holder"]
		for _, p := range []string{"a", "b"} {
			if ref := holder.Properties[p].Ref; ref.Identifier != "#/components/schemas/Size" || ref.Value != merged {
				t.Errorf("%s refers to %s", p, ref.Identifier)
			}
		}

		if doc.Components.Schemas["SizeA"] != nil || doc.Components.Schemas["SizeB"] != nil {
			t.Error("the members are still there")
		}
	})

	t.Run("under a member's name", func(t *testing.T) {
		doc := parse(t, `{}`, schemas)
		merged := &openapi.Schema{Type: openapi.TypeInteger}

		if err := edit.MergeSchemas(doc, "SizeA", merged, "SizeA", "SizeB"); err != nil {
			t.Fatal(err)
		}

		holder := doc.Components.Schemas["Holder"]
		for _, p := range []string{"a", "b"} {
			if ref := holder.Properties[p].Ref; ref.Identifier != "#/components/schemas/SizeA" || ref.Value != merged {
				t.Errorf("%s refers to %s", p, ref.Identifier)
			}
		}

		if doc.Components.Schemas["SizeA"] != merged || doc.Components.Schemas["SizeB"] != nil {
			t.Error("not merged in the member's place")
		}
	})

	t.Run("fails", func(t *testing.T) {
		doc := parse(t, `{}`, schemas)
		merged := &openapi.Schema{Type: openapi.TypeInteger}

		var notFound *edit.ErrSchemaNotFound
		if err := edit.MergeSchemas(doc, "Size", merged, "SizeA", "SizeC"); !errors.As(err, &notFound) {
			t.Errorf("a member not there: got %v", err)
		}

		var exists *edit.ErrSchemaExists
		if err := edit.MergeSchemas(doc, "Holder", merged, "SizeA", "SizeB"); !errors.As(err, &exists) {
			t.Errorf("a name taken: got %v", err)
		}

		var invalid *edit.ErrInvalidSchemaName
		if err := edit.MergeSchemas(doc, "no/name", merged, "SizeA"); !errors.As(err, &invalid) {
			t.Errorf("a name no reference could have: got %v", err)
		}

		if doc.Components.Schemas["Size"] != nil || doc.Components.Schemas["SizeA"] == nil {
			t.Error("changed the document")
		}
	})
}

func TestRemoveNullAlternatives(t *testing.T) {
	doc := parse(t, `{}`, `{
		"Pet":{"type":"object","properties":{
			"name":{"title":"Name","description":"what it is called","default":"Rex","anyOf":[{"type":"string","maxLength":9},{"type":"null"}]},
			"owner":{"description":"who has it","example":{"id":1},"anyOf":[{"$ref":"#/components/schemas/Owner"},{"type":"null"}]},
			"tag":{"anyOf":[{"type":"string"},{"type":"integer"},{"type":"null"}]},
			"size":{"anyOf":[{"type":"string"},{"type":"integer"}]}
		}},
		"Owner":{"type":"object","properties":{"id":{"type":"integer"}}}
	}`)

	edit.RemoveNullAlternatives(doc)

	props := doc.Components.Schemas["Pet"].Properties
	for prop, want := range map[string]string{
		"name":  `{"title":"Name","description":"what it is called","type":"string","maxLength":9,"default":"Rex"}`,
		"owner": `{"description":"who has it","$ref":"#/components/schemas/Owner"}`,
		"tag":   `{"anyOf":[{"type":"string"},{"type":"integer"}]}`,
		"size":  `{"anyOf":[{"type":"string"},{"type":"integer"}]}`,
	} {
		if got := written(t, props[prop]); got != want {
			t.Errorf("%s: got  %s\nwant %s", prop, got, want)
		}
	}

	if ex := string(doc.Components.Schemas["Owner"].Example); ex != `{"id":1}` {
		t.Errorf("Owner has the example %s, want the one of the anyOf", ex)
	}
}

func TestRemoveEmptyJSON(t *testing.T) {
	doc := parse(t, `{
		"/download":{"get":{"responses":{
			"200":{"description":"a zip","content":{"application/json":{"schema":{}},"application/zip":{"schema":{"type":"string","format":"binary"}}}},
			"201":{"description":"a file without a schema","content":{"application/json; charset=utf-8":{"schema":{}},"application/octet-stream":{}}},
			"202":{"description":"JSON of its own","content":{"application/json":{"schema":{"type":"object"}},"text/plain":{"schema":{}}}},
			"204":{"description":"nothing else","content":{"application/json":{"schema":{}}}}
		}}},
		"/other":{"get":{"responses":{"200":{"description":"no content"}}}}
	}`, `{}`)

	edit.RemoveEmptyJSON(doc)

	content := map[string][]string{}
	for code, rsp := range doc.Paths["/download"].Get.Responses.ByIndex() {
		for mr := range rsp.Value.Content {
			content[string(code)] = append(content[string(code)], string(mr))
		}

		slices.Sort(content[string(code)])
	}

	for code, want := range map[string][]string{
		"200": {"application/zip"},
		"201": {"application/octet-stream"},
		"202": {"application/json", "text/plain"},
		"204": {"application/json"},
	} {
		if !slices.Equal(content[code], want) {
			t.Errorf("%s: got %q, want %q", code, content[code], want)
		}
	}
}
