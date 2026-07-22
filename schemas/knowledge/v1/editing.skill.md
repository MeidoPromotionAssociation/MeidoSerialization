# MeidoSerialization format editing skill

Format: `{{FORMAT_ID}}`
Semantic coverage: `{{COVERAGE}}`
Schema: `meido://schemas/{{FORMAT_ID}}`
Guide: `meido://guides/{{FORMAT_ID}}`

## Required context

Read both the JSON Schema and the format guide before proposing or applying an edit. Treat the Schema as the exact
editing JSON shape. Treat only `verified` field claims as source-reviewed game-runtime behavior. A `serialization_only`
field documents codec or wire-preservation behavior, not game-runtime meaning. A `schema_only` field has no reviewed
semantics; do not infer its purpose from its name. Unprefixed review states may come from AI source review. Only a
`human_` prefix records explicit human approval; never infer or add that prefix automatically.

## Editing contract

1. Detect the source format unless the caller supplied a confirmed `format_id`.
2. Inspect or convert the native file to editing JSON before changing values.
3. Preserve every field outside the stated objective, including ordering, duplicates, null versus empty collections,
   numeric width, raw slots, future fields, trailing data, hashes, identifiers, and base64 bytes.
4. Honor the guide's `edit_role`, `edit_guidance`, invariants, field relationships, enum meanings, and command ordering.
   For script commands, select a `form` reviewed for the target build and resolve every `value_set_refs` entry before
   choosing an enum, MPN, slot, or other game constant.
5. Never use a preview field as the writable source of truth when an opaque/raw field is present.
6. Do not invent game assets, bone names, material properties, MPN values, hashes, PathIDs, enum values, or version
   migrations.
7. Submit the complete edited document to `meido.validate_editing_json`.
8. Convert to native only after validation succeeds, and follow the write policy declared by the current
   `meido://capabilities` resource.

## Validation boundary

JSON Schema verifies structure and published annotations. `meido.validate_editing_json` verifies native serialization
and supported cross-field rules. Neither proves that an external game resource exists or that a physics/material edit
looks correct in Unity; preserve evidence, report uncertainty, and require in-game verification for visual or behavioral
changes.
