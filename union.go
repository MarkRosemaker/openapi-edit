package edit

import (
	"bytes"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
	"fmt"
	"strings"

	"github.com/MarkRosemaker/openapi"
)

// MergeUnion turns s, a union of objects told apart by the property tag, into one object: the properties every
// variant has, tag as an enum of the variants' values, and each variant's own properties, optional.
//
// The union is s's oneOf, or else its anyOf, and may nest unions; its variants may be written inline or be
// references. s may instead be an allOf of objects and one such union, the objects holding what every variant shares:
// their properties come first and stay as required as they were.
//
// It returns the names of the component schemas the merge took in -- the variants, the unions within it and the
// parts of the allOf it refers to -- for [RemoveUnreferenced] to remove once nothing refers to them anymore.
//
// It fails, changing nothing, if a variant is neither an object nor a union, has no single value for tag, or has a
// property another one has too, but differently.
func MergeUnion(s *openapi.Schema, tag string) ([]string, error) {
	common, union, names, err := splitAllOf(s)
	if err != nil {
		return nil, err
	}

	variants, merged, err := leaves(alternatives(union))
	if err != nil {
		return nil, err
	}

	if len(variants) == 0 {
		return nil, errors.New("not a union")
	}

	m := &unionMerge{tag: tag, seen: map[string][]byte{}, shared: map[string]int{}, required: map[string]int{}}

	for _, c := range common {
		if err := m.addCommon(c, len(variants)); err != nil {
			return nil, err
		}
	}

	for _, v := range variants {
		if err := m.addVariant(v); err != nil {
			return nil, err
		}
	}

	props := m.properties(common, variants)

	s.Replace(&openapi.Schema{
		Title:       s.Title,
		Description: s.Description,
		Type:        openapi.TypeObject,
		Properties:  props,
		Required:    m.requiredOf(props, len(variants)),
	})

	return append(names, merged...), nil
}

// MergeUnions merges, with [MergeUnion], every union in doc it can, and removes the component schemas it merged that
// nothing refers to anymore. A union MergeUnion cannot merge is left as it is. It returns how many it merged.
func MergeUnions(doc *openapi.Document, tag string) int {
	var merged []string

	n := 0

	walkSchemas(doc, func(s *openapi.Schema) {
		if s.Ref != nil || len(s.AllOf) == 0 && len(alternatives(s)) == 0 {
			return
		}

		if names, err := MergeUnion(s, tag); err == nil {
			merged = append(merged, names...)
			n++
		}
	})

	RemoveUnreferenced(doc, merged...)

	return n
}

// RemoveUnreferenced removes those of the schemas names from components.schemas that nothing refers to. It repeats
// until each that remains is referred to, since removing one can leave another without a reference.
func RemoveUnreferenced(doc *openapi.Document, names ...string) {
	for removed := true; removed; {
		removed = false
		counts := CountReferences(doc)

		for _, n := range names {
			if _, ok := doc.Components.Schemas[n]; ok && counts[n] == 0 {
				delete(doc.Components.Schemas, n)

				removed = true
			}
		}
	}
}

// splitAllOf returns the object parts of s's allOf and its one union, with the names of the parts it refers to, or s
// itself as the union when it has no allOf.
func splitAllOf(s *openapi.Schema) (common []*openapi.Schema, union *openapi.Schema, names []string, err error) {
	if len(s.AllOf) == 0 {
		return nil, s, nil, nil
	}

	for i, part := range s.AllOf {
		if name, ok := componentName(part); ok {
			names = append(names, name)
		}

		switch p := deref(part); {
		case len(alternatives(p)) > 0 && union == nil:
			union = p
		case p.Type == openapi.TypeObject:
			common = append(common, p)
		default:
			return nil, nil, nil, fmt.Errorf("allOf[%d] is neither an object nor the one union", i)
		}
	}

	if union == nil {
		return nil, nil, nil, errors.New("no union in allOf")
	}

	return common, union, names, nil
}

// leaves are the objects alts holds, through the unions within it, with the names of the components among them.
func leaves(alts openapi.SchemaList) (objects []*openapi.Schema, names []string, err error) {
	for _, alt := range alts {
		if name, ok := componentName(alt); ok {
			names = append(names, name)
		}

		v := deref(alt)

		if sub := alternatives(v); len(sub) > 0 {
			o, n, err := leaves(sub)
			if err != nil {
				return nil, nil, err
			}

			objects, names = append(objects, o...), append(names, n...)

			continue
		}

		if v.Type != openapi.TypeObject {
			return nil, nil, errors.New("a variant is neither an object nor a union")
		}

		objects = append(objects, v)
	}

	return objects, names, nil
}

// unionMerge collects the properties of a union's variants.
type unionMerge struct {
	tag      string
	tags     []jsontext.Value
	seen     map[string][]byte // each property's JSON, to tell whether the variants agree on it
	shared   map[string]int    // how many variants have each property
	required map[string]int    // how many variants require each property
}

func (m *unionMerge) addCommon(c *openapi.Schema, variants int) error {
	for prop, p := range c.Properties.ByIndex() {
		if err := m.see(prop, p); err != nil {
			return err
		}
	}

	for _, r := range c.Required {
		m.required[r] = variants
	}

	return nil
}

func (m *unionMerge) addVariant(v *openapi.Schema) error {
	t, ok := v.Properties[m.tag]
	if !ok || len(t.Const) == 0 {
		return fmt.Errorf("a variant has no single value for %s", m.tag)
	}

	m.tags = append(m.tags, t.Const)

	for prop, p := range v.Properties.ByIndex() {
		if prop == m.tag {
			continue
		}

		if err := m.see(prop, p); err != nil {
			return err
		}

		m.shared[prop]++
	}

	for _, r := range v.Required {
		m.required[r]++
	}

	return nil
}

// see records p as the property prop, failing if another variant has it differently.
func (m *unionMerge) see(prop string, p *openapi.Schema) error {
	b, err := json.Marshal(p)
	if err != nil {
		return err
	}

	if prev, ok := m.seen[prop]; ok && !bytes.Equal(prev, b) {
		return fmt.Errorf("the variants differ on %s", prop)
	}

	m.seen[prop] = b

	return nil
}

// properties are the common parts' properties, then what every variant has, in the first one's order, with tag as
// an enum, then each variant's own.
func (m *unionMerge) properties(common, variants []*openapi.Schema) openapi.Schemas {
	props := openapi.Schemas{}

	for _, c := range common {
		for prop, p := range c.Properties.ByIndex() {
			props.Set(prop, p)
		}
	}

	m.shared[m.tag] = len(variants)

	for _, own := range []bool{false, true} {
		for _, v := range variants {
			for prop, p := range v.Properties.ByIndex() {
				switch _, done := props[prop]; {
				case done, own == (m.shared[prop] == len(variants)):
				case prop == m.tag:
					props.Set(prop, &openapi.Schema{Type: openapi.TypeString, Enum: m.tags})
				default:
					props.Set(prop, p)
				}
			}
		}
	}

	return props
}

// requiredOf are those of props every one of the variants requires, or a common part does, in their order.
func (m *unionMerge) requiredOf(props openapi.Schemas, variants int) []string {
	var req []string

	for prop := range props.ByIndex() {
		if m.required[prop] >= variants {
			req = append(req, prop)
		}
	}

	return req
}

// alternatives are s's oneOf, or else its anyOf.
func alternatives(s *openapi.Schema) openapi.SchemaList {
	if len(s.OneOf) > 0 {
		return s.OneOf
	}

	return s.AnyOf
}

// componentName is the name of the component schema s refers to, if it is such a reference.
func componentName(s *openapi.Schema) (string, bool) {
	if s.Ref == nil {
		return "", false
	}

	return strings.CutPrefix(s.Ref.Identifier, schemaRefPrefix)
}

// deref is the schema s stands for: the one it refers to, if it is a reference.
func deref(s *openapi.Schema) *openapi.Schema {
	if s.Ref != nil && s.Ref.Value != nil {
		return s.Ref.Value
	}

	return s
}
