package apispec

type SchemaKind int

const (
	SchemaKindObject SchemaKind = 1
	SchemaKindArray  SchemaKind = 2
	SchemaKindValue  SchemaKind = 3
	SchemaKindUnion  SchemaKind = 4
)

type Schema interface {
	Kind() SchemaKind
	AsObject() *ObjectSchema
	AsArray() *ArraySchema
	AsValue() *ValueSchema
	AsUnion() *UnionSchema
}

type ObjectSchema struct {
	Fields []ObjectSchemaField // Nothing keeps track of field order anywhere yet
}

type ObjectSchemaField struct {
	Key      string
	Value    Schema
	Required bool
}

func (o *ObjectSchema) Kind() SchemaKind {
	return SchemaKindObject
}

func (o *ObjectSchema) AsObject() *ObjectSchema {
	return o
}

func (o *ObjectSchema) AsArray() *ArraySchema {
	panic("object is not an array")
}

func (o *ObjectSchema) AsValue() *ValueSchema {
	panic("object is not a value")
}

func (o *ObjectSchema) AsUnion() *UnionSchema {
	panic("object is not a union")
}

type ArraySchema struct {
	Element Schema
}

func (a *ArraySchema) Kind() SchemaKind {
	return SchemaKindArray
}

func (a *ArraySchema) AsObject() *ObjectSchema {
	panic("array is not an object")
}

func (a *ArraySchema) AsArray() *ArraySchema {
	return a
}

func (a *ArraySchema) AsValue() *ValueSchema {
	panic("array is not a value")
}

func (a *ArraySchema) AsUnion() *UnionSchema {
	panic("array is not a union")
}

// UnionSchema Very dumb union schema. If two different object schemas are merged then
// that will be reflected in a bunch of non-required fields. This will allow combination
// of different types, e.g., null or object. This will need a big rework eventually
type UnionSchema struct {
	O Schema // not using *ObjectSchema because then we get nil interface values that don't match nil
	A Schema
	V Schema
}

func (u *UnionSchema) Kind() SchemaKind {
	return SchemaKindUnion
}

func (u *UnionSchema) AsObject() *ObjectSchema {
	panic("union is not an object")
}

func (u *UnionSchema) AsArray() *ArraySchema {
	panic("union is not an array")
}

func (u *UnionSchema) AsValue() *ValueSchema {
	panic("union is not a value")
}

func (u *UnionSchema) AsUnion() *UnionSchema {
	return u
}

type ValueSchema struct {
	MaybeString bool
	MaybeNumber bool
	MaybeBool   bool
	MaybeNull   bool
}

func (v *ValueSchema) Kind() SchemaKind {
	return SchemaKindValue
}

func (v *ValueSchema) AsObject() *ObjectSchema {
	panic("value is not an object")
}

func (v *ValueSchema) AsArray() *ArraySchema {
	panic("value is not an array")
}

func (v *ValueSchema) AsValue() *ValueSchema {
	return v
}

func (v *ValueSchema) AsUnion() *UnionSchema {
	panic("value is not a union")
}

func NewValueSchemaString() *ValueSchema {
	return &ValueSchema{
		MaybeString: true,
		MaybeNumber: false,
		MaybeBool:   false,
		MaybeNull:   false,
	}
}

func NewValueSchemaNumber() *ValueSchema {
	return &ValueSchema{
		MaybeString: false,
		MaybeNumber: true,
		MaybeBool:   false,
		MaybeNull:   false,
	}
}

func NewValueSchemaBool() *ValueSchema {
	return &ValueSchema{
		MaybeString: false,
		MaybeNumber: false,
		MaybeBool:   true,
		MaybeNull:   false,
	}
}

func NewValueSchemaNull() *ValueSchema {
	return &ValueSchema{
		MaybeString: false,
		MaybeNumber: false,
		MaybeBool:   false,
		MaybeNull:   true,
	}
}

func Merge(a, b Schema) Schema {
	if a == nil && b == nil {
		return nil
	}
	if a != nil && b == nil {
		return a
	}
	if a == nil && b != nil {
		return b
	}

	if a.Kind() == SchemaKindObject && b.Kind() == SchemaKindObject {
		return mergeObjects(a.AsObject(), b.AsObject())
	}

	if a.Kind() == SchemaKindArray && b.Kind() == SchemaKindArray {
		return mergeArrays(a.AsArray(), b.AsArray())
	}

	if a.Kind() == SchemaKindValue && b.Kind() == SchemaKindValue {
		return mergeValues(a.AsValue(), b.AsValue())
	}

	// unions / different kinds
	return mergeUnions(intoUnion(a), intoUnion(b))
}

// moop moop moop
type moop struct {
	af *ObjectSchemaField
	bf *ObjectSchemaField
}

func mergeObjects(a, b *ObjectSchema) Schema {
	pairs := make(map[string]moop)
	for _, f := range a.Fields {
		pairs[f.Key] = moop{af: &f}
	}
	for _, f := range b.Fields {
		if pair, ok := pairs[f.Key]; ok {
			pair.bf = &f
			pairs[f.Key] = pair
		} else {
			pair := moop{bf: &f}
			pairs[f.Key] = pair
		}
	}

	fields := make([]ObjectSchemaField, 0, len(pairs))
	for k, v := range pairs {
		if v.af == nil {
			f := *v.bf
			f.Required = false
			fields = append(fields, f)
			continue
		}
		if v.bf == nil {
			f := *v.af
			f.Required = false
			fields = append(fields, f)
			continue
		}

		f := ObjectSchemaField{
			Key:      k,
			Value:    Merge(v.af.Value, v.bf.Value),
			Required: v.af.Required && v.bf.Required,
		}

		fields = append(fields, f)
	}

	return &ObjectSchema{Fields: fields}
}

func mergeArrays(a, b *ArraySchema) Schema {
	elem := Merge(a.Element, b.Element)
	return &ArraySchema{Element: elem}
}

func mergeValues(a, b *ValueSchema) Schema {
	return &ValueSchema{
		MaybeString: a.MaybeString || b.MaybeString,
		MaybeNumber: a.MaybeNumber || b.MaybeNumber,
		MaybeBool:   a.MaybeBool || b.MaybeBool,
		MaybeNull:   a.MaybeNull || b.MaybeNull,
	}
}

func mergeUnions(a, b *UnionSchema) Schema {
	// should have assertions that each branch is the right kind
	return &UnionSchema{
		O: Merge(a.O, b.O),
		A: Merge(a.A, b.A),
		V: Merge(a.V, b.V),
	}
}

func intoUnion(s Schema) *UnionSchema {
	switch s.Kind() {
	case SchemaKindObject:
		return &UnionSchema{O: s.AsObject(), A: nil, V: nil}
	case SchemaKindArray:
		return &UnionSchema{O: nil, A: s.AsArray(), V: nil}
	case SchemaKindValue:
		return &UnionSchema{O: nil, A: nil, V: s.AsValue()}
	case SchemaKindUnion:
		return s.AsUnion()
	}

	panic("should be unreachable")
}
