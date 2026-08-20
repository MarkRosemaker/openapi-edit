package edit_test

import (
	"errors"
	"slices"
	"testing"

	"github.com/MarkRosemaker/openapi"
	edit "github.com/MarkRosemaker/openapi-edit"
)

const (
	oldRef = "#/components/schemas/Old"
	newRef = "#/components/schemas/New"
)

// ref builds a reference to a component schema, resolved as the loader would
// leave it: both the reference and the schema it points at.
func ref(identifier string, v *openapi.Schema) *openapi.SchemaRef {
	return &openapi.SchemaRef{
		Ref:   &openapi.Reference{Identifier: identifier},
		Value: v,
	}
}

func doc(target *openapi.Schema) *openapi.Document {
	d := &openapi.Document{
		OpenAPI: "3.1.0",
		Info:    &openapi.Info{Title: "test", Version: "0.0.0"},
	}
	d.Components.Schemas = openapi.Schemas{}
	d.Components.Schemas.Set("Old", target)

	return d
}

func TestRenameSchema_MovesTheSchema(t *testing.T) {
	target := &openapi.Schema{Type: openapi.TypeObject}
	d := doc(target)

	if err := edit.RenameSchema(d, "Old", "New"); err != nil {
		t.Fatal(err)
	}

	if _, ok := d.Components.Schemas["Old"]; ok {
		t.Error("the old name is still present")
	}

	got, ok := d.Components.Schemas["New"]
	if !ok {
		t.Fatal("the new name is missing")
	}

	if got != target {
		t.Error("the new name holds a different schema")
	}
}

// TestRenameSchema_KeepsPosition: a rename should be a one-line change, not a
// reordering of the whole section.
func TestRenameSchema_KeepsPosition(t *testing.T) {
	d := &openapi.Document{
		OpenAPI: "3.1.0",
		Info:    &openapi.Info{Title: "test", Version: "0.0.0"},
	}
	d.Components.Schemas = openapi.Schemas{}
	d.Components.Schemas.Set("First", &openapi.Schema{Type: openapi.TypeString})
	d.Components.Schemas.Set("Old", &openapi.Schema{Type: openapi.TypeObject})
	d.Components.Schemas.Set("Last", &openapi.Schema{Type: openapi.TypeString})

	if err := edit.RenameSchema(d, "Old", "New"); err != nil {
		t.Fatal(err)
	}

	var order []string
	for name := range d.Components.Schemas.ByIndex() {
		order = append(order, name)
	}

	if want := []string{"First", "New", "Last"}; !slices.Equal(order, want) {
		t.Errorf("order = %v, want %v", order, want)
	}
}

// TestRenameSchema_RewritesRefsEverywhere is the point of the function: a
// reference to the renamed schema has to be found wherever it sits.
func TestRenameSchema_RewritesRefsEverywhere(t *testing.T) {
	target := &openapi.Schema{Type: openapi.TypeObject}

	for _, tc := range []struct {
		name string
		// build places a reference to the renamed schema somewhere in d, and
		// returns the reference so the test can check it afterwards.
		build func(d *openapi.Document) *openapi.SchemaRef
	}{{
		name: "a property of another component schema",
		build: func(d *openapi.Document) *openapi.SchemaRef {
			r := ref(oldRef, target)
			parent := &openapi.Schema{Type: openapi.TypeObject, Properties: openapi.SchemaRefs{}}
			parent.Properties.Set("child", r)
			d.Components.Schemas.Set("Parent", parent)

			return r
		},
	}, {
		name: "an allOf branch",
		build: func(d *openapi.Document) *openapi.SchemaRef {
			r := ref(oldRef, target)
			d.Components.Schemas.Set("Parent", &openapi.Schema{
				Type: openapi.TypeObject, AllOf: openapi.SchemaRefList{r},
			})

			return r
		},
	}, {
		name: "a oneOf branch",
		build: func(d *openapi.Document) *openapi.SchemaRef {
			r := ref(oldRef, target)
			d.Components.Schemas.Set("Parent", &openapi.Schema{OneOf: openapi.SchemaRefList{r}})

			return r
		},
	}, {
		name: "an anyOf branch",
		build: func(d *openapi.Document) *openapi.SchemaRef {
			r := ref(oldRef, target)
			d.Components.Schemas.Set("Parent", &openapi.Schema{AnyOf: openapi.SchemaRefList{r}})

			return r
		},
	}, {
		name: "a not branch",
		build: func(d *openapi.Document) *openapi.SchemaRef {
			r := ref(oldRef, target)
			d.Components.Schemas.Set("Parent", &openapi.Schema{Not: r})

			return r
		},
	}, {
		name: "array items",
		build: func(d *openapi.Document) *openapi.SchemaRef {
			r := ref(oldRef, target)
			d.Components.Schemas.Set("Parent", &openapi.Schema{Type: openapi.TypeArray, Items: r})

			return r
		},
	}, {
		name: "additionalProperties",
		build: func(d *openapi.Document) *openapi.SchemaRef {
			r := ref(oldRef, target)
			d.Components.Schemas.Set("Parent", &openapi.Schema{
				Type: openapi.TypeObject, AdditionalProperties: r,
			})

			return r
		},
	}, {
		name: "a response body in an operation",
		build: func(d *openapi.Document) *openapi.SchemaRef {
			r := ref(oldRef, target)
			op := &openapi.Operation{Responses: openapi.OperationResponses{}}
			op.Responses.Set("200", &openapi.ResponseRef{Value: &openapi.Response{
				Description: "OK",
				Content: openapi.Content{
					"application/json": &openapi.MediaType{Schema: r},
				},
			}})
			d.Paths = openapi.Paths{"/thing": {Get: op}}

			return r
		},
	}, {
		name: "a request body in an operation",
		build: func(d *openapi.Document) *openapi.SchemaRef {
			r := ref(oldRef, target)
			d.Paths = openapi.Paths{"/thing": {Post: &openapi.Operation{
				RequestBody: &openapi.RequestBodyRef{Value: &openapi.RequestBody{
					Content: openapi.Content{
						"application/json": &openapi.MediaType{Schema: r},
					},
				}},
			}}}

			return r
		},
	}, {
		name: "a property of a parameter's schema",
		build: func(d *openapi.Document) *openapi.SchemaRef {
			r := ref(oldRef, target)
			ps := &openapi.Schema{Type: openapi.TypeObject, Properties: openapi.SchemaRefs{}}
			ps.Properties.Set("child", r)
			d.Paths = openapi.Paths{"/thing": {Get: &openapi.Operation{
				Parameters: openapi.ParameterList{{Value: &openapi.Parameter{
					Name: "q", In: openapi.ParameterLocationQuery, Schema: ps,
				}}},
			}}}

			return r
		},
	}, {
		name: "a parameter shared across a path item",
		build: func(d *openapi.Document) *openapi.SchemaRef {
			r := ref(oldRef, target)
			d.Paths = openapi.Paths{"/thing": {
				Parameters: openapi.ParameterList{{Value: &openapi.Parameter{
					Name: "body", In: openapi.ParameterLocationQuery,
					Content: openapi.Content{
						"application/json": &openapi.MediaType{Schema: r},
					},
				}}},
			}}

			return r
		},
	}, {
		name: "a response header",
		build: func(d *openapi.Document) *openapi.SchemaRef {
			r := ref(oldRef, target)
			op := &openapi.Operation{Responses: openapi.OperationResponses{}}
			op.Responses.Set("200", &openapi.ResponseRef{Value: &openapi.Response{
				Description: "OK",
				Headers: openapi.Headers{"X-Thing": {Value: &openapi.Header{
					Content: openapi.Content{
						"application/json": &openapi.MediaType{Schema: r},
					},
				}}},
			}})
			d.Paths = openapi.Paths{"/thing": {Get: op}}

			return r
		},
	}, {
		name: "an encoding header",
		build: func(d *openapi.Document) *openapi.SchemaRef {
			r := ref(oldRef, target)
			d.Components.RequestBodies = openapi.RequestBodies{}
			d.Components.RequestBodies.Set("Body", &openapi.RequestBodyRef{
				Value: &openapi.RequestBody{Content: openapi.Content{
					"multipart/form-data": &openapi.MediaType{
						Encoding: openapi.Encodings{"part": &openapi.Encoding{
							Headers: openapi.Headers{"X-Thing": {Value: &openapi.Header{
								Content: openapi.Content{
									"application/json": &openapi.MediaType{Schema: r},
								},
							}}},
						}},
					},
				}},
			})

			return r
		},
	}, {
		name: "a callback on an operation",
		build: func(d *openapi.Document) *openapi.SchemaRef {
			r := ref(oldRef, target)
			cbOp := &openapi.Operation{
				RequestBody: &openapi.RequestBodyRef{Value: &openapi.RequestBody{
					Content: openapi.Content{
						"application/json": &openapi.MediaType{Schema: r},
					},
				}},
			}
			d.Paths = openapi.Paths{"/thing": {Get: &openapi.Operation{
				Callbacks: openapi.Callbacks{
					"onEvent": openapi.Callback{"{$request.body#/url}": {
						Value: &openapi.PathItem{Post: cbOp},
					}},
				},
			}}}

			return r
		},
	}, {
		name: "a webhook",
		build: func(d *openapi.Document) *openapi.SchemaRef {
			r := ref(oldRef, target)
			d.Webhooks = openapi.Webhooks{"onThing": {Value: &openapi.PathItem{
				Post: &openapi.Operation{
					RequestBody: &openapi.RequestBodyRef{Value: &openapi.RequestBody{
						Content: openapi.Content{
							"application/json": &openapi.MediaType{Schema: r},
						},
					}},
				},
			}}}

			return r
		},
	}, {
		name: "a component response",
		build: func(d *openapi.Document) *openapi.SchemaRef {
			r := ref(oldRef, target)
			d.Components.Responses = openapi.ResponsesByName{}
			d.Components.Responses.Set("Thing", &openapi.ResponseRef{Value: &openapi.Response{
				Description: "OK",
				Content: openapi.Content{
					"application/json": &openapi.MediaType{Schema: r},
				},
			}})

			return r
		},
	}, {
		name: "a component path item",
		build: func(d *openapi.Document) *openapi.SchemaRef {
			r := ref(oldRef, target)
			d.Components.PathItems = openapi.PathItems{}
			d.Components.PathItems.Set("Thing", &openapi.PathItemRef{Value: &openapi.PathItem{
				Get: &openapi.Operation{
					RequestBody: &openapi.RequestBodyRef{Value: &openapi.RequestBody{
						Content: openapi.Content{
							"application/json": &openapi.MediaType{Schema: r},
						},
					}},
				},
			}})

			return r
		},
	}} {
		t.Run(tc.name, func(t *testing.T) {
			d := doc(target)
			r := tc.build(d)

			if err := edit.RenameSchema(d, "Old", "New"); err != nil {
				t.Fatal(err)
			}

			if r.Ref.Identifier != newRef {
				t.Errorf("reference = %q, want %q", r.Ref.Identifier, newRef)
			}
		})
	}
}

// TestRenameSchema_LeavesOtherRefsAlone: only references to the renamed schema
// change.
func TestRenameSchema_LeavesOtherRefsAlone(t *testing.T) {
	const otherRef = "#/components/schemas/Other"

	d := doc(&openapi.Schema{Type: openapi.TypeObject})
	other := ref(otherRef, &openapi.Schema{Type: openapi.TypeString})
	parent := &openapi.Schema{Type: openapi.TypeObject, Properties: openapi.SchemaRefs{}}
	parent.Properties.Set("other", other)
	d.Components.Schemas.Set("Parent", parent)

	if err := edit.RenameSchema(d, "Old", "New"); err != nil {
		t.Fatal(err)
	}

	if other.Ref.Identifier != otherRef {
		t.Errorf("unrelated reference changed to %q", other.Ref.Identifier)
	}
}

// TestRenameSchema_Cycle: a schema referring to itself must not loop forever.
func TestRenameSchema_Cycle(t *testing.T) {
	target := &openapi.Schema{Type: openapi.TypeObject, Properties: openapi.SchemaRefs{}}
	self := ref(oldRef, target)
	target.Properties.Set("self", self)

	d := doc(target)

	if err := edit.RenameSchema(d, "Old", "New"); err != nil {
		t.Fatal(err)
	}

	if self.Ref.Identifier != newRef {
		t.Errorf("self reference = %q, want %q", self.Ref.Identifier, newRef)
	}
}

func TestRenameSchema_SameName(t *testing.T) {
	target := &openapi.Schema{Type: openapi.TypeObject}
	d := doc(target)

	if err := edit.RenameSchema(d, "Old", "Old"); err != nil {
		t.Fatalf("renaming to the same name should do nothing, got %v", err)
	}

	if d.Components.Schemas["Old"] != target {
		t.Error("the schema was disturbed")
	}
}

func TestRenameSchema_Errors(t *testing.T) {
	notFound := func(err error) bool {
		var e *edit.ErrSchemaNotFound
		return errors.As(err, &e)
	}
	exists := func(err error) bool {
		var e *edit.ErrSchemaExists
		return errors.As(err, &e)
	}
	invalid := func(err error) bool {
		var e *edit.ErrInvalidSchemaName
		return errors.As(err, &e)
	}

	for _, tc := range []struct {
		name     string
		old, new string
		want     func(error) bool
	}{{
		name: "the schema does not exist",
		old:  "Missing", new: "New",
		want: notFound,
	}, {
		name: "the new name is taken",
		old:  "Old", new: "Taken",
		want: exists,
	}, {
		name: "the new name could not be referenced",
		old:  "Old", new: "Not A Name",
		want: invalid,
	}, {
		name: "the new name contains a slash",
		old:  "Old", new: "a/b",
		want: invalid,
	}, {
		name: "the new name is empty",
		old:  "Old", new: "",
		want: invalid,
	}} {
		t.Run(tc.name, func(t *testing.T) {
			target := &openapi.Schema{Type: openapi.TypeObject}
			d := doc(target)
			d.Components.Schemas.Set("Taken", &openapi.Schema{Type: openapi.TypeString})

			err := edit.RenameSchema(d, tc.old, tc.new)
			if err == nil {
				t.Fatal("expected an error")
			}

			if !tc.want(err) {
				t.Errorf("unexpected error type %T: %v", err, err)
			}

			// A failed rename must change nothing at all.
			if d.Components.Schemas["Old"] != target {
				t.Error("the schema moved despite the error")
			}
		})
	}
}

func TestRenameSchema_NilDocument(t *testing.T) {
	if err := edit.RenameSchema(nil, "Old", "New"); err != nil {
		t.Errorf("nil document should be a no-op, got %v", err)
	}
}
