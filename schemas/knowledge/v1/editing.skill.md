# MeidoSerialization format editing skill

Format: `{{FORMAT_ID}}`
Whole-file verification: `{{FORMAT_VERIFICATION}}`
Schema: `meido://schemas/{{FORMAT_ID}}`
Guide: `meido://guides/{{FORMAT_ID}}`

## Required context

Read both the JSON Schema and the format guide before proposing or applying an edit. Treat the Schema as the exact
editing JSON shape. Guide-level `format_verification.level` has only two meanings: `serialization_verified` confirms
the whole-file serialization contract, while `schema_only` means that only the generated structure is available. The
file-level state never claims that every field's game meaning is known.

Interpret each field's `verification` object independently. `serialization` confirms format, position, or read/write
behavior but does not establish game meaning. `source_semantics` confirms the documented purpose or consumption path
against game source and includes serialization verification. `game_behavior` requires evidence from an actual game
runtime observation; game-source review alone is not runtime observation. Every present claim has `status: verified`
and an explicit `authority` of `ai` or `human`. An empty `verification` object is not a certification: it means the
field is derived only from the Schema, must be preserved, and must never be interpreted from its name. Use
`field_coverage` only as a count summary; it never upgrades an individual field claim.

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
