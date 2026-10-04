```bash
go get github.com/MarkRosemaker/openapi-edit
```

```go
import (
    "github.com/MarkRosemaker/openapi"
    edit "github.com/MarkRosemaker/openapi-edit"
)

// Renames the schema and rewrites every reference to it.
if err := edit.RenameSchema(doc, "GetV1PetByPetIDOkJSONResponse", "Pet"); err != nil {
    log.Fatal(err)
}
```

The schema keeps its position among the components, so a rename produces a
one-line change rather than reordering the section.

Renaming a schema to its current name does nothing and reports no error.
Otherwise the rename fails, **changing nothing at all**, in three cases:

| Error | When |
|---|---|
| `ErrSchemaNotFound` | `components.schemas` has no schema under the old name |
| `ErrSchemaExists` | the new name is already taken by another schema |
| `ErrInvalidSchemaName` | the new name is not a valid key under `components` |

The second is the interesting one. Renaming onto an existing schema would
silently discard one of two different definitions and repoint every reference at
whichever survived — a change that looks successful and quietly alters the API.

The third matters more than validity alone suggests: a name containing `/` would
produce a reference that resolves somewhere else entirely, and one containing a
space would produce a reference that does not resolve at all. Component keys must
match `^[a-zA-Z0-9.\-_]+$`.

### Extracting inline schemas

`ExtractSchema` names a schema a document spells out inline wherever it is used. It moves the schemas a
function accepts into `components.schemas` under one name, and replaces each with a reference to it:

```go
// Every inline array of RichText becomes a reference to RichTexts.
err := edit.ExtractSchema(doc, "RichTexts", func(s *openapi.Schema) bool {
    return s.Type == openapi.TypeArray && s.Items != nil && s.Items.Ref != nil &&
        s.Items.Ref.Identifier == "#/components/schemas/RichText"
})
```

The first schema the function accepts becomes the component, so it should accept only schemas that are the
same. A description stays where it was, on the reference, since it says what the schema is used for there.
Schemas already in `components.schemas` are left alone.

It fails, changing nothing, with `ErrSchemaExists` or `ErrInvalidSchemaName` for the name, as a rename does,
and with `ErrNoMatch` if the function accepts no schema.

### Redirecting a schema onto another

`RenameSchema` refuses to rename a schema onto a name that already exists
(`ErrSchemaExists`). `RedirectSchema` is for when that's exactly the point —
several near-duplicate schemas, typically ones an OpenAPI generator produced
one per endpoint that happen to describe the same thing, are being
consolidated onto one of them:

```go
// Repoints every reference to "GetPetOkResponse" at "Pet", then removes
// "GetPetOkResponse" from components.schemas.
if err := edit.RedirectSchema(doc, "GetPetOkResponse", "Pet", ""); err != nil {
    log.Fatal(err)
}
```

**What it actually does**, precisely — this is a rewrite of references, not a
combination of content:

1. It finds every `$ref` in the document whose value is
   `"#/components/schemas/GetPetOkResponse"` (via the same [`walkSchemas`]
   traversal `RenameSchema` uses) and rewrites each one to
   `"#/components/schemas/Pet"`, now resolving to `Pet`. A discriminator's
   `mapping` value naming `GetPetOkResponse`, by name or by reference, is
   rewritten the same way, and so it is by `RenameSchema`. A discriminator
   that selected `GetPetOkResponse` by its name alone, with no `mapping`
   entry, gains one (`GetPetOkResponse: Pet`), so a payload naming it still
   selects the schema it meant. Both functions do this.
2. A `oneOf` or `anyOf` that listed both `GetPetOkResponse` and `Pet` now
   lists `Pet` twice, so it keeps only the first of those plain references.
   Two alternatives of the same schema are no alternative at all: a value
   matching one matches the other, so a `oneOf` could never hold for it. A
   reference with keywords of its own beside the `$ref` is kept, and a union
   the redirect did not change is left alone.
3. It deletes the `"GetPetOkResponse"` entry from `components.schemas`.
4. It does not look at, merge, or otherwise change the *content* of either
   schema. `Pet`'s definition (its properties, its bounds, its wording) is
   whatever it already was, byte for byte; `GetPetOkResponse`'s definition is
   simply gone, not folded into `Pet`'s.

If `GetPetOkResponse` carried bounds or wording worth keeping, pass it as
`description` instead of an empty string: it becomes the `description`
beside the `$ref` of every reference this repoints, replacing whatever
description that reference already had. That's the one piece of
`GetPetOkResponse` this function can carry forward — everything else about
its definition is discarded the moment step 3 above runs, so this is the
last chance to keep any of it on the sites that used it.

Redirecting a schema onto itself does nothing and reports no error.
Otherwise it fails, **changing nothing at all**, if either name is not in
`components.schemas` (`ErrSchemaNotFound`).

### Many at once

Each call walks the whole document, so a caller consolidating hundreds of
schemas should hand them over together. `RenameSchemas` and `RedirectSchemas`
take a map from old name to new and walk the document a fixed number of times
however many names the map holds:

```go
if err := edit.RedirectSchemas(doc, map[string]string{
    "GetPetOkResponse":   "Pet",
    "ListPetsOkItem":     "Pet",
    "GetOwnerOkResponse": "Owner",
}); err != nil {
    log.Fatal(err)
}
```

The result is the same as calling the single-name function for each entry, and
a failure again changes nothing. `RenameSchemas` renames all its schemas
together, so two can swap names; two taking the same new name is
`ErrSchemaExists`. `RedirectSchemas` refuses to redirect onto a schema it is
also redirecting away (`ErrSchemaNotFound`), since that schema would not be
there afterwards.

This is deliberately *not* the same operation as combining two schemas into
one wider shape (adding one's properties, enum values, etc. to the other) —
that's [`openapi-merge`]'s job, and it works on two schema values directly
rather than on a document and its references. The two are meant to compose
at the call site rather than one wrapping the other: a caller deduplicating
a specification decides, using whatever means it likes (`openapi-merge`
included), which of two schemas should survive and what its content should
be, then calls `RedirectSchema` to point every reference at the survivor and
drop the one that lost. See [Scope](#scope) below.

[`openapi-merge`]: https://github.com/MarkRosemaker/openapi-merge

### Merging tagged unions

An API that tells its objects apart by a property — `"type": "paragraph"` beside
a `paragraph` member — is often specified as a union of one object per type.
`MergeUnion` turns such a union into one object: the properties every variant
has, the tag as an enum of the variants' values, and each variant's own
properties, optional. The union may nest unions, and may be one part of an
`allOf` whose other parts hold what every variant shares.

```go
// Every union in the document that can be merged is.
n := edit.MergeUnions(doc, "type")
```

`MergeUnion` fails, changing nothing, if a variant is not an object, has no
single value for the tag, or disagrees with another on a property they share;
`MergeUnions` leaves such unions as they are. `MergeUnion` returns the component
schemas it took in, and `RemoveUnreferenced` removes those nothing refers to
anymore, which `MergeUnions` does itself.

### Counting and describing references

`CountReferences` counts the references to each component schema, wherever in
the document they occur.

`DescribeReferences` gives every reference to the named schemas a description
beside the `$ref`, unless the reference has one of its own. A description says
what a schema is used for in one place, so before redirecting schemas that
describe the same shape onto one, describing the references to each keeps what
each meant where it was used:

```go
if err := edit.DescribeReferences(doc, map[string]string{
    "BotWorkspaceName": "The name of the bot's workspace.",
}); err != nil {
    log.Fatal(err)
}
```

It fails, changing nothing, if a name is not in `components.schemas`
(`ErrSchemaNotFound`).
