package proto

import (
	"cmp"
	"errors"
	"fmt"
	"strings"

	pgs "github.com/lyft/protoc-gen-star/v2"
)

// GetPluginOptions parses the --jsonschema_opt parameters. Omitted
// parameters take their defaults; malformed values are errors.
func GetPluginOptions(params pgs.Parameters) (*PluginOptions, error) {
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
	options.RespectProtojsonPresence = parseBool("respect_protojson_presence", false)
	if _, ok := params["int64_as_string"]; ok {
		options.RespectProtojsonInt64 = parseBool("int64_as_string", false)
	} else {
		options.RespectProtojsonInt64 = parseBool("respect_protojson_int64", false)
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

func GetFileOptions(file pgs.File) *FileOptions {
	options := &FileOptions{}
	if ok, _ := file.Extension(E_File, options); ok {
		return options
	}
	return nil
}

func GetMessageOptions(message pgs.Message) *MessageOptions {
	options := &MessageOptions{}
	if ok, _ := message.Extension(E_Message, options); ok {
		return options
	}
	return nil
}

func GetFieldOptions(field pgs.Field) *FieldOptions {
	options := &FieldOptions{}
	if ok, _ := field.Extension(E_Field, options); ok {
		return options
	}
	return nil
}

func GetEnumOptions(enum pgs.Enum) *EnumOptions {
	options := &EnumOptions{}
	if ok, _ := enum.Extension(E_Enum, options); ok {
		return options
	}
	return nil
}

func GetEnumValueOptions(enumValue pgs.EnumValue) *EnumValueOptions {
	options := &EnumValueOptions{}
	if ok, _ := enumValue.Extension(E_EnumValue, options); ok {
		return options
	}
	return nil
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

type NameWithSourceResolver interface {
	SourceCodeInfo() pgs.SourceCodeInfo
	Name() pgs.Name
}

func GetDescriptionOrEmpty(description DescriptionResolver) string {
	if description != nil && description.GetDescription() != "" {
		return description.GetDescription()
	}
	return ""
}

func GetDescriptionOrComment(name NameWithSourceResolver, description DescriptionResolver) string {
	if description != nil && description.GetDescription() != "" {
		return description.GetDescription()
	}

	srcInfo := name.SourceCodeInfo()
	if srcInfo == nil {
		return ""
	}

	builder := &strings.Builder{}
	for _, comment := range srcInfo.LeadingDetachedComments() {
		builder.WriteString(comment)
	}
	builder.WriteString(srcInfo.LeadingComments())
	builder.WriteString(srcInfo.TrailingComments())
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
