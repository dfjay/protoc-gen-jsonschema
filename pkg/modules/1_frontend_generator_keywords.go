package modules

import (
	"github.com/dfjay/protoc-gen-jsonschema/pkg/jsonschema"
	"github.com/dfjay/protoc-gen-jsonschema/pkg/proto"
)

func fillSchemaByObjectKeywords(pluginOptions *proto.PluginOptions, schema *jsonschema.Schema, keywords *proto.ObjectKeywords) {
	if additionalProperties := proto.GetAdditionalProperties(pluginOptions, keywords); additionalProperties != nil {
		schema.AdditionalProperties = jsonschema.NewBooleanSchema(*additionalProperties)
	}

	if keywords == nil {
		return
	}
	if keywords.MinProperties != nil {
		schema.MinProperties = new(int(keywords.GetMinProperties()))
	}
	if keywords.MaxProperties != nil {
		schema.MaxProperties = new(int(keywords.GetMaxProperties()))
	}
}

func fillSchemaByNumericKeywords(schema *jsonschema.Schema, keywords *proto.NumericKeywords) {
	if keywords == nil {
		return
	}

	if val, ok := keywords.Min.(*proto.NumericKeywords_InclusiveMinimum); ok {
		schema.Minimum = &val.InclusiveMinimum
	}
	if val, ok := keywords.Max.(*proto.NumericKeywords_InclusiveMaximum); ok {
		schema.Maximum = &val.InclusiveMaximum
	}
	if val, ok := keywords.Max.(*proto.NumericKeywords_ExclusiveMaximum); ok {
		schema.ExclusiveMaximum = &val.ExclusiveMaximum
	}
	if val, ok := keywords.Min.(*proto.NumericKeywords_ExclusiveMinimum); ok {
		schema.ExclusiveMinimum = &val.ExclusiveMinimum
	}

	if keywords.MultipleOf != nil {
		schema.MultipleOf = new(int(keywords.GetMultipleOf()))
	}
}

func fillSchemaByStringKeywords(schema *jsonschema.Schema, keywords *proto.StringKeywords) {
	if keywords == nil {
		return
	}

	if keywords.Pattern != "" {
		schema.Pattern = keywords.GetPattern()
	}
	if keywords.Format != "" {
		schema.Format = keywords.GetFormat()
	}

	if keywords.MaxLength != nil {
		schema.MaxLength = new(int(keywords.GetMaxLength()))
	}
	if keywords.MinLength != nil {
		schema.MinLength = new(int(keywords.GetMinLength()))
	}
}

func fillSchemaByArrayKeywords(schema *jsonschema.Schema, keywords *proto.ArrayKeywords) {
	if keywords == nil {
		return
	}

	if keywords.MaxItems != nil {
		schema.MaxItems = new(int(keywords.GetMaxItems()))
	}
	if keywords.MinItems != nil {
		schema.MinItems = new(int(keywords.GetMinItems()))
	}
	if keywords.UniqueItems != nil {
		schema.UniqueItems = keywords.UniqueItems
	}
}
