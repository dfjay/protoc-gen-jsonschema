package jsonschema

import (
	"bytes"
	"encoding/json"
	"slices"
)

// OrderedMap keeps insertion order, so JSON output lists properties in proto
// declaration order and stays deterministic.
type OrderedMap[V any] struct {
	keys   []string
	values map[string]V
}

type SchemaMap = OrderedMap[*Schema]

func NewOrderedMap[V any]() *OrderedMap[V] {
	return &OrderedMap[V]{values: map[string]V{}}
}

func (m *OrderedMap[V]) Get(key string) (V, bool) {
	if m == nil {
		var zero V
		return zero, false
	}
	v, found := m.values[key]
	return v, found
}

// Set keeps the position of an existing key.
func (m *OrderedMap[V]) Set(key string, value V) {
	if _, found := m.values[key]; !found {
		m.keys = append(m.keys, key)
	}
	m.values[key] = value
}

func (m *OrderedMap[V]) Delete(key string) {
	if m == nil {
		return
	}
	if _, found := m.values[key]; !found {
		return
	}
	delete(m.values, key)
	m.keys = slices.DeleteFunc(m.keys, func(k string) bool { return k == key })
}

// Keys returns a copy, so callers may modify the map while ranging over it.
func (m *OrderedMap[V]) Keys() []string {
	if m == nil {
		return nil
	}
	return slices.Clone(m.keys)
}

func (m *OrderedMap[V]) MarshalJSON() ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteByte('{')
	for i, key := range m.keys {
		if i > 0 {
			buf.WriteByte(',')
		}
		k, err := json.Marshal(key)
		if err != nil {
			return nil, err
		}
		buf.Write(k)
		buf.WriteByte(':')
		v, err := json.Marshal(m.values[key])
		if err != nil {
			return nil, err
		}
		buf.Write(v)
	}
	buf.WriteByte('}')
	return buf.Bytes(), nil
}

// MapValues copies m in order, converting each value with f. A nil map stays nil.
func MapValues[A, B any](m *OrderedMap[A], f func(A) B) *OrderedMap[B] {
	if m == nil {
		return nil
	}
	dst := NewOrderedMap[B]()
	for _, key := range m.keys {
		dst.Set(key, f(m.values[key]))
	}
	return dst
}
