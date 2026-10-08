package edit_test

import (
	"bytes"
	"encoding/json/jsontext"
	"errors"
	"maps"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/MarkRosemaker/openapi"
	edit "github.com/MarkRosemaker/openapi-edit"
)

// load reads a document from testdata.
func load(t *testing.T, name string) *openapi.Document {
	t.Helper()

	doc, err := openapi.LoadFromFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}

	return doc
}

// toJSON is the document as it is written, indented as testdata is.
func toJSON(t *testing.T, doc *openapi.Document) []byte {
	t.Helper()

	b, err := doc.ToJSON()
	if err != nil {
		t.Fatal(err)
	}

	v := jsontext.Value(b)
	if err := v.Indent(jsontext.WithIndent("  ")); err != nil {
		t.Fatal(err)
	}

	return append(v, '\n')
}

// isList accepts what the golden document titles a list.
func isList(s *openapi.Schema) bool {
	return s.Type == openapi.TypeArray && strings.HasPrefix(s.Title, "List")
}

// isNested accepts what the golden document titles a nested list.
func isNested(s *openapi.Schema) bool { return s.Type == openapi.TypeArray && s.Title == "Nested" }

// TestEdit_Golden applies each edit to testdata/before.json, which must then be testdata/after.json. Both are edited by
// hand: before.json has a group of schemas for each thing an edit does, described in it, and after.json what it does.
func TestEdit_Golden(t *testing.T) {
	t.Parallel()

	doc := load(t, "before.json")

	for _, step := range []struct {
		name string
		do   func() error
	}{
		{"rename", func() error {
			return edit.RenameSchemas(doc, map[string]string{
				"First": "First", "Old": "New", "Cat": "Kitty", "Dog": "Hound", "Car": "Automobile", "Left": "Right", "Right": "Left",
			})
		}},
		{"redirect with a description", func() error { return edit.RedirectSchema(doc, "Copy", "Original", "was Copy") }},
		{"redirect", func() error { return edit.RedirectSchemas(doc, map[string]string{"Twin": "Original", "Last": "Last"}) }},
		{"extract", func() error { return edit.ExtractSchema(doc, "List", isList) }},
		{"extract nested", func() error { return edit.ExtractSchema(doc, "Rows", isNested) }},
		{"trim examples", func() error { return edit.TrimSchemaExamples(doc, 2) }},
		{"describe", func() error {
			return edit.DescribeReferences(doc, map[string]string{"Text": "Where the page lives."})
		}},
		{"remove unreferenced", func() error {
			removed := edit.RemoveUnreferenced(doc, "Inner", "Unused", "Named", "Missing")
			if want := []string{"Unused", "Inner"}; !slices.Equal(removed, want) {
				t.Errorf("removed %v, want %v", removed, want)
			}

			return nil
		}},
	} {
		if err := step.do(); err != nil {
			t.Fatalf("%s: %v", step.name, err)
		}
	}

	if err := doc.Validate(); err != nil {
		t.Fatal(err)
	}

	got := toJSON(t, doc)

	want, err := os.ReadFile("testdata/after.json")
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(got, want) {
		gotLines, wantLines := bytes.Split(got, []byte("\n")), bytes.Split(want, []byte("\n"))
		for i := range min(len(gotLines), len(wantLines)) {
			if !bytes.Equal(gotLines[i], wantLines[i]) {
				t.Fatalf("after.json line %d: got %s, want %s", i+1, bytes.TrimSpace(gotLines[i]), bytes.TrimSpace(wantLines[i]))
			}
		}

		t.Fatalf("got %d lines, want %d", len(gotLines), len(wantLines))
	}
}

// TestEdit_Errors: an edit that cannot be made fails, changing nothing.
func TestEdit_Errors(t *testing.T) {
	t.Parallel()

	notFound := func(err error) bool { _, ok := errors.AsType[*edit.ErrSchemaNotFound](err); return ok }
	exists := func(err error) bool { _, ok := errors.AsType[*edit.ErrSchemaExists](err); return ok }
	invalid := func(err error) bool { _, ok := errors.AsType[*edit.ErrInvalidSchemaName](err); return ok }
	noMatch := func(err error) bool { return errors.Is(err, edit.ErrNoMatch) }

	for _, tc := range []struct {
		name string
		edit func(*openapi.Document) error
		want func(error) bool
	}{
		{"renaming a schema that does not exist", func(d *openapi.Document) error {
			return edit.RenameSchema(d, "Missing", "New")
		}, notFound},
		{"renaming onto a name that is taken", func(d *openapi.Document) error {
			return edit.RenameSchema(d, "Old", "Last")
		}, exists},
		{"renaming two schemas to one name", func(d *openapi.Document) error {
			return edit.RenameSchemas(d, map[string]string{"Cat": "Kitty", "Dog": "Kitty"})
		}, exists},
		{"renaming onto a schema that stays", func(d *openapi.Document) error {
			return edit.RenameSchemas(d, map[string]string{"Cat": "Kitty", "Dog": "Bird"})
		}, exists},
		{"renaming to a name that cannot be referenced", func(d *openapi.Document) error {
			return edit.RenameSchema(d, "Old", "Not A Name")
		}, invalid},
		{"renaming to a name with a slash", func(d *openapi.Document) error {
			return edit.RenameSchema(d, "Old", "a/b")
		}, invalid},
		{"renaming to no name", func(d *openapi.Document) error {
			return edit.RenameSchema(d, "Old", "")
		}, invalid},
		{"redirecting a schema that does not exist", func(d *openapi.Document) error {
			return edit.RedirectSchemas(d, map[string]string{"Copy": "Original", "Missing": "Original"})
		}, notFound},
		{"redirecting onto a schema that does not exist", func(d *openapi.Document) error {
			return edit.RedirectSchema(d, "Copy", "Missing", "")
		}, notFound},
		{"redirecting onto a schema that is itself redirected", func(d *openapi.Document) error {
			return edit.RedirectSchemas(d, map[string]string{"Copy": "Twin", "Twin": "Original"})
		}, notFound},
		{"extracting what nothing matches", func(d *openapi.Document) error {
			return edit.ExtractSchema(d, "List", func(*openapi.Schema) bool { return false })
		}, noMatch},
		{"extracting onto a name that is taken", func(d *openapi.Document) error {
			return edit.ExtractSchema(d, "Lists", isList)
		}, exists},
		{"extracting to a name that cannot be referenced", func(d *openapi.Document) error {
			return edit.ExtractSchema(d, "Some Lists", isList)
		}, invalid},
		{"describing a schema that does not exist", func(d *openapi.Document) error {
			return edit.DescribeReferences(d, map[string]string{"Text": "x", "Missing": "y"})
		}, notFound},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			doc := load(t, "before.json")
			before := toJSON(t, doc)

			if err := tc.edit(doc); !tc.want(err) {
				t.Fatalf("got error %v", err)
			}

			if !bytes.Equal(toJSON(t, doc), before) {
				t.Error("the document changed")
			}
		})
	}
}

// TestEdit_SameAsOneByOne: renaming or redirecting many schemas at once walks the document a fixed number of times,
// and gives what an edit of each in turn gives.
func TestEdit_SameAsOneByOne(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		to    map[string]string
		one   func(*openapi.Document, string, string) error
		batch func(*openapi.Document, map[string]string) error
	}{
		"rename": {
			map[string]string{"Old": "New", "Cat": "Kitty", "Dog": "Hound", "Car": "Automobile", "First": "First"},
			edit.RenameSchema, edit.RenameSchemas,
		},
		"redirect": {
			map[string]string{"Copy": "Original", "Twin": "Original", "Last": "Last"},
			func(d *openapi.Document, oldName, newName string) error {
				return edit.RedirectSchema(d, oldName, newName, "")
			},
			edit.RedirectSchemas,
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			want := load(t, "before.json")
			for _, oldName := range slices.Sorted(maps.Keys(tc.to)) {
				if err := tc.one(want, oldName, tc.to[oldName]); err != nil {
					t.Fatal(err)
				}
			}

			got := load(t, "before.json")
			if err := tc.batch(got, tc.to); err != nil {
				t.Fatal(err)
			}

			if !bytes.Equal(toJSON(t, got), toJSON(t, want)) {
				t.Error("the batch differs from one edit after another")
			}
		})
	}
}

// TestExtractSchema_FirstInDocumentOrder: what tells matches apart comes from the first in the document every time,
// which a single run, as the golden test makes, could get right by chance.
func TestExtractSchema_FirstInDocumentOrder(t *testing.T) {
	t.Parallel()

	for range 20 {
		doc := load(t, "before.json")
		if err := edit.ExtractSchema(doc, "List", isList); err != nil {
			t.Fatal(err)
		}

		if got := doc.Components.Schemas["List"].Title; got != "List from Lists" {
			t.Fatalf("the component is %q, want the first list in the document", got)
		}
	}
}
