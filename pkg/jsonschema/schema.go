package jsonschema

import (
	"maps"
	"slices"

	"github.com/dfjay/protoc-gen-jsonschema/pkg/utils"
)

type Schema struct {
	Version       string
	ID            string
	Anchor        string
	DynamicAnchor string
	Ref           RefId
	DynamicRef    string
	Definitions   *SchemaMap
	Comments      string

	AllOf []*Schema
	AnyOf []*Schema
	OneOf []*Schema
	Not   *Schema

	If               *Schema
	Then             *Schema
	Else             *Schema
	DependentSchemas *SchemaMap

	PrefixItems []*Schema
	Items       *Schema
	Contains    *Schema

	Properties           *SchemaMap
	PatternProperties    *SchemaMap
	AdditionalProperties *Schema
	PropertyNames        *Schema

	Type              string
	Enum              []any
	Const             *any
	MultipleOf        *int
	Maximum           *float64
	ExclusiveMaximum  *float64
	Minimum           *float64
	ExclusiveMinimum  *float64
	MaxLength         *int
	MinLength         *int
	Pattern           string
	MaxItems          *int
	MinItems          *int
	UniqueItems       *bool
	MaxContains       *int
	MinContains       *int
	MaxProperties     *int
	MinProperties     *int
	Required          []string
	DependentRequired map[string][]string

	Format string

	ContentEncoding  string
	ContentMediaType string
	ContentSchema    *Schema

	Title       string
	Description string
	Default     *any
	Deprecated  *bool
	ReadOnly    *bool
	WriteOnly   *bool
	Examples    []any

	Extras map[string]any

	// Indicated boolean schema or generic schema
	IsBooleanSchema *bool
}

func DeepCopy(origin *Schema) *Schema {
	if origin == nil {
		return nil
	}

	dst := &Schema{}
	dst.Version = origin.Version
	dst.ID = origin.ID
	dst.Anchor = origin.Anchor
	dst.DynamicAnchor = origin.DynamicAnchor
	dst.Ref = origin.Ref
	dst.DynamicRef = origin.DynamicRef
	dst.Definitions = MapValues(origin.Definitions, DeepCopy)
	dst.Comments = origin.Comments

	dst.AllOf = DeepCopyArray(origin.AllOf)
	dst.AnyOf = DeepCopyArray(origin.AnyOf)
	dst.OneOf = DeepCopyArray(origin.OneOf)
	dst.Not = DeepCopy(origin.Not)

	dst.If = DeepCopy(origin.If)
	dst.Then = DeepCopy(origin.Then)
	dst.Else = DeepCopy(origin.Else)
	dst.DependentSchemas = MapValues(origin.DependentSchemas, DeepCopy)

	dst.PrefixItems = DeepCopyArray(origin.PrefixItems)
	dst.Items = DeepCopy(origin.Items)
	dst.Contains = DeepCopy(origin.Contains)

	dst.Properties = MapValues(origin.Properties, DeepCopy)
	dst.PatternProperties = MapValues(origin.PatternProperties, DeepCopy)
	dst.AdditionalProperties = DeepCopy(origin.AdditionalProperties)
	dst.PropertyNames = DeepCopy(origin.PropertyNames)

	dst.Type = origin.Type
	dst.Enum = slices.Clone(origin.Enum)
	dst.Const = utils.ClonePtr(origin.Const)
	dst.MultipleOf = utils.ClonePtr(origin.MultipleOf)
	dst.Maximum = utils.ClonePtr(origin.Maximum)
	dst.ExclusiveMaximum = utils.ClonePtr(origin.ExclusiveMaximum)
	dst.Minimum = utils.ClonePtr(origin.Minimum)
	dst.ExclusiveMinimum = utils.ClonePtr(origin.ExclusiveMinimum)
	dst.MaxLength = utils.ClonePtr(origin.MaxLength)
	dst.MinLength = utils.ClonePtr(origin.MinLength)
	dst.Pattern = origin.Pattern
	dst.MaxItems = utils.ClonePtr(origin.MaxItems)
	dst.MinItems = utils.ClonePtr(origin.MinItems)
	dst.UniqueItems = utils.ClonePtr(origin.UniqueItems)
	dst.MaxContains = utils.ClonePtr(origin.MaxContains)
	dst.MinContains = utils.ClonePtr(origin.MinContains)
	dst.MaxProperties = utils.ClonePtr(origin.MaxProperties)
	dst.MinProperties = utils.ClonePtr(origin.MinProperties)
	dst.Required = slices.Clone(origin.Required)
	dst.DependentRequired = utils.CloneMapSlices(origin.DependentRequired)

	dst.Format = origin.Format

	dst.ContentEncoding = origin.ContentEncoding
	dst.ContentMediaType = origin.ContentMediaType
	dst.ContentSchema = DeepCopy(origin.ContentSchema)

	dst.Title = origin.Title
	dst.Description = origin.Description
	dst.Default = utils.ClonePtr(origin.Default)
	dst.Deprecated = utils.ClonePtr(origin.Deprecated)
	dst.ReadOnly = utils.ClonePtr(origin.ReadOnly)
	dst.WriteOnly = utils.ClonePtr(origin.WriteOnly)

	dst.Extras = maps.Clone(origin.Extras)

	dst.IsBooleanSchema = origin.IsBooleanSchema
	return dst
}

func DeepCopyArray(schemas []*Schema) []*Schema {
	if schemas == nil {
		return nil
	}

	newSchemas := make([]*Schema, len(schemas))
	for i, schema := range schemas {
		newSchemas[i] = DeepCopy(schema)
	}
	return newSchemas
}

// Boolean Schema is special type of schema
var (
	// TrueSchema equals to `{}` or `true`
	TrueSchema = *NewBooleanSchema(true)
	// FalseSchema equals to `{"not": {}}` or `false`
	FalseSchema = *NewBooleanSchema(false)
)

func NewBooleanSchema(b bool) *Schema {
	return &Schema{IsBooleanSchema: &b}
}
