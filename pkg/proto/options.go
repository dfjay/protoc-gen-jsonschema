package proto

import (
	"cmp"
	"errors"
	"fmt"
	"strings"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// GetPluginOptions parses the --jsonschema_opt parameters. Omitted
// parameters take their defaults; malformed values are errors.
func GetPluginOptions(params Parameters) (*PluginOptions, error) {
	options := &PluginOptions{}
	var errs []error
	check := func(err error) {
		if err != nil {
			errs = append(errs, err)
		}
	}
	parseBool := func(name string, def bool) bool {
		v, err := params.BoolDefault(name, def)
		check(err)
		return v
	}

	visibilityLevel, err := params.UintDefault("visibility_level", 0)
	check(err)
	options.VisibilityLevel = uint32(visibilityLevel)

	options.EntrypointMessage = params.StrDefault("entrypoint_message", "")
	options.OutputFileSuffix = params.StrDefault("output_file_suffix", ".schema.json")
	options.PrettyJsonOutput = parseBool("pretty_json_output", true)
	options.MandatoryNullable = parseBool("mandatory_nullable", false)
	options.PreserveProtoFieldNames = parseBool("preserve_proto_field_names", false)
	options.RespectProtojsonPresence = parseBool("respect_protojson_presence", true)
	options.RespectFieldBehavior = parseBool("respect_field_behavior", true)
	if _, ok := params["int64_as_string"]; ok {
		options.RespectProtojsonInt64 = parseBool("int64_as_string", true)
	} else {
		options.RespectProtojsonInt64 = parseBool("respect_protojson_int64", true)
	}

	options.AdditionalProperties = PluginAdditionalProperties_DoNothing
	if name := params.Str("additional_properties"); name != "" {
		v, ok := PluginAdditionalProperties_value[name]
		switch {
		case !ok:
			check(fmt.Errorf("unknown additional_properties %q", name))
		case PluginAdditionalProperties(v) != PluginAdditionalProperties_DefaultAdditionalProperties:
			options.AdditionalProperties = PluginAdditionalProperties(v)
		}
	}

	options.Draft = Draft_Draft202012
	if name := params.Str("draft"); name != "" {
		v, ok := Draft_value[name]
		switch {
		case !ok:
			check(fmt.Errorf("unknown draft %q", name))
		case Draft(v) != Draft_DraftDefault:
			options.Draft = Draft(v)
		}
	}

	return options, errors.Join(errs...)
}

func extension[T proto.Message](opts proto.Message, xt protoreflect.ExtensionType) T {
	var zero T
	if opts == nil || !proto.HasExtension(opts, xt) {
		return zero
	}
	return proto.GetExtension(opts, xt).(T)
}

func GetFileOptions(file protoreflect.FileDescriptor) *FileOptions {
	return extension[*FileOptions](file.Options(), E_File)
}

func GetMessageOptions(message protoreflect.MessageDescriptor) *MessageOptions {
	return extension[*MessageOptions](message.Options(), E_Message)
}

func GetFieldOptions(field protoreflect.FieldDescriptor) *FieldOptions {
	return extension[*FieldOptions](field.Options(), E_Field)
}

func GetEnumOptions(enum protoreflect.EnumDescriptor) *EnumOptions {
	return extension[*EnumOptions](enum.Options(), E_Enum)
}

func GetEnumValueOptions(enumValue protoreflect.EnumValueDescriptor) *EnumValueOptions {
	return extension[*EnumValueOptions](enumValue.Options(), E_EnumValue)
}

type TitleResolver interface {
	GetTitle() string
}

func GetTitleOrEmpty(title TitleResolver) string {
	if title != nil && title.GetTitle() != "" {
		return title.GetTitle()
	}
	return ""
}

type DescriptionResolver interface {
	GetDescription() string
}

func GetDescriptionOrEmpty(description DescriptionResolver) string {
	if description != nil && description.GetDescription() != "" {
		return description.GetDescription()
	}
	return ""
}

// GetDescriptionOrComment returns the description option, or else the
// comments around desc in its .proto file.
func GetDescriptionOrComment(desc protoreflect.Descriptor, description DescriptionResolver) string {
	if description != nil && description.GetDescription() != "" {
		return description.GetDescription()
	}

	loc := desc.ParentFile().SourceLocations().ByDescriptor(desc)
	builder := &strings.Builder{}
	for _, comment := range loc.LeadingDetachedComments {
		builder.WriteString(comment)
	}
	builder.WriteString(loc.LeadingComments)
	builder.WriteString(loc.TrailingComments)
	return strings.TrimSpace(strings.Trim(builder.String(), "\n"))
}

func GetEntrypointMessage(pluginOptions *PluginOptions, fileOptions *FileOptions) string {
	return cmp.Or(fileOptions.GetEntrypointMessage(), pluginOptions.GetEntrypointMessage())
}

func GetAdditionalProperties(pluginOptions *PluginOptions, keywords *ObjectKeywords) *bool {
	switch pluginOptions.AdditionalProperties {
	case PluginAdditionalProperties_AlwaysTrue:
		return new(true)
	case PluginAdditionalProperties_AlwaysFalse:
		return new(false)
	case PluginAdditionalProperties_DefaultTrue:
		if keywords == nil || keywords.AdditionalProperties == nil {
			return new(true)
		} else {
			return keywords.AdditionalProperties
		}
	case PluginAdditionalProperties_DefaultFalse:
		if keywords == nil || keywords.AdditionalProperties == nil {
			return new(false)
		} else {
			return keywords.AdditionalProperties
		}
	case PluginAdditionalProperties_DoNothing:
		if keywords == nil {
			return nil
		} else {
			return keywords.AdditionalProperties
		}
	default:
		// Unreachable code
		panic(fmt.Sprintf("not supported additional properties option. parsed option index: '%d'", pluginOptions.AdditionalProperties))
	}
}
