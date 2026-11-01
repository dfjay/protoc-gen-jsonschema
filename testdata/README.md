# TestCases

This directory contains test cases for the protoc-gen-jsonschema plugin.

# Make a new test case

1. Create a new directory in the `testdata/cases` directory.
2. Create `test.proto` file and `test.yaml` file in the new directory.
3. Fill the `test.proto` file with proper content.
4. Fill the `test.yaml` file with expected test result.

### test.yaml format

```yaml
name: "proto-json-name-ignore" # test case name, should be unique
description: "test for ignore proto json_name option" # description of the test case

inputFiles: ["test.proto"] # input proto file
inputParameters: # protoc-gen-jsonschema plugin options. Map<string, any> type
  preserve_proto_field_names: true

expectResultFiles: ["test.schema.json"] # expected output schema file
expectResultIsNull: false # if true, expectResultFile is empty test will pass
expectFail: false # if true, generate schema fail test will pass

expectedBehaviorDescription: "" # not used
```

### Generate all test case results

```sh
just golden
```

### Generate single test case result

```sh
just golden <test case name>
```

# Test all cases

```sh
just test
```

# Test single case

Each case is a subtest named after its directory:

```sh
cd testdata && go test -run 'TestPlugin/proto-enum$' .
```

# Test cases

| Case | Checks |
|------|--------|
| `jsonschema-array` | array validation keywords |
| `jsonschema-description` | leading, trailing and detached comments become `description` |
| `jsonschema-field-required` | the per-field `required` option with `respect_protojson_presence` |
| `jsonschema-nullable` | `nullable` wraps the field schema in a `oneOf` with `null` |
| `jsonschema-number` | numeric validation keywords |
| `jsonschema-object` | object validation keywords |
| `jsonschema-string` | string validation keywords |
| `plugin-additional-properties-always-false` | `additional_properties=AlwaysFalse` sets `additionalProperties: false` on every message |
| `plugin-additional-properties-always-true` | `additional_properties=AlwaysTrue` sets `additionalProperties: true` on every message |
| `plugin-additional-properties-default-false` | `additional_properties=DefaultFalse` applies to messages that don't set it |
| `plugin-additional-properties-default-true` | `additional_properties=DefaultTrue` applies to messages that don't set it |
| `plugin-additional-properties-do-nothing` | `additional_properties=DoNothing` leaves `additionalProperties` alone |
| `plugin-consist-output` | the output is identical across runs, with messages imported from other files |
| `plugin-draft-04` | draft-04 output |
| `plugin-draft-06` | draft-06 output |
| `plugin-draft-07` | draft-07 output |
| `plugin-draft-201909` | draft-2019-09 output |
| `plugin-draft-202012` | draft-2020-12 output |
| `plugin-entrypoint` | the `entrypoint_message` plugin option |
| `plugin-output-yaml` | YAML output via `output_file_suffix` |
| `plugin-preserve-field-name-false` | field names follow `json_name` |
| `plugin-preserve-field-name-true` | field names keep the proto names, ignoring `json_name` |
| `plugin-respect-protojson-no-required` | `respect_protojson_presence` adds no automatic `required` |
| `proto-circular-dependent` | messages that reference each other |
| `proto-enum` | enums become `enum` |
| `proto-map` | maps become objects, with no array keywords |
| `proto-oneof-multiple` | several oneofs in one message |
| `proto-oneof-single` | a single oneof |
| `proto-optional` | `optional` fields are left out of `required` |
| `proto-version-syntax-proto2` | `syntax = "proto2"`, with `optional` and `required` labels |
| `proto-version-syntax-proto3` | `syntax = "proto3"` |
| `wellknown-k8s` | Kubernetes `IntOrString` |
| `wellknown-proto` | `google.protobuf.Any`, `Duration` and `Timestamp` |
