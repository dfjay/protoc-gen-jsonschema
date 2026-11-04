package modules

import (
	"encoding/json"
	"iter"
	"math"
	"reflect"
	"slices"

	"github.com/dfjay/protoc-gen-jsonschema/pkg/jsonschema"
	"github.com/dfjay/protoc-gen-jsonschema/pkg/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/structpb"
)

func buildFromMessage(pluginOptions *proto.PluginOptions, ext extensions, message protoreflect.MessageDescriptor, mo *proto.MessageOptions) *jsonschema.Schema {
	schema := &jsonschema.Schema{}
	schema.Type = "object"
	schema.Title = proto.GetTitleOrEmpty(mo)
	schema.Description = proto.GetDescriptionOrComment(message, mo)
	schema.Deprecated = deprecated(message)
	schema.Examples = examples(mo.GetExamples())
	schema.Properties = jsonschema.NewOrderedMap[*jsonschema.Schema]()

	fillSchemaByObjectKeywords(pluginOptions, schema, mo.GetObject())

	for field := range listOf(message.Fields()) {
		if isHiddenField(pluginOptions, field) {
			continue
		}

		propName := toPropertyName(field, pluginOptions.GetPreserveProtoFieldNames())
		fieldSchema := &jsonschema.Schema{Ref: toRefId(field)}
		// Parsers accept null for any field and leave it unset, except a
		// repeated Value or NullValue, where null is read as the list. An
		// unset field may fail protovalidate.
		nullable := proto.GetFieldOptions(field).GetNullable() || !ext.validate.requiresValue(field) &&
			(isLenient(pluginOptions) && (!field.IsList() || !isNullableKind(field)) ||
				!pluginOptions.GetMandatoryNullable() && (inRealOneof(field) || isOptionalField(field)))
		if nullable && !acceptsNull(field) {
			fieldSchema = &jsonschema.Schema{OneOf: []*jsonschema.Schema{
				{Type: "null"},
				fieldSchema,
			}}
		}

		if isRequiredField(pluginOptions, ext, field) {
			// If field is not a member of oneOf
			schema.Required = append(schema.Required, propName)
		}

		schema.Properties.Set(propName, fieldSchema)
	}

	// Convert Protobuf OneOfs to JSONSchema keywords. Synthetic oneofs
	// (proto3 optional fields) constrain nothing.
	for oneOf := range listOf(message.Oneofs()) {
		if oneOf.IsSynthetic() {
			continue
		}
		required := ruleBool(extensionMessage(oneOf, ext.validate.oneof), "required")
		if exclusive := exclusiveFields(pluginOptions, slices.Collect(listOf(oneOf.Fields())), required, required); exclusive != nil {
			schema.AllOf = append(schema.AllOf, exclusive)
		}
	}
	for _, rule := range ext.validate.messageOneofs(message) {
		var fields []protoreflect.FieldDescriptor
		for _, name := range rule.fields {
			if field := message.Fields().ByName(name); field != nil {
				fields = append(fields, field)
			}
		}
		if exclusive := exclusiveFields(pluginOptions, fields, rule.required, true); exclusive != nil {
			schema.AllOf = append(schema.AllOf, exclusive)
		}
	}
	return schema
}

// exclusiveFields returns a schema that allows at most one of fields to be
// set, or exactly one if required, or nil if that constrains nothing.
func exclusiveFields(pluginOptions *proto.PluginOptions, fields []protoreflect.FieldDescriptor, required, protovalidate bool) *jsonschema.Schema {
	var sets []*jsonschema.Schema
	for _, field := range fields {
		if isHiddenField(pluginOptions, field) {
			continue
		}
		propName := toPropertyName(field, pluginOptions.GetPreserveProtoFieldNames())
		set := &jsonschema.Schema{Required: []string{propName}}
		// A member set to null is unset, so another may be set, in Lenient
		// mode and for protovalidate's rules.
		if unset := unsetSchema(pluginOptions, field, isLenient(pluginOptions) || protovalidate); unset != nil {
			set.Properties = jsonschema.NewOrderedMap[*jsonschema.Schema]()
			set.Properties.Set(propName, &jsonschema.Schema{Not: unset})
		}
		sets = append(sets, set)
	}
	switch {
	case len(sets) == 0 || len(sets) == 1 && !required:
		return nil
	case len(sets) == 1:
		return sets[0]
	case required:
		return &jsonschema.Schema{OneOf: sets}
	}
	none := &jsonschema.Schema{Not: &jsonschema.Schema{AnyOf: slices.Clone(sets)}}
	return &jsonschema.Schema{OneOf: append(sets, none)}
}

// isRequiredField reports whether field goes into required. In order of
// precedence: the required field option; a proto2 required or editions
// LEGACY_REQUIRED field; protovalidate (see requiresValue) or the REQUIRED
// field behavior; and, unless respect_protojson_presence is set, every
// field that is not optional, repeated, a map or in a oneof.
func isRequiredField(pluginOptions *proto.PluginOptions, ext extensions, field protoreflect.FieldDescriptor) bool {
	if fo := proto.GetFieldOptions(field); fo != nil && fo.Required != nil {
		return fo.GetRequired()
	}
	if field.Cardinality() == protoreflect.Required {
		return true
	}
	// For protovalidate, a required oneof member must be the one set.
	if ext.validate.requiresValue(field) || !inRealOneof(field) && ext.behaviors.has(field, "REQUIRED") {
		return true
	}
	if pluginOptions.GetRespectProtojsonPresence() {
		// Per ProtoJSON spec, parsers must accept absent fields and use default values,
		// so no field is required. See https://protobuf.dev/programming-guides/json/
		return false
	}
	return !inRealOneof(field) && !isOptionalField(field) && !field.IsList() && !field.IsMap()
}

func buildFromWellKnownMessage(pluginOptions *proto.PluginOptions, ext extensions, message protoreflect.MessageDescriptor, mo *proto.MessageOptions) *jsonschema.Schema {
	baseSchema := buildFromMessage(pluginOptions, ext, message, mo)

	wellKnownType := getWellKnownMessageType(message)
	if wellKnownType == WellKnownMessageTypeNone {
		panic("not well known type")
	}
	switch wellKnownType {
	case WellKnownMessageTypeK8sIntOrString:
		schema := &jsonschema.Schema{}
		schema.Title = baseSchema.Title
		schema.Description = baseSchema.Description
		schema.Deprecated = baseSchema.Deprecated
		schema.Examples = baseSchema.Examples
		schema.OneOf = []*jsonschema.Schema{
			{Type: "string"},
			{Type: "integer"},
		}
		return schema
	case WellKnownMessageTypeK8sVolume:
		baseSchema.OneOf = []*jsonschema.Schema{{Ref: ".k8s.io.api.core.v1.VolumeSource"}}
		baseSchema.Required = deletePropertyInRequired(baseSchema.Required, "volumeSource")
		baseSchema.Properties.Delete("volumeSource")
	case WellKnownMessageTypeK8sSecretProjection:
		baseSchema.OneOf = []*jsonschema.Schema{{Ref: ".k8s.io.api.core.v1.LocalObjectReference"}}
		baseSchema.Required = deletePropertyInRequired(baseSchema.Required, "localObjectReference")
		baseSchema.Properties.Delete("localObjectReference")
	case WellKnownMessageTypeK8sConfigMapVolumeSource:
		baseSchema.OneOf = []*jsonschema.Schema{{Ref: ".k8s.io.api.core.v1.LocalObjectReference"}}
		baseSchema.Required = deletePropertyInRequired(baseSchema.Required, "localObjectReference")
		baseSchema.Properties.Delete("localObjectReference")
	case WellKnownMessageTypeK8sConfigMapProjection:
		baseSchema.OneOf = []*jsonschema.Schema{{Ref: ".k8s.io.api.core.v1.LocalObjectReference"}}
		baseSchema.Required = deletePropertyInRequired(baseSchema.Required, "localObjectReference")
		baseSchema.Properties.Delete("localObjectReference")
	case WellKnownMessageTypeK8sConfigMapKeySelector:
		baseSchema.OneOf = []*jsonschema.Schema{{Ref: ".k8s.io.api.core.v1.LocalObjectReference"}}
		baseSchema.Required = deletePropertyInRequired(baseSchema.Required, "localObjectReference")
		baseSchema.Properties.Delete("localObjectReference")
	case WellKnownMessageTypeK8sSecretKeySelector:
		baseSchema.OneOf = []*jsonschema.Schema{{Ref: ".k8s.io.api.core.v1.LocalObjectReference"}}
		baseSchema.Required = deletePropertyInRequired(baseSchema.Required, "localObjectReference")
		baseSchema.Properties.Delete("localObjectReference")
	case WellKnownMessageTypeK8sConfigMapEnvSource:
		baseSchema.OneOf = []*jsonschema.Schema{{Ref: ".k8s.io.api.core.v1.LocalObjectReference"}}
		baseSchema.Required = deletePropertyInRequired(baseSchema.Required, "localObjectReference")
		baseSchema.Properties.Delete("localObjectReference")
	case WellKnownMessageTypeK8sSecretEnvSource:
		baseSchema.OneOf = []*jsonschema.Schema{{Ref: ".k8s.io.api.core.v1.LocalObjectReference"}}
		baseSchema.Required = deletePropertyInRequired(baseSchema.Required, "localObjectReference")
		baseSchema.Properties.Delete("localObjectReference")
	case WellKnownMessageTypeK8sProbe:
		baseSchema.OneOf = []*jsonschema.Schema{{Ref: ".k8s.io.api.core.v1.ProbeHandler"}}
		baseSchema.Required = deletePropertyInRequired(baseSchema.Required, "handler")
		baseSchema.Properties.Delete("handler")
	case WellKnownMessageTypeK8sEphemeralContainer:
		baseSchema.OneOf = []*jsonschema.Schema{{Ref: ".k8s.io.api.core.v1.EphemeralContainerCommon"}}
		baseSchema.Required = deletePropertyInRequired(baseSchema.Required, "ephemeralContainerCommon")
		baseSchema.Properties.Delete("ephemeralContainerCommon")
	}
	return baseSchema
}

type WellKnownMessageType int

const (
	WellKnownMessageTypeNone WellKnownMessageType = iota
	WellKnownMessageTypeK8sIntOrString
	WellKnownMessageTypeK8sVolume
	WellKnownMessageTypeK8sSecretProjection
	WellKnownMessageTypeK8sConfigMapVolumeSource
	WellKnownMessageTypeK8sConfigMapProjection
	WellKnownMessageTypeK8sConfigMapKeySelector
	WellKnownMessageTypeK8sSecretKeySelector
	WellKnownMessageTypeK8sConfigMapEnvSource
	WellKnownMessageTypeK8sSecretEnvSource
	WellKnownMessageTypeK8sProbe
	WellKnownMessageTypeK8sEphemeralContainer
)

func isWellKnownMessage(message protoreflect.MessageDescriptor) bool {
	return getWellKnownMessageType(message) != WellKnownMessageTypeNone
}

func getWellKnownMessageType(message protoreflect.MessageDescriptor) WellKnownMessageType {
	switch fullyQualifiedName(message) {
	case ".k8s.io.apimachinery.pkg.util.intstr.IntOrString":
		return WellKnownMessageTypeK8sIntOrString
	case ".k8s.io.api.core.v1.Volume":
		return WellKnownMessageTypeK8sVolume
	case ".k8s.io.api.core.v1.SecretProjection":
		return WellKnownMessageTypeK8sSecretProjection
	case ".k8s.io.api.core.v1.ConfigMapVolumeSource":
		return WellKnownMessageTypeK8sConfigMapVolumeSource
	case ".k8s.io.api.core.v1.ConfigMapProjection":
		return WellKnownMessageTypeK8sConfigMapProjection
	case ".k8s.io.api.core.v1.ConfigMapKeySelector":
		return WellKnownMessageTypeK8sConfigMapKeySelector
	case ".k8s.io.api.core.v1.SecretKeySelector":
		return WellKnownMessageTypeK8sSecretKeySelector
	case ".k8s.io.api.core.v1.ConfigMapEnvSource":
		return WellKnownMessageTypeK8sConfigMapEnvSource
	case ".k8s.io.api.core.v1.SecretEnvSource":
		return WellKnownMessageTypeK8sSecretEnvSource
	case ".k8s.io.api.core.v1.Probe":
		return WellKnownMessageTypeK8sProbe
	case ".k8s.io.api.core.v1.EphemeralContainer":
		return WellKnownMessageTypeK8sEphemeralContainer
	}

	return WellKnownMessageTypeNone
}

// buildFromField builds the schema of a field that is not a map, with its
// protovalidate FieldRules, if any.
func buildFromField(pluginOptions *proto.PluginOptions, field protoreflect.FieldDescriptor, fo *proto.FieldOptions, rules protoreflect.Message) *jsonschema.Schema {
	valueRules := rules
	if field.IsList() {
		valueRules = activeRules(ruleMessage(ruleMessage(rules, "repeated"), "items"))
	}
	schema := buildFromValue(pluginOptions, field, fo, valueRules)
	schema.Title = proto.GetTitleOrEmpty(fo)
	schema.Description = proto.GetDescriptionOrComment(field, fo)

	if field.IsList() {
		schema = wrapSchemaInArray(schema, field, fo)
		if repeated := ruleMessage(rules, "repeated"); fo.GetArray() == nil {
			if ruleHas(repeated, "min_items") {
				minItems := ruleSize(repeated, "min_items")
				if ignoresZero(rules) {
					// Or the empty list, the zero value the rules skip.
					schema.AllOf = append(schema.AllOf, &jsonschema.Schema{AnyOf: []*jsonschema.Schema{{MaxItems: new(0)}, {MinItems: minItems}}})
				} else {
					schema.MinItems = minItems
				}
			}
			if ruleHas(repeated, "max_items") {
				schema.MaxItems = ruleSize(repeated, "max_items")
			}
			if ruleBool(repeated, "unique") && jsonEqualIsProtoEqual(pluginOptions, field) {
				schema.UniqueItems = new(true)
			}
		}
	}
	schema.Deprecated = deprecated(field)
	schema.Examples = examples(fo.GetExamples())
	return schema
}

func buildFromMapField(pluginOptions *proto.PluginOptions, field protoreflect.FieldDescriptor, fo *proto.FieldOptions, rules protoreflect.Message) *jsonschema.Schema {
	mapRules := ruleMessage(rules, "map")
	schema := &jsonschema.Schema{}
	schema.Title = proto.GetTitleOrEmpty(fo)
	schema.Description = proto.GetDescriptionOrComment(field, fo)
	schema.Deprecated = deprecated(field)
	schema.Examples = examples(fo.GetExamples())
	schema.Type = "object"
	schema.PropertyNames = mapKeySchema(field.MapKey().Kind(), isLenient(pluginOptions))
	keyFieldRules := activeRules(ruleMessage(mapRules, "keys"))
	if keyRules := typeRules(keyFieldRules, protoreflect.StringKind); field.MapKey().Kind() == protoreflect.StringKind && keyRules != nil {
		schema.PropertyNames = &jsonschema.Schema{}
		applyStringRules(schema.PropertyNames, keyRules)
		if ignoresZero(keyFieldRules) {
			schema.PropertyNames = &jsonschema.Schema{AnyOf: []*jsonschema.Schema{{Const: new(any(""))}, schema.PropertyNames}}
		}
	}
	schema.AdditionalProperties = buildFromValue(pluginOptions, field.MapValue(), fo, activeRules(ruleMessage(mapRules, "values")))
	if ruleHas(mapRules, "min_pairs") {
		minPairs := ruleSize(mapRules, "min_pairs")
		if ignoresZero(rules) {
			// Or the empty map, the zero value the rules skip.
			schema.AllOf = append(schema.AllOf, &jsonschema.Schema{AnyOf: []*jsonschema.Schema{{MaxProperties: new(0)}, {MinProperties: minPairs}}})
		} else {
			schema.MinProperties = minPairs
		}
	}
	if ruleHas(mapRules, "max_pairs") {
		schema.MaxProperties = ruleSize(mapRules, "max_pairs")
	}
	return schema
}

// mapKeySchema returns the schema of the keys of a map: ProtoJSON writes
// integer and bool keys as strings.
func mapKeySchema(kind protoreflect.Kind, lenient bool) *jsonschema.Schema {
	switch {
	case kind == protoreflect.BoolKind:
		return &jsonschema.Schema{Enum: []any{"true", "false"}}
	case lenient && isUnsignedKind(kind):
		// Parsers read keys with strconv.ParseUint and ParseInt, which
		// allow leading zeros and, for signed keys, a plus sign.
		return &jsonschema.Schema{Pattern: "^[0-9]+$"}
	case lenient && isIntKind(kind):
		return &jsonschema.Schema{Pattern: "^[+-]?[0-9]+$"}
	case isIntKind(kind):
		return &jsonschema.Schema{Pattern: integerPattern(kind)}
	}
	return nil
}

// integerPattern matches an integer of kind written as a decimal string.
func integerPattern(kind protoreflect.Kind) string {
	if isUnsignedKind(kind) {
		return "^(0|[1-9][0-9]*)$"
	}
	return "^-?(0|[1-9][0-9]*)$"
}

// lenientIntegerPattern matches the integer strings parsers accept: JSON
// numbers with an integral value, like "-5", "1.0" or "1e2". Unsigned
// integers allow only "-0" with a sign.
func lenientIntegerPattern(kind protoreflect.Kind) string {
	if isUnsignedKind(kind) {
		return `^(-?0|[1-9][0-9]*)(\.0+)?([eE]\+?[0-9]+)?$`
	}
	return `^-?(0|[1-9][0-9]*)(\.0+)?([eE]\+?[0-9]+)?$`
}

// lenientInteger is the schema of an integer of kind written as a number,
// within the range of 32-bit kinds and not negative for unsigned ones. The
// limits of 64-bit kinds are not exact in a JSON number.
func lenientInteger(kind protoreflect.Kind) *jsonschema.Schema {
	schema := &jsonschema.Schema{Type: "integer"}
	switch kind {
	case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sfixed32Kind, protoreflect.EnumKind:
		schema.Minimum, schema.Maximum = new(float64(math.MinInt32)), new(float64(math.MaxInt32))
	case protoreflect.Uint32Kind, protoreflect.Fixed32Kind:
		schema.Minimum, schema.Maximum = new(0.0), new(float64(math.MaxUint32))
	case protoreflect.Uint64Kind, protoreflect.Fixed64Kind:
		schema.Minimum = new(0.0)
	}
	return schema
}

// acceptsNull reports whether the schema of field already accepts null:
// ProtoJSON writes a NullValue and a null Value as null.
func acceptsNull(field protoreflect.FieldDescriptor) bool {
	return !field.IsList() && !field.IsMap() && isNullableKind(field)
}

// isNullableKind reports whether field holds a NullValue or a Value.
func isNullableKind(field protoreflect.FieldDescriptor) bool {
	switch {
	case field.Kind() == protoreflect.EnumKind:
		return field.Enum().FullName() == "google.protobuf.NullValue"
	case isMessageKind(field.Kind()):
		return field.Message().FullName() == "google.protobuf.Value"
	}
	return false
}

// buildFromValue builds the schema of one value of field as ProtoJSON
// writes it: the field itself, an element of a repeated field or the value
// of a map, with the protovalidate FieldRules of that value, if any.
func buildFromValue(pluginOptions *proto.PluginOptions, field protoreflect.FieldDescriptor, fo *proto.FieldOptions, rules protoreflect.Message) *jsonschema.Schema {
	schema, applied := buildValue(pluginOptions, field, fo, rules)
	// Elements, which a list field or a map entry holds, have no presence.
	if applied && ignoresZero(rules) && (!field.HasPresence() || field.ContainingMessage().IsMapEntry()) {
		return &jsonschema.Schema{AnyOf: []*jsonschema.Schema{zeroValue(pluginOptions, field), schema}}
	}
	return schema
}

// buildValue builds the schema of a value with the protovalidate rules of
// its type, unless field options of the same kind replace them, and reports
// whether rules apply.
func buildValue(pluginOptions *proto.PluginOptions, field protoreflect.FieldDescriptor, fo *proto.FieldOptions, fieldRules protoreflect.Message) (*jsonschema.Schema, bool) {
	schema := &jsonschema.Schema{}
	kind := field.Kind()
	rules := typeRules(fieldRules, kind)
	numericRules := rules
	if fo.GetNumeric() != nil {
		numericRules = nil
	}
	switch {
	case isMessageKind(kind):
		if value := wrappedValue(field.Message()); value != nil {
			// A wrapper is written as its value.
			return buildValue(pluginOptions, value, fo, fieldRules)
		}
		if wkt := buildFromWellKnownType(pluginOptions, field.Message(), fo); wkt != nil {
			return wkt, false
		}
		// The message's rules apply, except to an empty element under
		// IGNORE_IF_ZERO_VALUE.
		schema.Ref = toRefId(field.Message())
		return schema, true
	case kind == protoreflect.EnumKind:
		if field.Enum().FullName() == "google.protobuf.NullValue" {
			return nullValue(pluginOptions, field.Enum(), rules), rules != nil
		}
		schema.Ref = toRefId(field.Enum())
		applyEnumRules(pluginOptions, schema, field.Enum(), rules)
		return schema, rules != nil
	case isIntKind(kind):
		if isLenient(pluginOptions) && fo.GetNumeric() == nil && !hasNumericRules(numericRules) {
			// Parsers accept integers as numbers and as strings.
			return &jsonschema.Schema{AnyOf: []*jsonschema.Schema{
				lenientInteger(kind),
				{Type: "string", Pattern: lenientIntegerPattern(kind)},
			}}, false
		}
		if pluginOptions.GetRespectProtojsonInt64() && isInt64(kind) && !isLenient(pluginOptions) {
			// Not format "int64": no draft defines it, and validators such
			// as Ajv refuse unknown formats.
			schema.Type = "string"
			schema.Pattern = integerPattern(kind)
			applyNumericRules(schema, numericRules, true)
		} else {
			if isLenient(pluginOptions) && numericRules != nil {
				// Parsers reject numbers beyond the range of kind.
				schema = lenientInteger(kind)
			}
			schema.Type = "integer"
			fillSchemaByNumericKeywords(schema, fo.GetNumeric())
			applyNumericRules(schema, numericRules, false)
		}
		return schema, numericRules != nil
	case isNumericKind(kind):
		schema.Type = "number"
		if fo.GetNumeric() != nil || isLenient(pluginOptions) && hasNumericRules(numericRules) {
			// Numeric keywords bound the value, which rules out NaN, the
			// infinities and numbers in strings.
			fillSchemaByNumericKeywords(schema, fo.GetNumeric())
			applyNumericRules(schema, numericRules, false)
			return schema, numericRules != nil
		}
		// ProtoJSON writes NaN and the infinities as strings.
		applyNumericRules(schema, numericRules, false)
		specials := floatSpecials(numericRules)
		if len(specials) == 0 {
			return schema, numericRules != nil
		}
		anyOf := []*jsonschema.Schema{schema, {Type: "string", Enum: specials}}
		if isLenient(pluginOptions) {
			// Parsers also accept numbers in strings.
			anyOf = append(anyOf, &jsonschema.Schema{Type: "string", Pattern: numberPattern})
		}
		return &jsonschema.Schema{AnyOf: anyOf}, numericRules != nil
	case kind == protoreflect.BoolKind:
		schema.Type = "boolean"
		if ruleHas(rules, "const") {
			schema.Const = new(any(ruleGet(rules, "const").Bool()))
		}
		return schema, rules != nil
	case kind == protoreflect.StringKind:
		schema.Type = "string"
		fillSchemaByStringKeywords(schema, fo.GetString_())
		if fo.GetString_() == nil {
			applyStringRules(schema, rules)
			return schema, rules != nil
		}
	case kind == protoreflect.BytesKind:
		schema.Type = "string"
		schema.Pattern = base64Pattern
		if isLenient(pluginOptions) {
			schema.Pattern = lenientBase64Pattern
		}
		fillSchemaByStringKeywords(schema, fo.GetString_())
	}
	return schema, false
}

// unsetValue is the schema of the zero value of a field without presence.
func unsetValue(pluginOptions *proto.PluginOptions, field protoreflect.FieldDescriptor) *jsonschema.Schema {
	switch {
	case field.IsList():
		return &jsonschema.Schema{MaxItems: new(0)}
	case field.IsMap():
		return &jsonschema.Schema{MaxProperties: new(0)}
	}
	return zeroValue(pluginOptions, field)
}

// jsonEqualIsProtoEqual reports whether values of field are equal exactly
// when their JSON is, so uniqueItems matches protovalidate's unique: not for
// NaN, which equals nothing, enums with aliases or numbers sharing a custom
// value, nor in Lenient for numbers, enums and bytes, which have several
// forms.
func jsonEqualIsProtoEqual(pluginOptions *proto.PluginOptions, field protoreflect.FieldDescriptor) bool {
	kind := field.Kind()
	switch {
	case kind == protoreflect.FloatKind, kind == protoreflect.DoubleKind:
		return false
	case kind == protoreflect.EnumKind:
		var forms []any
		for value := range listOf(field.Enum().Values()) {
			for _, form := range enumJSONValues(pluginOptions, field.Enum(), value.Number()) {
				if containsValue(forms, form) {
					return false
				}
				forms = append(forms, form)
			}
		}
		return !isLenient(pluginOptions)
	case isLenient(pluginOptions):
		return !isIntKind(kind) && kind != protoreflect.BytesKind
	}
	return true
}

// wrappedValue returns the value field of a wrapper type such as
// Int32Value, or nil.
func wrappedValue(message protoreflect.MessageDescriptor) protoreflect.FieldDescriptor {
	switch message.FullName() {
	case "google.protobuf.DoubleValue", "google.protobuf.FloatValue",
		"google.protobuf.Int64Value", "google.protobuf.UInt64Value",
		"google.protobuf.Int32Value", "google.protobuf.UInt32Value",
		"google.protobuf.BoolValue", "google.protobuf.StringValue", "google.protobuf.BytesValue":
		return message.Fields().ByName("value")
	}
	return nil
}

// nullValue is the schema of NullValue, which ProtoJSON writes as null, with
// its enum rules: null and "NULL_VALUE" are 0, and Lenient takes any number.
func nullValue(pluginOptions *proto.PluginOptions, enum protoreflect.EnumDescriptor, rules protoreflect.Message) *jsonschema.Schema {
	var anyOf []*jsonschema.Schema
	if enumAccepts(rules, enum, 0) {
		anyOf = append(anyOf, &jsonschema.Schema{Type: "null"})
		if isLenient(pluginOptions) {
			anyOf = append(anyOf, &jsonschema.Schema{Type: "string", Enum: []any{"NULL_VALUE"}})
		}
	}
	if isLenient(pluginOptions) {
		numbers := lenientInteger(protoreflect.EnumKind)
		if rules != nil {
			// The same rules as an enum, on numbers only.
			numbers = &jsonschema.Schema{AllOf: []*jsonschema.Schema{numbers, enumNumberRules(enum, rules)}}
		}
		anyOf = append(anyOf, numbers)
	}
	switch len(anyOf) {
	case 0:
		return never()
	case 1:
		return anyOf[0]
	}
	return &jsonschema.Schema{AnyOf: anyOf}
}

// unsetSchema is the schema of a present property that leaves field unset:
// null if withNull, except for a Value or NullValue, where null is a value,
// and the zero value of a field without presence. It is nil if none does.
func unsetSchema(pluginOptions *proto.PluginOptions, field protoreflect.FieldDescriptor, withNull bool) *jsonschema.Schema {
	var forms []*jsonschema.Schema
	if withNull && !acceptsNull(field) && (!field.IsList() || !isNullableKind(field)) {
		forms = append(forms, &jsonschema.Schema{Type: "null"})
	}
	if !field.HasPresence() {
		forms = append(forms, unsetValue(pluginOptions, field))
	}
	switch len(forms) {
	case 0:
		return nil
	case 1:
		return forms[0]
	}
	return &jsonschema.Schema{AnyOf: forms}
}

// emptyMessage is the schema of message with no field set: the message
// without protovalidate rules, each property absent or unset. Whether null
// is allowed is up to the field's own schema.
func emptyMessage(pluginOptions *proto.PluginOptions, message protoreflect.MessageDescriptor) *jsonschema.Schema {
	// In allOf: draft-07 and earlier ignore the siblings of $ref.
	schema := &jsonschema.Schema{
		AllOf:      []*jsonschema.Schema{{Ref: jsonschema.RefId(unvalidatedPrefix) + toRefId(message)}},
		Properties: jsonschema.NewOrderedMap[*jsonschema.Schema](),
	}
	for field := range listOf(message.Fields()) {
		if isHiddenField(pluginOptions, field) {
			continue
		}
		property := unsetSchema(pluginOptions, field, true)
		if property == nil {
			property = never()
		}
		schema.Properties.Set(toPropertyName(field, pluginOptions.GetPreserveProtoFieldNames()), property)
	}
	return schema
}

// zeroValue is the schema of the zero value of field, which
// IGNORE_IF_ZERO_VALUE exempts from the rules.
func zeroValue(pluginOptions *proto.PluginOptions, field protoreflect.FieldDescriptor) *jsonschema.Schema {
	kind := field.Kind()
	if field.Message() != nil {
		if value := wrappedValue(field.Message()); value != nil {
			return zeroValue(pluginOptions, value)
		}
		return emptyMessage(pluginOptions, field.Message())
	}
	switch {
	case kind == protoreflect.EnumKind && field.Enum().FullName() == "google.protobuf.NullValue":
		if isLenient(pluginOptions) {
			return &jsonschema.Schema{AnyOf: []*jsonschema.Schema{{Type: "null"}, {Enum: []any{"NULL_VALUE", 0}}}}
		}
		return &jsonschema.Schema{Type: "null"}
	case kind == protoreflect.StringKind, kind == protoreflect.BytesKind:
		return &jsonschema.Schema{Const: new(any(""))}
	case kind == protoreflect.BoolKind:
		return &jsonschema.Schema{Const: new(any(false))}
	case kind == protoreflect.EnumKind:
		return &jsonschema.Schema{Enum: enumJSONValues(pluginOptions, field.Enum(), field.Enum().Values().Get(0).Number())}
	case pluginOptions.GetRespectProtojsonInt64() && isInt64(kind) && !isLenient(pluginOptions):
		return &jsonschema.Schema{Const: new(any("0"))}
	case isLenient(pluginOptions):
		// Parsers also read zero in a string, as "0" or "-0.0".
		return &jsonschema.Schema{AnyOf: []*jsonschema.Schema{
			{Const: new(any(0))},
			{Type: "string", Pattern: `^-?0(\.0+)?([eE][+-]?[0-9]+)?$`},
		}}
	}
	return &jsonschema.Schema{Const: new(any(0))}
}

const (
	// base64Pattern matches standard base64 with padding, as ProtoJSON
	// writes bytes.
	base64Pattern = "^(?:[A-Za-z0-9+/]{4})*(?:[A-Za-z0-9+/]{2}==|[A-Za-z0-9+/]{3}=)?$"
	// lenientBase64Pattern also matches the URL-safe alphabet and missing
	// padding, which parsers accept.
	lenientBase64Pattern = "^(?:(?:[A-Za-z0-9+/]{4})*(?:[A-Za-z0-9+/]{2}(?:==)?|[A-Za-z0-9+/]{3}=?)?" +
		"|(?:[A-Za-z0-9_-]{4})*(?:[A-Za-z0-9_-]{2}(?:==)?|[A-Za-z0-9_-]{3}=?)?)$"
	// numberPattern matches a JSON number.
	numberPattern = `^-?(0|[1-9][0-9]*)(\.[0-9]+)?([eE][+-]?[0-9]+)?$`
)

// isLenient reports whether the schema accepts everything ProtoJSON parsers
// accept rather than only what protobuf writes.
func isLenient(pluginOptions *proto.PluginOptions) bool {
	return pluginOptions.GetProtojsonMode() == proto.ProtojsonMode_Lenient
}

// buildFromWellKnownType returns the schema of a google.protobuf message
// with a special ProtoJSON form, or nil for any other message.
// See https://protobuf.dev/programming-guides/json/
func buildFromWellKnownType(pluginOptions *proto.PluginOptions, message protoreflect.MessageDescriptor, fo *proto.FieldOptions) *jsonschema.Schema {
	switch message.FullName() {
	case "google.protobuf.Timestamp":
		schema := &jsonschema.Schema{Type: "string", Format: "date-time"}
		fillSchemaByStringKeywords(schema, fo.GetString_())
		return schema
	case "google.protobuf.Duration":
		// Seconds with up to 9 fractional digits, like "1.5s"; not the
		// ISO 8601 duration of format "duration".
		schema := &jsonschema.Schema{Type: "string", Pattern: `^-?[0-9]+(\.[0-9]{1,9})?s$`}
		if isLenient(pluginOptions) {
			// Parsers also accept a plus sign and a missing integer or
			// fraction part, like "+1s", ".5s" or "1.s".
			schema.Pattern = `^[-+]?([0-9]+(\.[0-9]{0,9})?|\.[0-9]{1,9})s$`
		}
		fillSchemaByStringKeywords(schema, fo.GetString_())
		return schema
	case "google.protobuf.FieldMask":
		schema := &jsonschema.Schema{Type: "string"}
		fillSchemaByStringKeywords(schema, fo.GetString_())
		return schema
	case "google.protobuf.Any", "google.protobuf.Struct":
		return &jsonschema.Schema{Type: "object"}
	case "google.protobuf.ListValue":
		return &jsonschema.Schema{Type: "array"}
	case "google.protobuf.Value":
		// Any JSON value.
		return &jsonschema.Schema{}
	}
	// Wrappers are written as their value: see buildValue.
	return nil
}

func buildFromEnum(pluginOptions *proto.PluginOptions, enum protoreflect.EnumDescriptor) (*jsonschema.Schema, error) {
	eo := proto.GetEnumOptions(enum)

	schema := &jsonschema.Schema{}
	schema.Title = proto.GetTitleOrEmpty(eo)
	schema.Description = proto.GetDescriptionOrComment(enum, eo)
	schema.Deprecated = deprecated(enum)

	lenient := isLenient(pluginOptions) && eo.GetMappingType() != proto.EnumOptions_MapToCustom
	var enumValues []any
	var consts []*jsonschema.Schema
	for enumValue := range listOf(enum.Values()) {
		var value any
		switch {
		case lenient || eo.GetMappingType() == proto.EnumOptions_MapToString:
			// Parsers accept a value's name or any number.
			value = string(enumValue.Name())
		case eo.GetMappingType() == proto.EnumOptions_MapToNumber:
			value = int32(enumValue.Number())
		case eo.GetMappingType() == proto.EnumOptions_MapToCustom:
			evo := proto.GetEnumValueOptions(enumValue)

			customValue, err := parseScalaValueFromAny(evo.GetCustomValue())
			if err != nil {
				return nil, err
			}
			value = customValue
			if value == nil {
				value = string(enumValue.Name())
			}
		}

		// Aliases (allow_alias) share a number, and custom values may repeat;
		// draft-04/06/07 require unique enum items.
		if slices.ContainsFunc(enumValues, func(v any) bool { return reflect.DeepEqual(v, value) }) {
			continue
		}
		enumValues = append(enumValues, value)
		if pluginOptions.GetEnumValueDescriptions() {
			consts = append(consts, &jsonschema.Schema{
				Const:       &value,
				Description: proto.GetDescriptionOrComment(enumValue, nil),
				Deprecated:  deprecated(enumValue),
			})
		}
	}

	if lenient {
		names := &jsonschema.Schema{Type: "string", Enum: enumValues}
		if pluginOptions.GetEnumValueDescriptions() {
			schema.AnyOf = append(consts, lenientInteger(protoreflect.EnumKind))
		} else {
			schema.AnyOf = []*jsonschema.Schema{names, lenientInteger(protoreflect.EnumKind)}
		}
		return schema, nil
	}
	if pluginOptions.GetEnumValueDescriptions() {
		schema.AnyOf = consts
	} else {
		schema.Enum = enumValues
	}

	switch eo.GetMappingType() {
	case proto.EnumOptions_MapToString:
		schema.Type = "string"
	case proto.EnumOptions_MapToNumber:
		schema.Type = "integer"
	case proto.EnumOptions_MapToCustom:
		// Custom values may be of any JSON type; only narrow when all agree.
		if !slices.ContainsFunc(enumValues, func(v any) bool { _, ok := v.(string); return !ok }) {
			schema.Type = "string"
		}
	}
	return schema, nil
}

// deprecated returns nil, which writes nothing, unless desc is deprecated.
func deprecated(desc protoreflect.Descriptor) *bool {
	if opts, ok := desc.Options().(interface{ GetDeprecated() bool }); ok && opts.GetDeprecated() {
		return new(true)
	}
	return nil
}

func examples(values []*structpb.Value) []any {
	var out []any
	for _, v := range values {
		out = append(out, v.AsInterface())
	}
	return out
}

func isHiddenMessage(pluginOptions *proto.PluginOptions, message protoreflect.MessageDescriptor) bool {
	return proto.GetMessageOptions(message).GetVisibilityLevel() < pluginOptions.GetVisibilityLevel()
}

func isHiddenField(pluginOptions *proto.PluginOptions, field protoreflect.FieldDescriptor) bool {
	return proto.GetFieldOptions(field).GetVisibilityLevel() < pluginOptions.GetVisibilityLevel()
}

func wrapSchemaInArray(schema *jsonschema.Schema, field protoreflect.FieldDescriptor, fo *proto.FieldOptions) *jsonschema.Schema {
	repeatedSchema := &jsonschema.Schema{}
	repeatedSchema.Title = schema.Title
	repeatedSchema.Description = schema.Description
	repeatedSchema.Type = "array"
	repeatedSchema.Items = schema

	fillSchemaByArrayKeywords(repeatedSchema, fo.GetArray())
	return repeatedSchema
}

func parseScalaValueFromAny(anyValue *anypb.Any) (any, error) {
	if anyValue == nil || anyValue.Value == nil {
		return nil, nil
	}

	var value any
	if err := json.Unmarshal(anyValue.Value, &value); err != nil {
		return nil, err
	}
	return value, nil
}

func toPropertyName(field protoreflect.FieldDescriptor, preserveProto bool) string {
	if !preserveProto && field.HasJSONName() {
		return field.JSONName()
	}
	return string(field.Name())
}

// fullyQualifiedName returns the name of desc with a leading dot, as in
// ".pkg.Message.field"; it is the key of the schema of desc.
func fullyQualifiedName(desc protoreflect.Descriptor) string {
	return "." + string(desc.FullName())
}

func toRefId(desc protoreflect.Descriptor) jsonschema.RefId {
	return jsonschema.RefId(fullyQualifiedName(desc))
}

// isMessageKind reports whether kind is a message. Groups count: in
// editions a message field with DELIMITED encoding is a group, and a proto2
// group is a message in JSON too.
func isMessageKind(kind protoreflect.Kind) bool {
	return kind == protoreflect.MessageKind || kind == protoreflect.GroupKind
}

// isOptionalField reports whether field is singular, outside a oneof and
// may be absent: an optional field in proto2 or proto3, or a field with
// explicit presence in an editions file. Message fields follow the
// field_presence feature too, so an editions file migrated from proto2 or
// proto3 keeps its schema.
func isOptionalField(field protoreflect.FieldDescriptor) bool {
	if field.ParentFile().Syntax() != protoreflect.Editions {
		return field.HasOptionalKeyword()
	}
	return field.Cardinality() == protoreflect.Optional && field.ContainingOneof() == nil &&
		fieldPresence(field) == descriptorpb.FeatureSet_EXPLICIT
}

// fieldPresence resolves the field_presence feature of a field in an
// editions file: the closest of the field, its messages and its file that
// sets it, or EXPLICIT, the default of every edition.
func fieldPresence(field protoreflect.FieldDescriptor) descriptorpb.FeatureSet_FieldPresence {
	for d := protoreflect.Descriptor(field); d != nil; d = d.Parent() {
		if p := featuresOf(d).GetFieldPresence(); p != descriptorpb.FeatureSet_FIELD_PRESENCE_UNKNOWN {
			return p
		}
	}
	return descriptorpb.FeatureSet_EXPLICIT
}

func featuresOf(d protoreflect.Descriptor) *descriptorpb.FeatureSet {
	switch opts := d.Options().(type) {
	case *descriptorpb.FieldOptions:
		return opts.GetFeatures()
	case *descriptorpb.MessageOptions:
		return opts.GetFeatures()
	case *descriptorpb.FileOptions:
		return opts.GetFeatures()
	}
	return nil
}

// inRealOneof reports whether field is in a oneof written in the .proto
// file, not the synthetic oneof of a proto3 optional field.
func inRealOneof(field protoreflect.FieldDescriptor) bool {
	oneof := field.ContainingOneof()
	return oneof != nil && !oneof.IsSynthetic()
}

func isIntKind(kind protoreflect.Kind) bool {
	switch kind {
	case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sfixed32Kind,
		protoreflect.Uint32Kind, protoreflect.Fixed32Kind,
		protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind,
		protoreflect.Uint64Kind, protoreflect.Fixed64Kind:
		return true
	}
	return false
}

func isUnsignedKind(kind protoreflect.Kind) bool {
	switch kind {
	case protoreflect.Uint32Kind, protoreflect.Fixed32Kind, protoreflect.Uint64Kind, protoreflect.Fixed64Kind:
		return true
	}
	return false
}

func isNumericKind(kind protoreflect.Kind) bool {
	return isIntKind(kind) || kind == protoreflect.DoubleKind || kind == protoreflect.FloatKind
}

func isInt64(kind protoreflect.Kind) bool {
	switch kind {
	case protoreflect.Int64Kind, protoreflect.Uint64Kind, protoreflect.Sfixed64Kind, protoreflect.Sint64Kind, protoreflect.Fixed64Kind:
		return true
	}
	return false
}

func listOf[T any](list interface {
	Len() int
	Get(int) T
}) iter.Seq[T] {
	return func(yield func(T) bool) {
		for i := range list.Len() {
			if !yield(list.Get(i)) {
				return
			}
		}
	}
}

func deletePropertyInRequired(required []string, item string) []string {
	return slices.DeleteFunc(required, func(val string) bool {
		return val == item
	})
}
