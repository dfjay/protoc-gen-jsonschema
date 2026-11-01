package modules

import (
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
)

const fieldBehaviorExtension = "google.api.field_behavior"

// fieldBehaviors reads the google.api.field_behavior option of fields. The
// plugin does not link googleapis: the extension comes from the request,
// and the option stays in the unknown fields of the field options.
type fieldBehaviors struct {
	extension protoreflect.ExtensionDescriptor
}

// newFieldBehaviors finds the field_behavior extension among the request's
// files. Without it no field can set the option.
func newFieldBehaviors(files *protoregistry.Files) fieldBehaviors {
	d, err := files.FindDescriptorByName(fieldBehaviorExtension)
	if err != nil {
		return fieldBehaviors{}
	}
	xd, ok := d.(protoreflect.ExtensionDescriptor)
	if !ok || xd.Kind() != protoreflect.EnumKind || xd.ContainingMessage().FullName() != "google.protobuf.FieldOptions" {
		return fieldBehaviors{}
	}
	return fieldBehaviors{extension: xd}
}

// has reports whether field has the field behavior named behavior, such as
// REQUIRED or OUTPUT_ONLY.
func (b fieldBehaviors) has(field protoreflect.FieldDescriptor, behavior protoreflect.Name) bool {
	if b.extension == nil {
		return false
	}
	value := b.extension.Enum().Values().ByName(behavior)
	if value == nil {
		return false
	}
	want := value.Number()

	opts := field.Options().ProtoReflect()
	found := false
	// The option is a known extension if a linked package registered it.
	opts.Range(func(fd protoreflect.FieldDescriptor, v protoreflect.Value) bool {
		if fd.FullName() != fieldBehaviorExtension || !fd.IsList() {
			return true
		}
		list := v.List()
		for i := range list.Len() {
			if list.Get(i).Enum() == want {
				found = true
				return false
			}
		}
		return true
	})
	if found {
		return true
	}

	unknown := opts.GetUnknown()
	for len(unknown) > 0 {
		num, typ, n := protowire.ConsumeTag(unknown)
		if n < 0 {
			return false
		}
		unknown = unknown[n:]
		m := protowire.ConsumeFieldValue(num, typ, unknown)
		if m < 0 {
			return false
		}
		if num == b.extension.Number() && enumListContains(typ, unknown[:m], want) {
			return true
		}
		unknown = unknown[m:]
	}
	return false
}

// enumListContains reports whether the value of one repeated enum field,
// packed or not, contains want.
func enumListContains(typ protowire.Type, value []byte, want protoreflect.EnumNumber) bool {
	switch typ {
	case protowire.VarintType:
		v, n := protowire.ConsumeVarint(value)
		return n > 0 && protoreflect.EnumNumber(v) == want
	case protowire.BytesType:
		packed, n := protowire.ConsumeBytes(value)
		if n < 0 {
			return false
		}
		for len(packed) > 0 {
			v, n := protowire.ConsumeVarint(packed)
			if n < 0 {
				return false
			}
			if protoreflect.EnumNumber(v) == want {
				return true
			}
			packed = packed[n:]
		}
	}
	return false
}
