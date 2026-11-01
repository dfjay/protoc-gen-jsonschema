package modules

import (
	"fmt"
	"slices"
	"strings"

	"github.com/dfjay/protoc-gen-jsonschema/pkg/jsonschema"
	"google.golang.org/protobuf/reflect/protoreflect"
)

type BackendOptimizer interface {
	Optimize(registry *jsonschema.Registry, entrypointMessage protoreflect.MessageDescriptor) (*jsonschema.Registry, error)
}

var _ BackendOptimizer = (*OptimizerImpl)(nil)

type OptimizerImpl struct{}

func NewOptimizerImpl() *OptimizerImpl {
	return &OptimizerImpl{}
}

// Optimize returns copies of the schemas reachable from the entrypoint and
// leaves registry unchanged. It fails when a reachable $ref has no schema,
// e.g. a message hidden by visibility_level but used by a visible field.
func (o *OptimizerImpl) Optimize(registry *jsonschema.Registry, entrypointMessage protoreflect.MessageDescriptor) (*jsonschema.Registry, error) {
	r := reachability{registry: registry, reached: map[string]bool{}}
	r.visitRef(toRefId(entrypointMessage).String())
	if len(r.missing) > 0 {
		return nil, fmt.Errorf("schemas not found (hidden by visibility_level?): %s", strings.Join(r.missing, ", "))
	}

	optimized := jsonschema.NewRegistry()
	for _, key := range registry.GetKeys() {
		if r.reached[key] {
			optimized.AddSchema(key, jsonschema.DeepCopy(registry.GetSchema(key)))
		}
	}
	return optimized, nil
}

type reachability struct {
	registry *jsonschema.Registry
	reached  map[string]bool
	missing  []string
}

func (r *reachability) visitRef(ref string) {
	if r.reached[ref] {
		return
	}
	schema := r.registry.GetSchema(ref)
	if schema == nil {
		if !slices.Contains(r.missing, ref) {
			r.missing = append(r.missing, ref)
		}
		return
	}
	r.reached[ref] = true
	r.visitSchema(schema)
}

func (r *reachability) visitSchema(schema *jsonschema.Schema) {
	if schema == nil {
		return
	}

	if !schema.Ref.IsEmpty() {
		r.visitRef(schema.Ref.String())
	}
	r.visitSchemaMap(schema.Definitions)

	r.visitSchemaArray(schema.AllOf)
	r.visitSchemaArray(schema.AnyOf)
	r.visitSchemaArray(schema.OneOf)
	r.visitSchema(schema.Not)

	r.visitSchema(schema.If)
	r.visitSchema(schema.Then)
	r.visitSchema(schema.Else)
	r.visitSchemaMap(schema.DependentSchemas)

	r.visitSchemaArray(schema.PrefixItems)
	r.visitSchema(schema.Items)
	r.visitSchema(schema.Contains)

	r.visitSchemaMap(schema.Properties)
	r.visitSchemaMap(schema.PatternProperties)
	r.visitSchema(schema.AdditionalProperties)
	r.visitSchema(schema.PropertyNames)

	r.visitSchema(schema.ContentSchema)
}

func (r *reachability) visitSchemaArray(schemas []*jsonschema.Schema) {
	for _, schema := range schemas {
		r.visitSchema(schema)
	}
}

func (r *reachability) visitSchemaMap(schemaMap *jsonschema.SchemaMap) {
	if schemaMap == nil {
		return
	}
	for _, key := range schemaMap.Keys() {
		schema, _ := schemaMap.Get(key)
		r.visitSchema(schema)
	}
}
