# Changelog

## [0.12.0](https://github.com/dfjay/protoc-gen-jsonschema/compare/v0.11.0...v0.12.0) (2026-11-10)

Generated schemas now reflect supported protovalidate rules by default. This release also publishes the schema annotations to the Buf Schema Registry, so Buf projects can resolve them as a dependency.

### ⚠ BREAKING CHANGES

* **protovalidate:** Existing `buf.validate` annotations now affect schemas by default, including annotations on referenced imported types. Regeneration can tighten accepted values, require fields and enforce oneof rules. Set `respect_buf_validate=false` to ignore these annotations as in 0.11.x. ([#18](https://github.com/dfjay/protoc-gen-jsonschema/issues/18)) ([b11ebd8](https://github.com/dfjay/protoc-gen-jsonschema/commit/b11ebd84a2ec00e1c9898dfddf7592a0c8a19ec5))

### Features

* **protovalidate rules:** New option `respect_buf_validate` (default `true`) maps supported numeric, string, boolean, enum, repeated and map rules to schema constraints, including rules on wrapper types. Compatible RE2 patterns are translated to ECMA-262. Rules are read from the request descriptors; the plugin does not link protovalidate. ([#18](https://github.com/dfjay/protoc-gen-jsonschema/issues/18)) ([b11ebd8](https://github.com/dfjay/protoc-gen-jsonschema/commit/b11ebd84a2ec00e1c9898dfddf7592a0c8a19ec5))
* **Required fields:** `(buf.validate.field).required`, and rules that reject the zero value of a field without presence, can make fields required. An explicitly required field without presence must also be nonzero; a required oneof member must be selected. The per-field `required` option takes precedence, followed by proto2 `required` and editions `LEGACY_REQUIRED`. Protovalidate requirements also suppress automatic null allowances. ([#18](https://github.com/dfjay/protoc-gen-jsonschema/issues/18)) ([b11ebd8](https://github.com/dfjay/protoc-gen-jsonschema/commit/b11ebd84a2ec00e1c9898dfddf7592a0c8a19ec5))
* **Oneofs:** `(buf.validate.oneof).required` requires exactly one set member. `(buf.validate.message).oneof` requires exactly one of its listed fields when `required: true`, or at most one otherwise. Zero values of fields without presence count as unset. ([#18](https://github.com/dfjay/protoc-gen-jsonschema/issues/18)) ([b11ebd8](https://github.com/dfjay/protoc-gen-jsonschema/commit/b11ebd84a2ec00e1c9898dfddf7592a0c8a19ec5))
* **Overrides and exemptions:** Field annotations with `numeric`, `string` or `array` keywords replace the corresponding protovalidate rules; an array override retains item rules. Supported `IGNORE_IF_ZERO_VALUE` exemptions include empty messages in repeated items and map values. `IGNORE_ALWAYS` skips protovalidate rules for the annotated field or element and its nested messages, while retaining custom schema annotations. ([#18](https://github.com/dfjay/protoc-gen-jsonschema/issues/18)) ([b11ebd8](https://github.com/dfjay/protoc-gen-jsonschema/commit/b11ebd84a2ec00e1c9898dfddf7592a0c8a19ec5))
* **Numeric representations:** Bounds on 64-bit integers written as strings filter `const` and `in` values but do not otherwise constrain the strings; `not_in` still applies. In Lenient mode, fields with numeric rules accept numbers instead of numeric strings. ([#18](https://github.com/dfjay/protoc-gen-jsonschema/issues/18)) ([b11ebd8](https://github.com/dfjay/protoc-gen-jsonschema/commit/b11ebd84a2ec00e1c9898dfddf7592a0c8a19ec5))
* **Mapping limits:** CEL rules, unsupported string rules and patterns, non-string map-key rules, and type-specific rules for `bytes`, `Duration`, `Timestamp`, `Any` and `FieldMask` are skipped. String map-key constraints require draft-06 or later. Validators control enforcement of emitted `format` keywords. The README documents the full mapping and its limitations. ([#18](https://github.com/dfjay/protoc-gen-jsonschema/issues/18)) ([b11ebd8](https://github.com/dfjay/protoc-gen-jsonschema/commit/b11ebd84a2ec00e1c9898dfddf7592a0c8a19ec5))
* **Buf Schema Registry:** The annotations are available as `buf.build/dfjay-dev/protoc-gen-jsonschema`, with labels matching plugin release tags. Add `buf.build/dfjay-dev/protoc-gen-jsonschema:v0.12.0` to `deps` in `buf.yaml` and run `buf dep update`. Keep the installed executable configured as a local plugin in `buf.gen.yaml`; the README's new "Use with Buf" section shows the configuration. ([#21](https://github.com/dfjay/protoc-gen-jsonschema/issues/21)) ([9609858](https://github.com/dfjay/protoc-gen-jsonschema/commit/96098585abbea7c0cb49f84e63e84140ea0c7a00))

### Upgrading from 0.11.x

* **Regeneration:** Regenerate and review schemas whose reachable types carry `buf.validate` annotations, including imported types. Schemas without these annotations in reachable types retain their existing output. To defer applying the rules, generate with `--jsonschema_opt=respect_buf_validate=false`. ([#18](https://github.com/dfjay/protoc-gen-jsonschema/issues/18)) ([b11ebd8](https://github.com/dfjay/protoc-gen-jsonschema/commit/b11ebd84a2ec00e1c9898dfddf7592a0c8a19ec5))

## [0.11.0](https://github.com/dfjay/protoc-gen-jsonschema/compare/v0.10.0...v0.11.0) (2026-11-02)

This release adds multiple entrypoints per proto file, examples, deprecation annotations and optional descriptions for individual enum values. It also fixes validation of non-object entrypoints and preserves keywords alongside references in older JSON Schema drafts.

### Features

* **Entrypoints:** Mark any number of messages, including nested messages, with `(dfjay.jsonschema.message) = {entrypoint: true}`. Each gets a self-contained schema: `form.Customer.schema.json` or `form.Outer.Inner.schema.json`. Adding an entrypoint renames no existing file. These annotations work alongside `entrypoint_message`, which keeps its `form.schema.json` output name; marked messages hidden by `visibility_level` are skipped. Two entrypoints that would write the same file fail generation. ([#14](https://github.com/dfjay/protoc-gen-jsonschema/issues/14)) ([7bb105b](https://github.com/dfjay/protoc-gen-jsonschema/commit/7bb105ba20c3f2215254109ec7df51a0538eed8d))
* **Deprecation:** Protobuf `deprecated = true` on messages, fields and enums emits `deprecated: true` in draft 2019-09 and later, including on repeated and map fields. Earlier drafts omit the keyword. ([#17](https://github.com/dfjay/protoc-gen-jsonschema/issues/17)) ([a5c193e](https://github.com/dfjay/protoc-gen-jsonschema/commit/a5c193edf47d6cdbead0e77c8bed72b25ea2392b))
* **Examples:** Field and message annotations now accept `examples` as `google.protobuf.Value` values and emit them in draft-06 and later. Supply examples in ProtoJSON form: an array for a repeated field and an object for a map. ([#17](https://github.com/dfjay/protoc-gen-jsonschema/issues/17)) ([a5c193e](https://github.com/dfjay/protoc-gen-jsonschema/commit/a5c193edf47d6cdbead0e77c8bed72b25ea2392b))
* **Enum value descriptions:** New option `enum_value_descriptions=true` (default `false`) replaces `enum` with an `anyOf` of per-value `const` schemas, carrying comments as `description` and `deprecated: true` in draft 2019-09 and later. It supports `MapToNumber`, `MapToCustom` and `protojson_mode=Lenient`; draft-04, which has no `const`, gets a one-item `enum` per value. ([#17](https://github.com/dfjay/protoc-gen-jsonschema/issues/17)) ([a5c193e](https://github.com/dfjay/protoc-gen-jsonschema/commit/a5c193edf47d6cdbead0e77c8bed72b25ea2392b))

### Bug Fixes

* **Reference siblings:** Draft-04, draft-06 and draft-07 now move `$ref` into `allOf` when sibling keywords would otherwise be ignored. This preserves field annotations such as `title`, `description` and `examples` on referenced message and enum types. ([#17](https://github.com/dfjay/protoc-gen-jsonschema/issues/17)) ([a5c193e](https://github.com/dfjay/protoc-gen-jsonschema/commit/a5c193edf47d6cdbead0e77c8bed72b25ea2392b))
* **Schema roots:** Remove the unconditional root `type: object`, leaving the referenced entrypoint to define its type. This fixes validation of non-object entrypoints such as Kubernetes `IntOrString` in draft 2019-09 and later. ([#17](https://github.com/dfjay/protoc-gen-jsonschema/issues/17)) ([a5c193e](https://github.com/dfjay/protoc-gen-jsonschema/commit/a5c193edf47d6cdbead0e77c8bed72b25ea2392b))

### Upgrading from 0.10.x

* **Regeneration:** Regenerate schemas to apply the root-type and reference fixes. Every generated schema loses the root `type: object`; draft-04, draft-06 and draft-07 output may also gain `allOf` wrappers around references with sibling keywords. Messages, fields and enums marked `deprecated` gain `deprecated: true` in draft 2019-09 and later. ([#17](https://github.com/dfjay/protoc-gen-jsonschema/issues/17)) ([a5c193e](https://github.com/dfjay/protoc-gen-jsonschema/commit/a5c193edf47d6cdbead0e77c8bed72b25ea2392b))

## [0.10.0](https://github.com/dfjay/protoc-gen-jsonschema/compare/v0.9.1...v0.10.0) (2026-11-01)

Generated schemas now follow ProtoJSON by default. This release also adds protobuf editions, `google.api.field_behavior` support and a `Lenient` mode for JSON or YAML that a protobuf parser reads.

### ⚠ BREAKING CHANGES

* **Presence:** `respect_protojson_presence` now defaults to `true`, so ordinary singular fields are no longer automatically added to `required`. Set `respect_protojson_presence=false` to restore automatic requirements for non-optional singular fields outside oneofs. ([#10](https://github.com/dfjay/protoc-gen-jsonschema/issues/10)) ([f928bf4](https://github.com/dfjay/protoc-gen-jsonschema/commit/f928bf44d72637d7d2b854bf2d94481fa6b247cb))
* **Required fields:** Proto2 `required` and editions `LEGACY_REQUIRED` fields are now required even with `respect_protojson_presence=true`. The per-field `(dfjay.jsonschema.field).required` option still takes precedence: set `required: false` to make such a field optional. ([#10](https://github.com/dfjay/protoc-gen-jsonschema/issues/10)) ([f928bf4](https://github.com/dfjay/protoc-gen-jsonschema/commit/f928bf44d72637d7d2b854bf2d94481fa6b247cb))
* **64-bit integers:** `respect_protojson_int64` and its deprecated alias `int64_as_string` now default to `true`. All 64-bit integer kinds, including repeated elements, map values and wrappers, use decimal strings with signed or unsigned patterns. The undefined `format: int64` is no longer emitted. Set `respect_protojson_int64=false` to use integers in `Canonical` mode. ([#10](https://github.com/dfjay/protoc-gen-jsonschema/issues/10)) ([f928bf4](https://github.com/dfjay/protoc-gen-jsonschema/commit/f928bf44d72637d7d2b854bf2d94481fa6b247cb))
* **Field behavior:** Existing `google.api.field_behavior` annotations now affect schemas by default. Set `respect_field_behavior=false` to ignore them as in 0.9.x. ([#10](https://github.com/dfjay/protoc-gen-jsonschema/issues/10)) ([f928bf4](https://github.com/dfjay/protoc-gen-jsonschema/commit/f928bf44d72637d7d2b854bf2d94481fa6b247cb))

### Upgrading from 0.9.x

Install this version and regenerate your schemas:

```sh
go install github.com/dfjay/protoc-gen-jsonschema@v0.10.0
```

To restore the previous option defaults when generating a schema for `Form` in `form.proto`:

```sh
protoc --jsonschema_out=. \
  --jsonschema_opt=entrypoint_message=Form \
  --jsonschema_opt=respect_protojson_presence=false \
  --jsonschema_opt=respect_protojson_int64=false \
  --jsonschema_opt=respect_field_behavior=false \
  form.proto
```

The well-known type, map-key and floating-point fixes below still apply. Proto2 `required` and editions `LEGACY_REQUIRED` fields still require the per-field override to become optional.

### Features

* **Editions:** Generate schemas for `edition = "2023"` and `edition = "2024"` files, including `DELIMITED` message fields. Resolved `field_presence` settings on fields, enclosing messages and files are honored: `EXPLICIT` fields follow optional-field behavior, while `IMPLICIT` fields are no longer made nullable, including in imported editions files. ([#8](https://github.com/dfjay/protoc-gen-jsonschema/issues/8)) ([2d23a5f](https://github.com/dfjay/protoc-gen-jsonschema/commit/2d23a5f278030a6aa61385c17ac27f8beb0bfcc6))
* **Field behavior:** `google.api.field_behavior = REQUIRED` adds fields outside oneofs to `required`, unless overridden by the per-field `required` option. `OUTPUT_ONLY` adds `readOnly` and `INPUT_ONLY` adds `writeOnly` in draft-07 and later. References on annotated fields move into `allOf` so these keywords work beside them in draft-07. ([#10](https://github.com/dfjay/protoc-gen-jsonschema/issues/10)) ([f928bf4](https://github.com/dfjay/protoc-gen-jsonschema/commit/f928bf44d72637d7d2b854bf2d94481fa6b247cb))
* **ProtoJSON modes:** New option `protojson_mode=Canonical` (default) describes serializer output; `protojson_mode=Lenient` also accepts parser input forms, for uses such as handwritten configuration. Unknown mode values fail generation. ([#11](https://github.com/dfjay/protoc-gen-jsonschema/issues/11)) ([5cf5261](https://github.com/dfjay/protoc-gen-jsonschema/commit/5cf5261d3dfbef91d0cb108aac124cc728c41f37))
* **Lenient numbers and enums:** Integers accept numbers or integral numeric strings such as `"-5"`, `"1.0"` and `"1e2"`. Numeric branches enforce 32-bit ranges and reject negative unsigned values. Floats also accept numeric strings such as `"1e3"`. Enums accept value names or any int32 number; `NullValue` additionally accepts `"NULL_VALUE"` or any int32 number alongside `null`. ([#11](https://github.com/dfjay/protoc-gen-jsonschema/issues/11)) ([5cf5261](https://github.com/dfjay/protoc-gen-jsonschema/commit/5cf5261d3dfbef91d0cb108aac124cc728c41f37))
* **Lenient strings and map keys:** Bytes accept URL-safe base64 and missing padding. `Duration` also accepts spellings such as `"+1s"`, `".5s"` and `"1.s"`. Integer map keys accept leading zeros and, for signed keys, a leading plus sign. ([#11](https://github.com/dfjay/protoc-gen-jsonschema/issues/11)) ([5cf5261](https://github.com/dfjay/protoc-gen-jsonschema/commit/5cf5261d3dfbef91d0cb108aac124cc728c41f37))
* **Lenient nulls and oneofs:** Fields accept `null`, except whole repeated `Value` or `NullValue` lists. A oneof member set to `null` counts as unset, allowing another member to be set; `Value` and `NullValue` members still count as set because `null` is a value for those types. ([#11](https://github.com/dfjay/protoc-gen-jsonschema/issues/11)) ([5cf5261](https://github.com/dfjay/protoc-gen-jsonschema/commit/5cf5261d3dfbef91d0cb108aac124cc728c41f37))
* **Lenient limits:** Numeric keywords keep numeric schemas number-only. Field names stay as configured, and `MapToCustom` enum mappings are unchanged. `mandatory_nullable` and `respect_protojson_int64` have no effect in this mode; `respect_protojson_presence` still controls automatic requirements. ([#11](https://github.com/dfjay/protoc-gen-jsonschema/issues/11)) ([5cf5261](https://github.com/dfjay/protoc-gen-jsonschema/commit/5cf5261d3dfbef91d0cb108aac124cc728c41f37))

### Bug Fixes

* **Well-known messages:** Apply ProtoJSON forms consistently to singular fields, repeated elements and map values: `Timestamp` is a string with `format: date-time`; `FieldMask` is a string; `Any` and `Struct` are objects; `ListValue` is an array; `Value` accepts any JSON value; wrappers use the wrapped value's schema. `Duration` uses a pattern for seconds with an `s` suffix, such as `"1.5s"`, replacing the ISO 8601 `format: duration`. ([#10](https://github.com/dfjay/protoc-gen-jsonschema/issues/10)) ([f928bf4](https://github.com/dfjay/protoc-gen-jsonschema/commit/f928bf44d72637d7d2b854bf2d94481fa6b247cb))
* **Null values:** `NullValue` uses `type: null` instead of the enum string `"NULL_VALUE"`. Optional `Value` and `NullValue` fields no longer receive a nullable `oneOf` whose branches both match `null`. ([#10](https://github.com/dfjay/protoc-gen-jsonschema/issues/10)) ([f928bf4](https://github.com/dfjay/protoc-gen-jsonschema/commit/f928bf44d72637d7d2b854bf2d94481fa6b247cb))
* **Floating-point values:** `float` and `double` also accept `"NaN"`, `"Infinity"` and `"-Infinity"`. Fields with numeric keywords such as `minimum` remain plain numbers. ([#10](https://github.com/dfjay/protoc-gen-jsonschema/issues/10)) ([f928bf4](https://github.com/dfjay/protoc-gen-jsonschema/commit/f928bf44d72637d7d2b854bf2d94481fa6b247cb))
* **Map keys:** Integer keys get signed or unsigned decimal patterns, and boolean keys are restricted to `"true"` or `"false"`, through `propertyNames` in draft-06 and later. Draft-04 cannot validate map keys. ([#10](https://github.com/dfjay/protoc-gen-jsonschema/issues/10)) ([f928bf4](https://github.com/dfjay/protoc-gen-jsonschema/commit/f928bf44d72637d7d2b854bf2d94481fa6b247cb))
* **Plugin execution:** Errors are returned to protoc for reporting on one line. Notes and `DEBUG` output go to stderr with a `protoc-gen-jsonschema:` prefix and file context where applicable. Output files follow protoc's input order. ([#7](https://github.com/dfjay/protoc-gen-jsonschema/issues/7)) ([1615555](https://github.com/dfjay/protoc-gen-jsonschema/commit/16155550b8619d40fe1db169757d28b5983c8e18))

## [0.9.1](https://github.com/dfjay/protoc-gen-jsonschema/compare/v0.9.0...v0.9.1) (2026-11-01)


### Bug Fixes

* explain the required entrypoint message ([#5](https://github.com/dfjay/protoc-gen-jsonschema/issues/5)) ([a340ce3](https://github.com/dfjay/protoc-gen-jsonschema/commit/a340ce3d9ab90a9fb0b749d7c44e0b2f41253306))
* report the module version for go install builds ([a340ce3](https://github.com/dfjay/protoc-gen-jsonschema/commit/a340ce3d9ab90a9fb0b749d7c44e0b2f41253306))

## [0.9.0](https://github.com/dfjay/protoc-gen-jsonschema/compare/v0.8.0...v0.9.0) (2026-11-02)

This is the first release of the fork continuing the unmaintained [pubg/protoc-gen-jsonschema](https://github.com/pubg/protoc-gen-jsonschema). Versioning continues from upstream v0.8.0.

### ⚠ BREAKING CHANGES

* The Go module path, options proto import path and option namespace have changed. Building from source now requires Go 1.26 or later. ([#1](https://github.com/dfjay/protoc-gen-jsonschema/issues/1)) ([14f2662](https://github.com/dfjay/protoc-gen-jsonschema/commit/14f2662ed8a728ec1abfb25d876765121349e171))
* `Draft05` has been removed from the `Draft` enum. ([#4](https://github.com/dfjay/protoc-gen-jsonschema/issues/4)) ([c2ad754](https://github.com/dfjay/protoc-gen-jsonschema/commit/c2ad754b9dc403a1eb9e8cb3079f94bec686455d))
* Generation now fails for unknown `draft` or `additional_properties` values, malformed boolean values and malformed `visibility_level` values. ([#4](https://github.com/dfjay/protoc-gen-jsonschema/issues/4)) ([c2ad754](https://github.com/dfjay/protoc-gen-jsonschema/commit/c2ad754b9dc403a1eb9e8cb3079f94bec686455d))
* `respect_protojson_presence=true` no longer automatically adds fields to `required`. ([4f03835](https://github.com/dfjay/protoc-gen-jsonschema/commit/4f03835204297e6ac209f63e85882381d074d2d7))

### Upgrading from pubg/protoc-gen-jsonschema

Install this version:

```sh
go install github.com/dfjay/protoc-gen-jsonschema@v0.9.0
```

Update the import and all custom option references in your `.proto` files:

```diff
 syntax = "proto3";

-import "jsonschema.proto";
+import "dfjay/jsonschema/options.proto";

-option (pubg.jsonschema.file) = {entrypoint_message: "Form"};
+option (dfjay.jsonschema.file) = {entrypoint_message: "Form"};

 message Form {
-  string name = 1 [(pubg.jsonschema.field) = {title: "Name"}];
+  string name = 1 [(dfjay.jsonschema.field) = {title: "Name"}];
 }
```

Point `-I` at the `proto` directory from a repository checkout or an extracted release archive:

```sh
protoc -I path/to/protoc-gen-jsonschema/proto -I . --jsonschema_out=. your.proto
```

If you previously selected `Draft05`, use `--jsonschema_opt=draft=Draft04`. Regenerate your schemas after upgrading.

### Features

* Prebuilt binaries for Linux, macOS and Windows on amd64 and arm64, with cosign-signed checksums, SBOMs and GitHub build provenance. ([#1](https://github.com/dfjay/protoc-gen-jsonschema/issues/1)) ([14f2662](https://github.com/dfjay/protoc-gen-jsonschema/commit/14f2662ed8a728ec1abfb25d876765121349e171))
* A per-field `required` option overrides plugin defaults: use `[(dfjay.jsonschema.field) = {required: true}]` to require a field, or `required: false` to make it optional. ([4f03835](https://github.com/dfjay/protoc-gen-jsonschema/commit/4f03835204297e6ac209f63e85882381d074d2d7))

### Bug Fixes

* **Arrays:** Repeated-field element schemas now use `items` in draft-04, draft-06, draft-07 and draft-2019-09. Previously, array elements were not validated. ([#4](https://github.com/dfjay/protoc-gen-jsonschema/issues/4)) ([c2ad754](https://github.com/dfjay/protoc-gen-jsonschema/commit/c2ad754b9dc403a1eb9e8cb3079f94bec686455d))
* **Maps:** Integer-valued maps remain objects, with types and formats applied to their values. Numeric constraints now apply to integer and floating-point map values. ([#4](https://github.com/dfjay/protoc-gen-jsonschema/issues/4)) ([c2ad754](https://github.com/dfjay/protoc-gen-jsonschema/commit/c2ad754b9dc403a1eb9e8cb3079f94bec686455d))
* **Enums:** Remove duplicate values, use `integer` for `MapToNumber`, and only add `type: string` to `MapToCustom` when every value is a string. ([#4](https://github.com/dfjay/protoc-gen-jsonschema/issues/4)) ([c2ad754](https://github.com/dfjay/protoc-gen-jsonschema/commit/c2ad754b9dc403a1eb9e8cb3079f94bec686455d))
* **Visibility:** Omit hidden fields from properties, required lists and oneofs. References to hidden messages produce a clear error instead of a panic. ([#4](https://github.com/dfjay/protoc-gen-jsonschema/issues/4)) ([c2ad754](https://github.com/dfjay/protoc-gen-jsonschema/commit/c2ad754b9dc403a1eb9e8cb3079f94bec686455d))
* **Oneofs:** Omit redundant constraints for proto3 `optional` fields and oneofs with fewer than two visible members. ([#4](https://github.com/dfjay/protoc-gen-jsonschema/issues/4)) ([c2ad754](https://github.com/dfjay/protoc-gen-jsonschema/commit/c2ad754b9dc403a1eb9e8cb3079f94bec686455d))
* **Options:** `draft=DraftDefault` selects draft-2020-12 instead of producing no output; `additional_properties=DefaultAdditionalProperties` behaves like `DoNothing` instead of panicking. ([#4](https://github.com/dfjay/protoc-gen-jsonschema/issues/4)) ([c2ad754](https://github.com/dfjay/protoc-gen-jsonschema/commit/c2ad754b9dc403a1eb9e8cb3079f94bec686455d))
* **Numeric constraints:** Reject `multiple_of <= 0` instead of generating an invalid schema. ([#4](https://github.com/dfjay/protoc-gen-jsonschema/issues/4)) ([c2ad754](https://github.com/dfjay/protoc-gen-jsonschema/commit/c2ad754b9dc403a1eb9e8cb3079f94bec686455d))
* **Schema copies:** Changing numeric bounds or boolean keywords in a copied schema no longer changes the original. ([#1](https://github.com/dfjay/protoc-gen-jsonschema/issues/1)) ([14f2662](https://github.com/dfjay/protoc-gen-jsonschema/commit/14f2662ed8a728ec1abfb25d876765121349e171))
