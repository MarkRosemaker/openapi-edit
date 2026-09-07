package edit_test

import (
	"errors"
	"testing"

	"github.com/MarkRosemaker/openapi"
	edit "github.com/MarkRosemaker/openapi-edit"
)

// mergeDoc builds a document with two component schemas, "Old" and "New",
// and a "Parent" schema whose "child" property points at "Old".
func mergeDoc() (d *openapi.Document, child *openapi.SchemaRef) {
	d = &openapi.Document{
		OpenAPI: "3.1.0",
		Info:    &openapi.Info{Title: "test", Version: "0.0.0"},
	}
	d.Components.Schemas = openapi.Schemas{}
	d.Components.Schemas.Set("Old", &openapi.Schema{Type: openapi.TypeObject})
	d.Components.Schemas.Set("New", &openapi.Schema{Type: openapi.TypeString})

	child = ref(oldRef, d.Components.Schemas["Old"])
	parent := &openapi.Schema{Type: openapi.TypeObject, Properties: openapi.SchemaRefs{}}
	parent.Properties.Set("child", child)
	d.Components.Schemas.Set("Parent", parent)

	return d, child
}

func TestMergeSchema_RepointsRefsAndDeletesOld(t *testing.T) {
	d, child := mergeDoc()
	newSchema := d.Components.Schemas["New"]

	if err := edit.MergeSchema(d, "Old", "New", ""); err != nil {
		t.Fatal(err)
	}

	if _, ok := d.Components.Schemas["Old"]; ok {
		t.Error("the old schema is still present")
	}

	if got := d.Components.Schemas["New"]; got != newSchema {
		t.Error("the new schema's own definition was disturbed")
	}

	if child.Ref.Identifier != newRef {
		t.Errorf("reference = %q, want %q", child.Ref.Identifier, newRef)
	}
}

func TestMergeSchema_SetsDescriptionOnRepointedRefs(t *testing.T) {
	d, child := mergeDoc()

	if err := edit.MergeSchema(d, "Old", "New", "was Old"); err != nil {
		t.Fatal(err)
	}

	if child.Ref.Description != "was Old" {
		t.Errorf("description = %q, want %q", child.Ref.Description, "was Old")
	}
}

func TestMergeSchema_EmptyDescriptionLeavesExistingOneAlone(t *testing.T) {
	d, child := mergeDoc()
	child.Ref.Description = "already set"

	if err := edit.MergeSchema(d, "Old", "New", ""); err != nil {
		t.Fatal(err)
	}

	if child.Ref.Description != "already set" {
		t.Errorf("description = %q, want it left alone", child.Ref.Description)
	}
}

func TestMergeSchema_LeavesOtherRefsAlone(t *testing.T) {
	const otherRef = "#/components/schemas/Other"

	d, _ := mergeDoc()
	other := ref(otherRef, &openapi.Schema{Type: openapi.TypeBoolean})
	d.Components.Schemas["Parent"].Properties.Set("other", other)

	if err := edit.MergeSchema(d, "Old", "New", ""); err != nil {
		t.Fatal(err)
	}

	if other.Ref.Identifier != otherRef {
		t.Errorf("unrelated reference changed to %q", other.Ref.Identifier)
	}
}

func TestMergeSchema_LeavesInlineSchemasAlone(t *testing.T) {
	d, _ := mergeDoc()
	inline := &openapi.SchemaRef{Value: &openapi.Schema{Type: openapi.TypeObject}}
	d.Components.Schemas["Parent"].Properties.Set("inline", inline)

	if err := edit.MergeSchema(d, "Old", "New", "description"); err != nil {
		t.Fatal(err)
	}

	if inline.Ref != nil {
		t.Errorf("an inline schema grew a $ref: %+v", inline.Ref)
	}
}

func TestMergeSchema_SameName(t *testing.T) {
	d, child := mergeDoc()

	if err := edit.MergeSchema(d, "Old", "Old", "description"); err != nil {
		t.Fatalf("merging a schema into itself should do nothing, got %v", err)
	}

	if _, ok := d.Components.Schemas["Old"]; !ok {
		t.Error("the schema was removed")
	}

	if child.Ref.Identifier != oldRef {
		t.Error("the reference was disturbed")
	}

	if child.Ref.Description != "" {
		t.Error("the description was set despite the no-op")
	}
}

func TestMergeSchema_Errors(t *testing.T) {
	notFound := func(err error) bool {
		var e *edit.ErrSchemaNotFound
		return errors.As(err, &e)
	}

	for _, tc := range []struct {
		name     string
		old, new string
	}{
		{name: "the old schema does not exist", old: "Missing", new: "New"},
		{name: "the new schema does not exist", old: "Old", new: "Missing"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d, child := mergeDoc()

			err := edit.MergeSchema(d, tc.old, tc.new, "")
			if err == nil {
				t.Fatal("expected an error")
			}

			if !notFound(err) {
				t.Errorf("unexpected error type %T: %v", err, err)
			}

			// A failed merge must change nothing at all.
			if _, ok := d.Components.Schemas["Old"]; !ok {
				t.Error("the old schema was removed despite the error")
			}

			if child.Ref.Identifier != oldRef {
				t.Error("the reference was rewritten despite the error")
			}
		})
	}
}
