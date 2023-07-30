package jsonschema

type schemaKind int32
type schemaID int32

type Schema struct {
	rootID   schemaID
	rootKind schemaKind
}

type SchemaStorage struct {
	structs map[schemaID]*StructSchema
	unions  map[schemaID]*UnionSchema
	arrays  map[schemaID]*ArraySchema
}

type StructSchema struct {
	id     schemaID
	fields []StructSchemaField
}

type StructSchemaField struct {
	name     string
	required bool
	nullable bool
	schema   Schema
}

type UnionSchema struct {
	id      schemaID
	schemas []Schema
}

type ArraySchema struct {
	id     schemaID
	maxLen int
	unique bool
	schema Schema
}

type StringSchema struct {
	id           schemaID
	regex        string
	maybeNumeric bool
	maybeUUID    bool
}

func Merge(a, b *Schema) *Schema {
	return nil
}

// schemas coming from a sample will be pure, in the sense that each field has a unique
// schema (ignoring arrays, maybe).
// merged schemas need to represent possibly any combination of types, primitives,
// object, mixed.
// composite node type?
// ref node type?
// where do we store metrics?
// want a count at the top level, like, I've seen this exact schema X times?
// will this be stupid for arrays?

// where do I store things like, array length, array unique, etc etc?
type schemaElem struct {
	t schemaElemType
	k schemaElemKind
	p schemaElemIndex
	n schemaElemIndex
}

type schemaElemStructField struct {
	name   string
	schema int32
}

type schemaElemIndex uint16

type schemaElemType uint16

const (
	schemaElemTypeNone   schemaElemType = 0
	schemaElemTypeStruct schemaElemType = 1
	schemaElemTypeUnion  schemaElemType = 2
	schemaElemTypeArray  schemaElemType = 3
	schemaElemTypeString schemaElemType = 4
	schemaElemTypeBool   schemaElemType = 5
)

type schemaElemKind uint16
