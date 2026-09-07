<div align="center" id=badges>

[![Go Reference](https://pkg.go.dev/badge/github.com/MarkRosemaker/openapi-edit.svg)](https://pkg.go.dev/github.com/MarkRosemaker/openapi-edit)
[![Go Report Card](https://goreportcard.com/badge/github.com/MarkRosemaker/openapi-edit)](https://goreportcard.com/report/github.com/MarkRosemaker/openapi-edit)
![Code Coverage](https://img.shields.io/badge/coverage-93.8%25-green)
[![License: Apache](https://img.shields.io/badge/License-Apache-yellow.svg)](./LICENSE)

</div>

<p align="center">
  <img alt="A gopher moving one luggage tag while others, connected by strings, swing into alignment" src=openapi-edit.jpg width=500>
</p>

<h3 align="center">
  Change an API spec without breaking it.
</h3>

`openapi-edit` provides safe structural edits to an
[OpenAPI 3.x](https://spec.openapis.org/oas/v3.1.0) specification — the kind of
change where touching one place obliges you to touch several others, and forgetting
one leaves a document that no longer resolves.

> **Status: early.** The scope below is settled and operations arrive one at a
> time, as each earns its place. `RenameSchema` was the first; `MergeSchema`
> and the underlying `WalkSchemaRefs` traversal followed.

## Introduction

Renaming a schema is the canonical example. The rename itself is a single map
operation, but every `$ref` that pointed at the old name is now dangling — and those
`$ref`s can be anywhere: nested inside another schema's properties, inside an
`allOf` branch, in a response's content, in a parameter, in a callback. Getting this
right means walking the entire document. Getting it wrong means a spec that looks
fine and fails to resolve.

That traversal is worth writing once, carefully, and reusing.

This module serves two kinds of caller:

- **Directly**, when you are writing code against your own specification and want to
  make a specific change safely, without reimplementing the bookkeeping.
- **As a dependency**, for tools like
  [`openapi-compress`](https://github.com/MarkRosemaker/openapi-compress) and
  [`openapi-flatten`](https://github.com/MarkRosemaker/openapi-flatten) that run an
  algorithm over a whole specification and need the same primitives underneath.

## Usage

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

### Merging two schemas

`RenameSchema` refuses to rename a schema onto a name that already exists.
`MergeSchema` is for when that's exactly the point — several near-duplicate
schemas, typically ones an OpenAPI generator produced one per endpoint that
happen to describe the same thing, are being consolidated into one:

```go
// Repoints every reference to "GetPetOkResponse" at "Pet", then removes
// "GetPetOkResponse" from components.schemas.
if err := edit.MergeSchema(doc, "GetPetOkResponse", "Pet", ""); err != nil {
    log.Fatal(err)
}
```

Unlike `RenameSchema`, `Pet`'s own definition is left untouched — only the
references that pointed at `GetPetOkResponse` move. If `GetPetOkResponse`
carried bounds or wording worth keeping, pass it as `description` instead of
an empty string: it becomes the `$ref`-level description on every reference
this repoints, which is the last chance to keep that information once
`GetPetOkResponse`'s own definition is discarded.

Merging a schema into itself does nothing and reports no error. Otherwise it
fails, **changing nothing at all**, if either name is not in
`components.schemas` (`ErrSchemaNotFound`).

`MergeSchema` only repoints references — it never widens or reshapes a
schema's own definition to cover what the other one described. Deciding
whether two schemas are close enough to consolidate, or combining two
independently inferred schemas into one wider shape, is out of scope here;
see [Scope](#scope) below.

### Finding every reference to a schema

The traversal both operations above are built on is exported in its own
right, for callers that need to find every reference to a schema without
rewriting them:

```go
edit.WalkSchemaRefs(doc, func(r *openapi.SchemaRef) {
    if r.Ref != nil && r.Ref.Identifier == "#/components/schemas/Pet" {
        fmt.Println("referenced")
    }
})
```

It calls `fn` once per schema reference reachable from `doc` — through every
component (schemas, responses, parameters, request bodies, headers,
callbacks, path items) and through every path, operation, and webhook — and
walks into a schema reached more than once only the first time, so `fn` can
freely mutate what it's given without looping on a self-referential schema.

## Scope

Operations belong here when they satisfy two conditions: they **mutate** a
document, and doing them correctly requires knowledge of the document *beyond* the
node being changed.

**In scope**

- ✅ Renaming a component and rewriting every reference to it (`RenameSchema`)
- ✅ Consolidating duplicate components, repointing references left behind onto
  the survivor (`MergeSchema`)
- Moving a definition between inline and `components`, keeping references intact
- ✅ Finding every location that refers to a given component (`WalkSchemaRefs`)

**Out of scope**

- Deciding *whether* two things should be merged — that is
  [`openapi-compare`](https://github.com/MarkRosemaker/openapi-compare)
- Combining two independently inferred schemas into one wider schema that
  covers what both described — that is
  [`openapi-merge`](https://github.com/MarkRosemaker/openapi-merge).
  `MergeSchema` above is a different, narrower operation: it never touches a
  schema's own definition, only the references that pointed at the one being
  discarded.
- Whole-document policies such as flattening or deduplication — those are their own
  modules, and they are expected to *use* this one
- Anything universal enough to belong on the types themselves — that goes into
  [`openapi`](https://github.com/MarkRosemaker/openapi) instead, so that users who
  only want to parse and validate a spec aren't made to carry it

## The openapi family

| Module | Purpose |
|---|---|
| [openapi](https://github.com/MarkRosemaker/openapi) | Parse, validate, and write OpenAPI 3.x specifications |
| [openapi-compare](https://github.com/MarkRosemaker/openapi-compare) | Compare specification objects — exact equality and shape equivalence |
| **openapi-edit** (this module) | Safe structural edits, such as renaming a schema and rewriting every `$ref` to it |
| [openapi-flatten](https://github.com/MarkRosemaker/openapi-flatten) | Promote inline definitions into named `components` entries |
| [openapi-compress](https://github.com/MarkRosemaker/openapi-compress) | Deduplicate and merge equivalent component schemas |
| [openapi-merge](https://github.com/MarkRosemaker/openapi-merge) | Merge schemas that were inferred independently from different samples |
| [openapi-enrich](https://github.com/MarkRosemaker/openapi-enrich) | Infer specification content from observed HTTP traffic |
| [openapi-codegen](https://github.com/MarkRosemaker/openapi-codegen) | Generate Go types, clients, and servers from a specification |

## Contributing

If you have any contributions to make, please submit a pull request or open an issue on the [GitHub repository](https://github.com/MarkRosemaker/openapi-edit).

## License

This project is licensed under the [Apache 2.0 License](./LICENSE).
