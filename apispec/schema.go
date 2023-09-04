package apispec

import (
	"bytes"
	"fmt"
)

type NodeKind int16

const (
	NodeKindObject NodeKind = 1
	NodeKindArray  NodeKind = 2
	NodeKindValue  NodeKind = 3
)

type Schema struct {
	Root Node
}

type Node interface {
	fmt.Stringer
	Kind() NodeKind
	AsObject() *ObjectNode
	AsArray() *ArrayNode
	AsValue() *ValueNode
}

type ObjectNode struct {
	Fields []ObjectNodeField
}

type ObjectNodeField struct {
	Key      string
	Value    Node
	Required bool
	Nullable bool
}

func (o *ObjectNode) Kind() NodeKind {
	return NodeKindObject
}

func (o *ObjectNode) AsObject() *ObjectNode {
	return o
}

func (o *ObjectNode) AsArray() *ArrayNode {
	panic("object is not an array")
}

func (o *ObjectNode) AsValue() *ValueNode {
	panic("object is not a value")
}

func (o *ObjectNode) String() string {
	var buf bytes.Buffer
	buf.WriteString("<obj ")

	for _, f := range o.Fields {
		buf.WriteString(f.Key)
		buf.WriteString(": ")
		buf.WriteString(f.Value.String())
	}

	buf.WriteString(">")
	return buf.String()
}

type ArrayNode struct {
	Element Node
}

func (a *ArrayNode) Kind() NodeKind {
	return NodeKindArray
}

func (a *ArrayNode) AsObject() *ObjectNode {
	panic("array is not an object")
}

func (a *ArrayNode) AsArray() *ArrayNode {
	return a
}

func (a *ArrayNode) AsValue() *ValueNode {
	panic("array is not a value")
}

func (a *ArrayNode) String() string {
	if a.Element != nil {
		return fmt.Sprintf("<arr %s>", a.Element.String())
	} else {
		return "<arr empty>"
	}
}

type ValueNode struct {
	MaybeString bool
	MaybeNumber bool
	MaybeBool   bool
	MaybeNull   bool
}

func (v *ValueNode) Kind() NodeKind {
	return NodeKindValue
}

func (v *ValueNode) AsObject() *ObjectNode {
	panic("value is not an object")
}

func (v *ValueNode) AsArray() *ArrayNode {
	panic("value is not an array")
}

func (v *ValueNode) AsValue() *ValueNode {
	return v
}

func (v *ValueNode) String() string {
	// absolute jank
	var buf bytes.Buffer
	var subsequent bool
	buf.WriteString("<")
	if v.MaybeString {
		buf.WriteString("str")
		subsequent = true
	}
	if v.MaybeNumber {
		if subsequent {
			buf.WriteString("|num")
		} else {
			buf.WriteString("num")
		}
		subsequent = true
	}
	if v.MaybeBool {
		if subsequent {
			buf.WriteString("|bool")
		} else {
			buf.WriteString("bool")
		}
		subsequent = true
	}
	if v.MaybeNull {
		if subsequent {
			buf.WriteString("|null")
		} else {
			buf.WriteString("null")
		}
		subsequent = true
	}
	buf.WriteString(">")
	return buf.String()
}

func Merge(a, b *Schema) *Schema {
	return nil
}
