# Protocol Documentation
<a name="top"></a>

## Table of Contents

- [dfjay/jsonschema/options.proto](#dfjay_jsonschema_options-proto)
    - [ArrayKeywords](#dfjay-jsonschema-ArrayKeywords)
    - [EnumOptions](#dfjay-jsonschema-EnumOptions)
    - [EnumValueOptions](#dfjay-jsonschema-EnumValueOptions)
    - [FieldOptions](#dfjay-jsonschema-FieldOptions)
    - [FileOptions](#dfjay-jsonschema-FileOptions)
    - [MessageOptions](#dfjay-jsonschema-MessageOptions)
    - [NumericKeywords](#dfjay-jsonschema-NumericKeywords)
    - [ObjectKeywords](#dfjay-jsonschema-ObjectKeywords)
    - [PluginOptions](#dfjay-jsonschema-PluginOptions)
    - [StringKeywords](#dfjay-jsonschema-StringKeywords)
  
    - [Draft](#dfjay-jsonschema-Draft)
    - [EnumOptions.MappingType](#dfjay-jsonschema-EnumOptions-MappingType)
    - [PluginAdditionalProperties](#dfjay-jsonschema-PluginAdditionalProperties)
    - [ProtojsonMode](#dfjay-jsonschema-ProtojsonMode)
  
    - [File-level Extensions](#dfjay_jsonschema_options-proto-extensions)
    - [File-level Extensions](#dfjay_jsonschema_options-proto-extensions)
    - [File-level Extensions](#dfjay_jsonschema_options-proto-extensions)
    - [File-level Extensions](#dfjay_jsonschema_options-proto-extensions)
    - [File-level Extensions](#dfjay_jsonschema_options-proto-extensions)
  
- [Scalar Value Types](#scalar-value-types)



<a name="dfjay_jsonschema_options-proto"></a>
<p align="right"><a href="#top">Top</a></p>

## dfjay/jsonschema/options.proto



<a name="dfjay-jsonschema-ArrayKeywords"></a>

### ArrayKeywords



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| min_items | [uint32](#uint32) | optional |  |
| max_items | [uint32](#uint32) | optional |  |
| unique_items | [bool](#bool) | optional |  |






<a name="dfjay-jsonschema-EnumOptions"></a>

### EnumOptions



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| mapping_type | [EnumOptions.MappingType](#dfjay-jsonschema-EnumOptions-MappingType) |  |  |
| title | [string](#string) |  |  |
| description | [string](#string) |  |  |






<a name="dfjay-jsonschema-EnumValueOptions"></a>

### EnumValueOptions



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| custom_value | [google.protobuf.Any](#google-protobuf-Any) |  |  |






<a name="dfjay-jsonschema-FieldOptions"></a>

### FieldOptions



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| visibility_level | [uint32](#uint32) |  | Field visibility level. The field is omitted when this value is below the visibility_level plugin option. default: 0 |
| title | [string](#string) |  |  |
| description | [string](#string) |  |  |
| nullable | [bool](#bool) |  |  |
| required | [bool](#bool) | optional | required explicitly controls whether this field is required in the output JSON Schema. When set to true, the field is always required regardless of other options. When set to false, the field is never required regardless of other options. When not set, the default behavior is used (based on plugin options).

default: not set |
| examples | [google.protobuf.Value](#google-protobuf-Value) | repeated | Example values of the field in its JSON form, written as `examples`: an array for a repeated field, an object for a map. Needs draft-06 or later. |
| default | [google.protobuf.Any](#google-protobuf-Any) |  |  |
| array | [ArrayKeywords](#dfjay-jsonschema-ArrayKeywords) |  |  |
| numeric | [NumericKeywords](#dfjay-jsonschema-NumericKeywords) |  |  |
| string | [StringKeywords](#dfjay-jsonschema-StringKeywords) |  |  |






<a name="dfjay-jsonschema-FileOptions"></a>

### FileOptions



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| visibility_level | [uint32](#uint32) |  | Reserved for file-level visibility configuration; currently unused. |
| entrypoint_message | [string](#string) |  | Simple name of the top-level message in this file that the schema describes. It takes precedence over the entrypoint_message plugin option. default: the entrypoint_message plugin option |
| title | [string](#string) |  |  |
| description | [string](#string) |  |  |






<a name="dfjay-jsonschema-MessageOptions"></a>

### MessageOptions



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| visibility_level | [uint32](#uint32) |  | Message visibility level. The message is omitted when this value is below the visibility_level plugin option. default: 0 |
| title | [string](#string) |  |  |
| description | [string](#string) |  |  |
| entrypoint | [bool](#bool) |  | Write a schema of this message to its own file: Form in form.proto gives form.Form.schema.json, a nested Outer.Inner gives form.Outer.Inner.schema.json. Any number of messages can set it, alongside entrypoint_message. Skipped when hidden by visibility_level. default: false |
| examples | [google.protobuf.Value](#google-protobuf-Value) | repeated | Example values of the message in its JSON form, written as `examples`. Needs draft-06 or later. |
| object | [ObjectKeywords](#dfjay-jsonschema-ObjectKeywords) |  |  |






<a name="dfjay-jsonschema-NumericKeywords"></a>

### NumericKeywords



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| inclusive_minimum | [double](#double) |  |  |
| exclusive_minimum | [double](#double) |  |  |
| inclusive_maximum | [double](#double) |  |  |
| exclusive_maximum | [double](#double) |  |  |
| multiple_of | [int32](#int32) | optional |  |






<a name="dfjay-jsonschema-ObjectKeywords"></a>

### ObjectKeywords



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| additional_properties | [bool](#bool) | optional |  |
| min_properties | [uint32](#uint32) | optional | repeated JsonSchema additional_properties = 10; |
| max_properties | [uint32](#uint32) | optional |  |






<a name="dfjay-jsonschema-PluginOptions"></a>

### PluginOptions
Command-line plugin options. Defaults are applied by GetPluginOptions.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| visibility_level | [uint32](#uint32) |  | Minimum visibility level for messages and fields. Descriptors with a lower annotated level are omitted. A visible reference to a hidden type is an error. default: 0 |
| entrypoint_message | [string](#string) |  | entrypoint_message is the simple name of a top-level message of the input file, whose schema is written to &lt;file&gt;.schema.json. Messages can also be marked with (dfjay.jsonschema.message).entrypoint, instead or as well. The file option (dfjay.jsonschema.file).entrypoint_message takes precedence over this one.

default: none example: - --jsonschema_opt=entrypoint_message=MyMessage |
| output_file_suffix | [string](#string) |  | Output filename suffix. Must end with &#39;.json&#39; or &#39;.yaml&#39; or &#39;.yml&#39;.

default: .schema.json example: - --jsonschema_opt=output_file_suffix=.schema.json - --jsonschema_opt=output_file_suffix=.schema.yaml |
| pretty_json_output | [bool](#bool) |  | Pretty-print JSON output. Applies to any suffix ending in &#39;.json&#39;; ignored for YAML output.

default: true example: - --jsonschema_opt=pretty_json_output=true - --jsonschema_opt=pretty_json_output=false |
| draft | [Draft](#dfjay-jsonschema-Draft) |  | draft is used to determine which draft version should be used. The value should be one of Draft04, Draft06, Draft07, Draft201909, Draft202012.

default: Draft202012 example: - --jsonschema_opt=draft=Draft202012 |
| mandatory_nullable | [bool](#bool) |  | Disable automatic nullability for optional fields and oneof members when true. By default, those fields accept null as well as their value type. This does not control whether a field may be absent. Explicit nullable annotations still apply. Ignored in Lenient mode.

default: false example: - --jsonschema_opt=mandatory_nullable=true - --jsonschema_opt=mandatory_nullable=false |
| int64_as_string | [bool](#bool) |  | Deprecated alias for respect_protojson_int64. If both options are supplied, int64_as_string takes precedence. Ignored in Lenient mode.

default: true example: - --jsonschema_opt=int64_as_string=true - --jsonschema_opt=int64_as_string=false |
| preserve_proto_field_names | [bool](#bool) |  | preserve_proto_field_names is used to determine if output json field names should be identical to the proto field names. Otherwise field names either use the value of the `json_name` field option or they are automatically converted to lowerCamelCase. This default behaviour mirrors the behaviour of Protobuf&#39;s canonical JSON format (ProtoJSON).

default: false example: - --jsonschema_opt=preserve_proto_field_names=true - --jsonschema_opt=preserve_proto_field_names=false |
| additional_properties | [PluginAdditionalProperties](#dfjay-jsonschema-PluginAdditionalProperties) |  | Control additionalProperties on message schemas. AlwaysTrue and AlwaysFalse override message annotations; DefaultTrue and DefaultFalse apply only where no annotation is set. DoNothing preserves annotations without adding a default.

default: DoNothing example: - --jsonschema_opt=additional_properties=AlwaysTrue - --jsonschema_opt=additional_properties=AlwaysFalse - --jsonschema_opt=additional_properties=DefaultTrue - --jsonschema_opt=additional_properties=DefaultFalse - --jsonschema_opt=additional_properties=DoNothing |
| respect_protojson_presence | [bool](#bool) |  | respect_protojson_presence follows the ProtoJSON rule that a parser accepts a message with any field absent. If set to true, the plugin adds to `required` only fields that must be present anyway: fields with the `required` field option set to true, proto2 `required` and editions `LEGACY_REQUIRED` fields, and fields with the REQUIRED `google.api.field_behavior`. If set to false, every field that is not optional, repeated, a map or a oneof member is required as well.

default: true example: - --jsonschema_opt=respect_protojson_presence=true - --jsonschema_opt=respect_protojson_presence=false |
| respect_protojson_int64 | [bool](#bool) |  | respect_protojson_int64 writes 64-bit integer fields (int64, uint64, sint64, fixed64, sfixed64 and their wrappers) as strings, as ProtoJSON does. If set to false, they are integers, for JSON readers that parse large numbers themselves; JavaScript numbers lose precision above 2^53.

default: true example: - --jsonschema_opt=respect_protojson_int64=true - --jsonschema_opt=respect_protojson_int64=false |
| respect_field_behavior | [bool](#bool) |  | respect_field_behavior applies the `google.api.field_behavior` field option (google/api/field_behavior.proto): a REQUIRED field outside a oneof is required, an OUTPUT_ONLY field is `readOnly` and an INPUT_ONLY field is `writeOnly`. `readOnly` and `writeOnly` need draft-07 or later. The `required` field option takes precedence over REQUIRED.

default: true example: - --jsonschema_opt=respect_field_behavior=true - --jsonschema_opt=respect_field_behavior=false |
| protojson_mode | [ProtojsonMode](#dfjay-jsonschema-ProtojsonMode) |  | protojson_mode chooses which JSON the schema accepts. Canonical accepts what protobuf&#39;s ProtoJSON serializer writes. Lenient broadens the schema to accept parser-compatible representations: null for any field, integers and floats as numbers or strings, enum values as names or numbers, URL-safe and unpadded base64, and integer map keys with leading zeros or a plus sign. Fields with numeric keywords accept only numbers. In Lenient, mandatory_nullable and respect_protojson_int64 have no effect; respect_protojson_presence still decides which fields are required. Field names stay as configured: parsers also accept the proto field name, but the schema does not.

Use Canonical to generate types, document an API or check protobuf output; use Lenient to check JSON or YAML that people write and a protobuf parser reads, such as config files.

default: Canonical example: - --jsonschema_opt=protojson_mode=Canonical - --jsonschema_opt=protojson_mode=Lenient |
| enum_value_descriptions | [bool](#bool) |  | enum_value_descriptions writes an enum as an `anyOf` of one `const` per value instead of `enum`, so each value carries its comment as `description` and, in draft 2019-09 and later, `deprecated` when the value is deprecated. Editors show them when completing the value. Some code generators handle `enum` better.

default: false example: - --jsonschema_opt=enum_value_descriptions=true |
| respect_buf_validate | [bool](#bool) |  | respect_buf_validate applies protovalidate rules (`buf.validate.field`, `buf.validate.oneof` and `buf.validate.message` from buf/validate/validate.proto) that JSON Schema can express: `required`, numeric bounds, string lengths, patterns and formats, enum and repeated and map rules, `ignore`, and oneof rules. CEL rules, rules for integer map keys, patterns with no ECMA-262 form, and bytes, duration, timestamp, any and field_mask rules are skipped. A `numeric`, `string` or `array` field option replaces the protovalidate rules of the same kind. A field without presence whose zero value the rules reject becomes required.

default: true example: - --jsonschema_opt=respect_buf_validate=true - --jsonschema_opt=respect_buf_validate=false |






<a name="dfjay-jsonschema-StringKeywords"></a>

### StringKeywords



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| min_length | [uint32](#uint32) | optional |  |
| max_length | [uint32](#uint32) | optional |  |
| pattern | [string](#string) |  |  |
| format | [string](#string) |  |  |





 


<a name="dfjay-jsonschema-Draft"></a>

### Draft


| Name | Number | Description |
| ---- | ------ | ----------- |
| DraftDefault | 0 |  |
| Draft04 | 1 |  |
| Draft06 | 3 |  |
| Draft07 | 4 |  |
| Draft201909 | 5 |  |
| Draft202012 | 6 |  |



<a name="dfjay-jsonschema-EnumOptions-MappingType"></a>

### EnumOptions.MappingType


| Name | Number | Description |
| ---- | ------ | ----------- |
| MapToString | 0 |  |
| MapToNumber | 1 |  |
| MapToCustom | 2 |  |



<a name="dfjay-jsonschema-PluginAdditionalProperties"></a>

### PluginAdditionalProperties


| Name | Number | Description |
| ---- | ------ | ----------- |
| DefaultAdditionalProperties | 0 |  |
| AlwaysTrue | 1 | always_true: all messages to have additional_properties set to true |
| AlwaysFalse | 2 | always_false: all messages to have additional_properties set to false |
| DefaultTrue | 3 | default_true: all messages will have additional_properties set to true if not defined |
| DefaultFalse | 4 | default_false: all messages will have additional_properties set to false if not defined |
| DoNothing | 5 | do_nothing: do not set additional_properties default value |



<a name="dfjay-jsonschema-ProtojsonMode"></a>

### ProtojsonMode


| Name | Number | Description |
| ---- | ------ | ----------- |
| ProtojsonModeDefault | 0 |  |
| Canonical | 1 | Canonical: accept what protobuf writes |
| Lenient | 2 | Lenient: accept what protobuf parsers accept |


 


<a name="dfjay_jsonschema_options-proto-extensions"></a>

### File-level Extensions
| Extension | Type | Base | Number | Description |
| --------- | ---- | ---- | ------ | ----------- |
| enum | EnumOptions | .google.protobuf.EnumOptions | 11344 |  |
| enum_value | EnumValueOptions | .google.protobuf.EnumValueOptions | 11345 |  |
| field | FieldOptions | .google.protobuf.FieldOptions | 11343 |  |
| file | FileOptions | .google.protobuf.FileOptions | 11341 |  |
| message | MessageOptions | .google.protobuf.MessageOptions | 11342 |  |

 

 



## Scalar Value Types

| .proto Type | Notes | C++ | Java | Python | Go | C# | PHP | Ruby |
| ----------- | ----- | --- | ---- | ------ | -- | -- | --- | ---- |
| <a name="double" /> double |  | double | double | float | float64 | double | float | Float |
| <a name="float" /> float |  | float | float | float | float32 | float | float | Float |
| <a name="int32" /> int32 | Uses variable-length encoding. Inefficient for encoding negative numbers – if your field is likely to have negative values, use sint32 instead. | int32 | int | int | int32 | int | integer | Bignum or Fixnum (as required) |
| <a name="int64" /> int64 | Uses variable-length encoding. Inefficient for encoding negative numbers – if your field is likely to have negative values, use sint64 instead. | int64 | long | int/long | int64 | long | integer/string | Bignum |
| <a name="uint32" /> uint32 | Uses variable-length encoding. | uint32 | int | int/long | uint32 | uint | integer | Bignum or Fixnum (as required) |
| <a name="uint64" /> uint64 | Uses variable-length encoding. | uint64 | long | int/long | uint64 | ulong | integer/string | Bignum or Fixnum (as required) |
| <a name="sint32" /> sint32 | Uses variable-length encoding. Signed int value. These more efficiently encode negative numbers than regular int32s. | int32 | int | int | int32 | int | integer | Bignum or Fixnum (as required) |
| <a name="sint64" /> sint64 | Uses variable-length encoding. Signed int value. These more efficiently encode negative numbers than regular int64s. | int64 | long | int/long | int64 | long | integer/string | Bignum |
| <a name="fixed32" /> fixed32 | Always four bytes. More efficient than uint32 if values are often greater than 2^28. | uint32 | int | int | uint32 | uint | integer | Bignum or Fixnum (as required) |
| <a name="fixed64" /> fixed64 | Always eight bytes. More efficient than uint64 if values are often greater than 2^56. | uint64 | long | int/long | uint64 | ulong | integer/string | Bignum |
| <a name="sfixed32" /> sfixed32 | Always four bytes. | int32 | int | int | int32 | int | integer | Bignum or Fixnum (as required) |
| <a name="sfixed64" /> sfixed64 | Always eight bytes. | int64 | long | int/long | int64 | long | integer/string | Bignum |
| <a name="bool" /> bool |  | bool | boolean | boolean | bool | bool | boolean | TrueClass/FalseClass |
| <a name="string" /> string | A string must always contain UTF-8 encoded or 7-bit ASCII text. | string | String | str/unicode | string | string | string | String (UTF-8) |
| <a name="bytes" /> bytes | May contain any arbitrary sequence of bytes. | string | ByteString | str | []byte | ByteString | string | String (ASCII-8BIT) |

