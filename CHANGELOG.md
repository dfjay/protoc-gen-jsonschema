# Changelog

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
