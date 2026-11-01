package draft_07

import (
	"maps"
	"slices"

	"github.com/dfjay/protoc-gen-jsonschema/pkg/jsonschema"
	"github.com/dfjay/protoc-gen-jsonschema/pkg/utils"
	"github.com/iancoleman/orderedmap"
)

type Schema struct {
	Version     string                 `json:"$schema,omitempty"`
	ID          string                 `json:"$id,omitempty"`
	Ref         string                 `json:"$ref,omitempty"`
	Definitions *orderedmap.OrderedMap `json:"definitions,omitempty"`
	Comments    string                 `json:"$comment,omitempty"`

	AllOf []*Schema `json:"allOf,omitempty"`
	AnyOf []*Schema `json:"anyOf,omitempty"`
	OneOf []*Schema `json:"oneOf,omitempty"`
	Not   *Schema   `json:"not,omitempty"`

	If   *Schema `json:"if,omitempty"`
	Then *Schema `json:"then,omitempty"`
	Else *Schema `json:"else,omitempty"`

	// Items is a *Schema for every element, or a []*Schema for a tuple.
	Items           any     `json:"items,omitempty"`
	AdditionalItems *Schema `json:"additionalItems,omitempty"`
	Contains        *Schema `json:"contains,omitempty"`

	Properties           *orderedmap.OrderedMap `json:"properties,omitempty"`
	PatternProperties    *orderedmap.OrderedMap `json:"patternProperties,omitempty"`
	AdditionalProperties *Schema                `json:"additionalProperties,omitempty"`
	PropertyNames        *Schema                `json:"propertyNames,omitempty"`

	Type             string   `json:"type,omitempty"`
	Enum             []any    `json:"enum,omitempty"`
	Const            *any     `json:"const,omitempty"`
	MultipleOf       *int     `json:"multipleOf,omitempty"`
	Maximum          *float64 `json:"maximum,omitempty"`
	ExclusiveMaximum *float64 `json:"exclusiveMaximum,omitempty"`
	Minimum          *float64 `json:"minimum,omitempty"`
	ExclusiveMinimum *float64 `json:"exclusiveMinimum,omitempty"`
	MaxLength        *int     `json:"maxLength,omitempty"`
	MinLength        *int     `json:"minLength,omitempty"`
	Pattern          string   `json:"pattern,omitempty"`
	MaxItems         *int     `json:"maxItems,omitempty"`
	MinItems         *int     `json:"minItems,omitempty"`
	UniqueItems      *bool    `json:"uniqueItems,omitempty"`
	MaxProperties    *int     `json:"maxProperties,omitempty"`
	MinProperties    *int     `json:"minProperties,omitempty"`
	Required         []string `json:"required,omitempty"`

	Format string `json:"format,omitempty"`

	ContentEncoding  string `json:"contentEncoding,omitempty"`
	ContentMediaType string `json:"contentMediaType,omitempty"`

	Title       string `json:"title,omitempty"`
	Description string `json:"description,omitempty"`
	Default     *any   `json:"default,omitempty"`
	ReadOnly    *bool  `json:"readOnly,omitempty"`
	WriteOnly   *bool  `json:"writeOnly,omitempty"`
	Examples    []any  `json:"examples,omitempty"`

	Extras map[string]any `json:"-"`
}

func New(schema *jsonschema.Schema) *Schema {
	return deepCopy(schema)
}

func deepCopy(origin *jsonschema.Schema) *Schema {
	if origin == nil {
		return nil
	}

	dst := &Schema{}
	dst.Version = origin.Version
	dst.ID = origin.ID
	if origin.Ref.String() != "" {
		dst.Ref = "#/definitions/" + origin.Ref.String()
	}
	dst.Definitions = deepCopyMap(origin.Definitions)
	dst.Comments = origin.Comments

	dst.AllOf = deepCopyArray(origin.AllOf)
	dst.AnyOf = deepCopyArray(origin.AnyOf)
	dst.OneOf = deepCopyArray(origin.OneOf)
	dst.Not = deepCopy(origin.Not)

	dst.If = deepCopy(origin.If)
	dst.Then = deepCopy(origin.Then)
	dst.Else = deepCopy(origin.Else)

	if len(origin.PrefixItems) > 0 {
		dst.Items = deepCopyArray(origin.PrefixItems)
		dst.AdditionalItems = deepCopy(origin.Items)
	} else if origin.Items != nil {
		dst.Items = deepCopy(origin.Items)
	}
	dst.Contains = deepCopy(origin.Contains)

	dst.Properties = deepCopyMap(origin.Properties)
	dst.PatternProperties = deepCopyMap(origin.PatternProperties)
	dst.AdditionalProperties = deepCopy(origin.AdditionalProperties)
	dst.PropertyNames = deepCopy(origin.PropertyNames)

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
	dst.MaxProperties = utils.ClonePtr(origin.MaxProperties)
	dst.MinProperties = utils.ClonePtr(origin.MinProperties)
	dst.Required = slices.Clone(origin.Required)

	dst.Format = origin.Format

	dst.ContentEncoding = origin.ContentEncoding
	dst.ContentMediaType = origin.ContentMediaType

	dst.Title = origin.Title
	dst.Description = origin.Description
	dst.Default = utils.ClonePtr(origin.Default)
	dst.ReadOnly = utils.ClonePtr(origin.ReadOnly)
	dst.WriteOnly = utils.ClonePtr(origin.WriteOnly)

	dst.Extras = maps.Clone(origin.Extras)

	// Do nothing if IsBooleanSchema is true. Cause empty schema means true
	// If IsBooleanSchema is false, { not: {} } is generated
	if origin.IsBooleanSchema != nil && !*origin.IsBooleanSchema {
		dst.Not = &Schema{}
	}
	return dst
}

func deepCopyArray(arr []*jsonschema.Schema) []*Schema {
	dst := make([]*Schema, len(arr))
	for i, schema := range arr {
		dst[i] = deepCopy(schema)
	}
	return dst
}

func deepCopyMap(schemaMap jsonschema.SchemaMap) *orderedmap.OrderedMap {
	if schemaMap == nil {
		return nil
	}
	dst := orderedmap.New()
	for _, key := range schemaMap.Keys() {
		schema, _ := schemaMap.Get(key)
		dst.Set(key, deepCopy(schema))
	}
	return dst
}
