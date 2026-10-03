package edit_test

import (
	"errors"
	"maps"
	"testing"

	"github.com/MarkRosemaker/openapi"
	edit "github.com/MarkRosemaker/openapi-edit"
)

func referencesDoc(t *testing.T) *openapi.Document {
	t.Helper()

	doc, err := openapi.LoadFromDataJSON([]byte(`{
  "openapi": "3.1.0",
  "info": {"title": "t", "version": "1"},
  "paths": {
    "/pages": {"get": {"responses": {"200": {"description": "ok", "content": {"application/json": {
      "schema": {"$ref": "#/components/schemas/Page"}
    }}}}}}
  },
  "components": {
    "schemas": {
      "Page": {"type": "object", "properties": {
        "url": {"$ref": "#/components/schemas/Text"},
        "name": {"$ref": "#/components/schemas/Text", "description": "The page's own name."},
        "parent": {"$ref": "#/components/schemas/Page"}
      }},
      "Text": {"type": "string", "description": "The name of the workspace."},
      "Unused": {"type": "string"}
    }
  }
}`))
	if err != nil {
		t.Fatal(err)
	}

	return doc
}

func TestCountReferences(t *testing.T) {
	got := edit.CountReferences(referencesDoc(t))

	if want := map[string]int{"Page": 2, "Text": 2}; !maps.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestDescribeReferences(t *testing.T) {
	doc := referencesDoc(t)

	if err := edit.DescribeReferences(doc, map[string]string{"Text": "Where the page lives."}); err != nil {
		t.Fatal(err)
	}

	props := doc.Components.Schemas["Page"].Properties

	// a reference without a description of its own gets the one given; one with its own keeps it
	if got, want := props["url"].Description, "Where the page lives."; got != want {
		t.Errorf("url: got %q, want %q", got, want)
	}

	if got, want := props["name"].Description, "The page's own name."; got != want {
		t.Errorf("name: got %q, want %q", got, want)
	}

	if got := props["parent"].Description; got != "" {
		t.Errorf("parent: got %q, want none", got)
	}

	// the schema itself is not changed
	if got, want := doc.Components.Schemas["Text"].Description, "The name of the workspace."; got != want {
		t.Errorf("Text: got %q, want %q", got, want)
	}

	if err := doc.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestDescribeReferences_NotFound(t *testing.T) {
	doc := referencesDoc(t)
	before := toJSON(t, doc)

	err := edit.DescribeReferences(doc, map[string]string{"Text": "x", "Missing": "y"})

	var e *edit.ErrSchemaNotFound
	if !errors.As(err, &e) || e.Name != "Missing" {
		t.Fatalf("got %v, want ErrSchemaNotFound for Missing", err)
	}

	if toJSON(t, doc) != before {
		t.Error("the document changed despite the error")
	}
}
