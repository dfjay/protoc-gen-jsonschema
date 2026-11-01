package jsonschema_test

import (
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/dfjay/protoc-gen-jsonschema/pkg/jsonschema/draft_04"
	"github.com/dfjay/protoc-gen-jsonschema/pkg/jsonschema/draft_06"
	"github.com/dfjay/protoc-gen-jsonschema/pkg/jsonschema/draft_07"
	"github.com/dfjay/protoc-gen-jsonschema/pkg/jsonschema/draft_201909"
	"github.com/dfjay/protoc-gen-jsonschema/pkg/jsonschema/draft_202012"
)

// Keywords each draft defines, from its meta-schemas (docs/schema-draft*.json
// and the 2019-09 and 2020-12 vocabularies). Validators ignore keywords their
// draft lacks, so a constraint written with one would silently not apply.
var (
	draft04Keywords = []string{
		"id", "$schema", "$ref", "definitions",
		"allOf", "anyOf", "oneOf", "not",
		"items", "additionalItems", "properties", "patternProperties", "additionalProperties", "dependencies",
		"type", "enum", "multipleOf", "maximum", "exclusiveMaximum", "minimum", "exclusiveMinimum",
		"maxLength", "minLength", "pattern", "maxItems", "minItems", "uniqueItems",
		"maxProperties", "minProperties", "required", "format",
		"title", "description", "default",
	}
	draft06Keywords = slices.Concat(without(draft04Keywords, "id"), []string{
		"$id", "const", "contains", "propertyNames", "examples",
	})
	draft07Keywords = slices.Concat(draft06Keywords, []string{
		"$comment", "if", "then", "else", "contentEncoding", "contentMediaType", "readOnly", "writeOnly",
	})
	draft201909Keywords = slices.Concat(without(draft07Keywords, "definitions", "dependencies"), []string{
		"$anchor", "$recursiveRef", "$recursiveAnchor", "$vocabulary", "$defs",
		"dependentSchemas", "dependentRequired", "unevaluatedItems", "unevaluatedProperties",
		"maxContains", "minContains", "contentSchema", "deprecated",
	})
	draft202012Keywords = slices.Concat(without(draft201909Keywords, "$recursiveRef", "$recursiveAnchor", "additionalItems"), []string{
		"$dynamicRef", "$dynamicAnchor", "prefixItems",
	})
)

func TestDraftKeywords(t *testing.T) {
	for _, tc := range []struct {
		name     string
		schema   any
		keywords []string
	}{
		{"draft-04", draft_04.Schema{}, draft04Keywords},
		{"draft-06", draft_06.Schema{}, draft06Keywords},
		{"draft-07", draft_07.Schema{}, draft07Keywords},
		{"draft-2019-09", draft_201909.Schema{}, draft201909Keywords},
		{"draft-2020-12", draft_202012.Schema{}, draft202012Keywords},
	} {
		t.Run(tc.name, func(t *testing.T) {
			typ := reflect.TypeOf(tc.schema)
			for field := range typ.Fields() {
				name, _, _ := strings.Cut(field.Tag.Get("json"), ",")
				if name == "-" {
					continue
				}
				if !slices.Contains(tc.keywords, name) {
					t.Errorf("field %s writes %q, which %s does not define", field.Name, name, tc.name)
				}
			}
		})
	}
}

func without(keywords []string, drop ...string) []string {
	return slices.DeleteFunc(slices.Clone(keywords), func(k string) bool { return slices.Contains(drop, k) })
}
