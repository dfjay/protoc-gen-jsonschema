package modules

import (
	"fmt"
	"strings"

	"github.com/dfjay/protoc-gen-jsonschema/pkg/jsonschema"
	"github.com/dfjay/protoc-gen-jsonschema/pkg/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
)

// FrontendVisitor generates the intermediate schemas of every message,
// field and enum in the request.
type FrontendVisitor struct {
	registry      *jsonschema.Registry
	pluginOptions *proto.PluginOptions
	ext           extensions
	// unvalidated is set when a schema refers to a message without rules.
	unvalidated bool
}

// extensions reads options defined in other packages' .proto files.
type extensions struct {
	behaviors fieldBehaviors
	validate  bufValidate
}

// NewVisitor returns a visitor of the files in files.
func NewVisitor(pluginOptions *proto.PluginOptions, files *protoregistry.Files) *FrontendVisitor {
	v := &FrontendVisitor{
		registry:      jsonschema.NewRegistry(),
		pluginOptions: pluginOptions,
	}
	if pluginOptions.GetRespectFieldBehavior() {
		v.ext.behaviors = newFieldBehaviors(files)
	}
	if pluginOptions.GetRespectBufValidate() {
		v.ext.validate = newBufValidate(files)
	}
	return v
}

func (v *FrontendVisitor) VisitFile(file protoreflect.FileDescriptor) error {
	if err := v.visitEnums(file.Enums()); err != nil {
		return err
	}
	return v.visitMessages(file.Messages())
}

func (v *FrontendVisitor) visitMessages(messages protoreflect.MessageDescriptors) error {
	for i := range messages.Len() {
		if err := v.visitMessage(messages.Get(i)); err != nil {
			return err
		}
	}
	return nil
}

func (v *FrontendVisitor) visitEnums(enums protoreflect.EnumDescriptors) error {
	for i := range enums.Len() {
		if err := v.visitEnum(enums.Get(i)); err != nil {
			return err
		}
	}
	return nil
}

// visitMessage registers message and, unless it is hidden, its nested
// enums, messages and fields. Map entries are skipped: map fields describe
// their keys and values themselves.
func (v *FrontendVisitor) visitMessage(message protoreflect.MessageDescriptor) error {
	if message.IsMapEntry() || isHiddenMessage(v.pluginOptions, message) {
		return nil
	}
	mo := proto.GetMessageOptions(message)

	var schema *jsonschema.Schema
	if isWellKnownMessage(message) {
		schema = buildFromWellKnownMessage(v.pluginOptions, v.ext, message, mo)
	} else {
		schema = buildFromMessage(v.pluginOptions, v.ext, message, mo)
	}
	v.registry.AddSchema(fullyQualifiedName(message), schema)

	if err := v.visitEnums(message.Enums()); err != nil {
		return err
	}
	if err := v.visitMessages(message.Messages()); err != nil {
		return err
	}
	fields := message.Fields()
	for i := range fields.Len() {
		if err := v.visitField(fields.Get(i)); err != nil {
			return err
		}
	}
	return nil
}

func (v *FrontendVisitor) visitField(field protoreflect.FieldDescriptor) error {
	if isHiddenField(v.pluginOptions, field) {
		return nil
	}
	fo := proto.GetFieldOptions(field)
	if numeric := fo.GetNumeric(); numeric != nil && numeric.MultipleOf != nil && numeric.GetMultipleOf() <= 0 {
		return fmt.Errorf("%s: multiple_of must be greater than 0, got %d", fullyQualifiedName(field), numeric.GetMultipleOf())
	}

	rules := v.ext.validate.fieldRules(field)
	var schema *jsonschema.Schema
	if field.IsMap() {
		schema = buildFromMapField(v.pluginOptions, field, fo, rules)
	} else {
		schema = buildFromField(v.pluginOptions, field, fo, rules)
	}
	if (fo == nil || fo.Required == nil) && !field.HasPresence() && ruleBool(rules, "required") {
		// protovalidate's required also rejects the zero value.
		schema.AllOf = append(schema.AllOf, &jsonschema.Schema{Not: unsetValue(v.pluginOptions, field)})
	}
	ignored := v.ext.validate.ignoresAlways(field)
	jsonschema.Walk(schema, func(s *jsonschema.Schema) {
		if ignored != nil && s.Ref == toRefId(ignored) {
			s.Ref = jsonschema.RefId(unvalidatedPrefix) + s.Ref
		}
		// Also set by an empty message element under IGNORE_IF_ZERO_VALUE.
		if strings.HasPrefix(string(s.Ref), unvalidatedPrefix) {
			v.unvalidated = true
		}
	})
	readOnly, writeOnly := v.ext.behaviors.has(field, "OUTPUT_ONLY"), v.ext.behaviors.has(field, "INPUT_ONLY")
	if (readOnly || writeOnly) && schema.Ref != "" {
		// Draft-07 ignores the keywords beside $ref.
		schema.AllOf = append(schema.AllOf, &jsonschema.Schema{Ref: schema.Ref})
		schema.Ref = ""
	}
	if readOnly {
		schema.ReadOnly = new(true)
	}
	if writeOnly {
		schema.WriteOnly = new(true)
	}
	v.registry.AddSchema(fullyQualifiedName(field), schema)
	return nil
}

func (v *FrontendVisitor) visitEnum(enum protoreflect.EnumDescriptor) error {
	schema, err := buildFromEnum(v.pluginOptions, enum)
	if err != nil {
		return err
	}
	v.registry.AddSchema(fullyQualifiedName(enum), schema)
	return nil
}
