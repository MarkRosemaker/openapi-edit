---
tagline: Change an API spec without breaking it.
logo:
    alt: A gopher moving one luggage tag while others, connected by strings, swing into alignment
    source: openapi-edit.jpg
    width: 500
---

<div align="center" id=badges>

![Code Coverage](https://img.shields.io/badge/coverage-93.8%25-green)

</div>





`openapi-edit` provides safe structural edits to an
[OpenAPI 3.x](https://spec.openapis.org/oas/v3.1.0) specification — the kind of
change where touching one place obliges you to touch several others, and forgetting
one leaves a document that no longer resolves.

> **Status: early.** The scope below is settled and operations arrive one at a
> time, as each earns its place. `RenameSchema` and `RedirectSchema` are the first.
