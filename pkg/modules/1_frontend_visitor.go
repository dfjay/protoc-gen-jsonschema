package modules

import (
	"fmt"

	"github.com/dfjay/protoc-gen-jsonschema/pkg/jsonschema"
	"github.com/dfjay/protoc-gen-jsonschema/pkg/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// FrontendVisitor generates the intermediate schemas of every message,
// field and enum in the request.
type FrontendVisitor struct {
	registry      *jsonschema.Registry
	pluginOptions *proto.PluginOptions
}

func NewVisitor(pluginOptions *proto.PluginOptions) *FrontendVisitor {
	return &FrontendVisitor{
		registry:      jsonschema.NewRegistry(),
		pluginOptions: pluginOptions,
	}
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
		schema = buildFromWellKnownMessage(v.pluginOptions, message, mo)
	} else {
		schema = buildFromMessage(v.pluginOptions, message, mo)
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

	var schema *jsonschema.Schema
	switch {
	case isWellKnownField(field):
		schema = buildFromWellKnownField(field, fo)
	case field.IsMap():
		schema = buildFromMapField(v.pluginOptions, field, fo)
	case isMessageKind(field.Kind()):
		schema = buildFromMessageField(field, fo)
	case isScalarType(field):
		schema = buildFromScalaField(v.pluginOptions, field, fo)
	default:
		return fmt.Errorf("%s: unsupported field type %s", fullyQualifiedName(field), field.Kind())
	}
	v.registry.AddSchema(fullyQualifiedName(field), schema)
	return nil
}

func (v *FrontendVisitor) visitEnum(enum protoreflect.EnumDescriptor) error {
	schema, err := buildFromEnum(enum)
	if err != nil {
		return err
	}
	v.registry.AddSchema(fullyQualifiedName(enum), schema)
	return nil
}
