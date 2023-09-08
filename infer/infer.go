package infer

import (
	"github.com/getkin/kin-openapi/openapi3"
	"siege/merge"
)

func NewObjectSchema(props map[string]*openapi3.Schema) *openapi3.Schema {
	ps := make(map[string]*openapi3.SchemaRef, len(props))
	rs := make([]string, len(props))
	i := 0
	for k, v := range props {
		ps[k] = v.NewRef()
		rs[i] = k
		i += 1
	}
	return &openapi3.Schema{
		Type:       openapi3.TypeObject,
		Required:   rs,
		Properties: ps,
	}
}

func NewArraySchema(elems []*openapi3.Schema) *openapi3.Schema {
	var item *openapi3.Schema
	for _, e := range elems {
		item = merge.Schema(item, e)
	}
	return &openapi3.Schema{
		Type:  openapi3.TypeArray,
		Items: item.NewRef(),
	}
}

func NewStringSchema(s string) *openapi3.Schema {
	// especially interested in inferring the "format" property
	// should look for things like uuid, email, various date formats
	return &openapi3.Schema{
		Type: openapi3.TypeString,
	}
}

func NewNumberSchema() *openapi3.Schema {
	return &openapi3.Schema{
		Type: openapi3.TypeNumber,
	}
}

func NewBooleanSchema(b bool) *openapi3.Schema {
	return &openapi3.Schema{
		Type: openapi3.TypeBoolean,
	}
}

func NewNullSchema() *openapi3.Schema {
	// Is this actually the right way to represent this?
	return &openapi3.Schema{
		Nullable: true,
	}
}
