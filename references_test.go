package edit_test

import (
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
