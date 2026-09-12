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
   `"#/components/schemas/GetPetOkResponse"` (via the same [`walkSchemaRefs`]
   traversal `RenameSchema` uses) and rewrites each one to
   `"#/components/schemas/Pet"`.
2. It deletes the `"GetPetOkResponse"` entry from `components.schemas`.
3. It does not look at, merge, or otherwise change the *content* of either
   schema. `Pet`'s definition (its properties, its bounds, its wording) is
   whatever it already was, byte for byte; `GetPetOkResponse`'s definition is
   simply gone, not folded into `Pet`'s.

If `GetPetOkResponse` carried bounds or wording worth keeping, pass it as
`description` instead of an empty string: it becomes the `$ref`-level
`description` on every reference this repoints, replacing whatever
description that reference already had. That's the one piece of
`GetPetOkResponse` this function can carry forward — everything else about
its definition is discarded the moment step 2 above runs, so this is the
last chance to keep any of it on the sites that used it.

Redirecting a schema onto itself does nothing and reports no error.
Otherwise it fails, **changing nothing at all**, if either name is not in
`components.schemas` (`ErrSchemaNotFound`).

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
