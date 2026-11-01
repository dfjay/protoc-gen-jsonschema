package modules

import (
	"fmt"
	"slices"
	"strings"

	"github.com/dfjay/protoc-gen-jsonschema/pkg/jsonschema"
	"google.golang.org/protobuf/reflect/protoreflect"
)

type BackendOptimizer interface {
	Optimize(registry *jsonschema.Registry, entrypointMessage protoreflect.MessageDescriptor) error
}

var _ BackendOptimizer = (*OptimizerImpl)(nil)

type OptimizerImpl struct{}

func NewOptimizerImpl() *OptimizerImpl {
	return &OptimizerImpl{}
}

const linkedFromEntrypoint = "linkedFromEntrypoint"

// Optimize keeps only the schemas reachable from the entrypoint. It fails
// when a reachable $ref has no schema, e.g. a message hidden by
// visibility_level but used by a visible field.
func (o *OptimizerImpl) Optimize(registry *jsonschema.Registry, entrypointMessage protoreflect.MessageDescriptor) error {
	var missing []string
	o.visitRef(registry, toRefId(entrypointMessage).String(), &missing)
	if len(missing) > 0 {
		return fmt.Errorf("schemas not found (hidden by visibility_level?): %s", strings.Join(missing, ", "))
	}

	o.optimizeDefinitions(registry)
	return nil
}

func (o *OptimizerImpl) optimizeDefinitions(registry *jsonschema.Registry) {
	var deleteKeys []string
	for _, key := range registry.GetKeys() {
		schema := registry.GetSchema(key)
		if linked, _ := schema.GetExtrasItem(linkedFromEntrypoint).(bool); !linked {
			deleteKeys = append(deleteKeys, key)
		}
	}

	for _, key := range deleteKeys {
		registry.DeleteSchema(key)
	}
}

// visitRef marks the schema behind ref as linked and visits it on its first
// visit; a ref without a schema is recorded in missing.
func (o *OptimizerImpl) visitRef(registry *jsonschema.Registry, ref string, missing *[]string) {
	schema := registry.GetSchema(ref)
	if schema == nil {
		if !slices.Contains(*missing, ref) {
			*missing = append(*missing, ref)
		}
		return
	}
	if schema.GetExtrasItem(linkedFromEntrypoint) != nil {
		return
	}
	schema.SetExtrasItem(linkedFromEntrypoint, true)
	o.visitSchema(registry, schema, missing)
}

func (o *OptimizerImpl) visitSchema(registry *jsonschema.Registry, schema *jsonschema.Schema, missing *[]string) {
	if schema == nil {
		return
	}

	if !schema.Ref.IsEmpty() {
		o.visitRef(registry, schema.Ref.String(), missing)
	}
	o.visitSchemaMap(registry, schema.Definitions, missing)

	o.visitSchemaArray(registry, schema.AllOf, missing)
	o.visitSchemaArray(registry, schema.AnyOf, missing)
	o.visitSchemaArray(registry, schema.OneOf, missing)
	o.visitSchema(registry, schema.Not, missing)

	o.visitSchema(registry, schema.If, missing)
	o.visitSchema(registry, schema.Then, missing)
	o.visitSchema(registry, schema.Else, missing)
	o.visitSchemaMap(registry, schema.DependentSchemas, missing)

	o.visitSchemaArray(registry, schema.PrefixItems, missing)
	o.visitSchema(registry, schema.Items, missing)
	o.visitSchema(registry, schema.Contains, missing)

	o.visitSchemaMap(registry, schema.Properties, missing)
	o.visitSchemaMap(registry, schema.PatternProperties, missing)
	o.visitSchema(registry, schema.AdditionalProperties, missing)
	o.visitSchema(registry, schema.PropertyNames, missing)

	o.visitSchema(registry, schema.ContentSchema, missing)
}

func (o *OptimizerImpl) visitSchemaArray(registry *jsonschema.Registry, schemas []*jsonschema.Schema, missing *[]string) {
	for _, schema := range schemas {
		o.visitSchema(registry, schema, missing)
	}
}

func (o *OptimizerImpl) visitSchemaMap(registry *jsonschema.Registry, schemaMap jsonschema.SchemaMap, missing *[]string) {
	if schemaMap == nil {
		return
	}
	for _, key := range schemaMap.Keys() {
		schema, _ := schemaMap.Get(key)
		o.visitSchema(registry, schema, missing)
	}
}
