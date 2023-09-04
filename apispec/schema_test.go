package apispec

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func makeStringValue() *ValueSchema {
	return &ValueSchema{MaybeString: true, MaybeNumber: false, MaybeBool: false, MaybeNull: false}
}

func TestMergeObjectsSame(t *testing.T) {
	a := ObjectSchema{Fields: []ObjectSchemaField{{
		Key:      "aaa",
		Value:    makeStringValue(),
		Required: true,
	}}}

	b := ObjectSchema{Fields: []ObjectSchemaField{{
		Key:      "aaa",
		Value:    makeStringValue(),
		Required: true,
	}}}

	c := Merge(&a, &b)
	m, ok := c.(*ObjectSchema)

	assert.True(t, ok)
	assert.Equal(t, len(m.Fields), 1)
	assert.Equal(t, m.Fields[0].Key, "aaa")
	assert.Equal(t, m.Fields[0].Value.Kind(), SchemaKindValue)
	assert.True(t, m.Fields[0].Value.AsValue().MaybeString)
	assert.True(t, m.Fields[0].Required)
}

func TestMergeObjectsDifferent(t *testing.T) {
	a := ObjectSchema{Fields: []ObjectSchemaField{{
		Key:      "aaa",
		Value:    makeStringValue(),
		Required: true,
	}}}

	b := ObjectSchema{Fields: []ObjectSchemaField{{
		Key:      "bbb",
		Value:    makeStringValue(),
		Required: true,
	}}}

	c := Merge(&a, &b)
	m, ok := c.(*ObjectSchema)

	// TODO this is fundamentally broken because Nothing guarantees order of fields...
	assert.True(t, ok)
	assert.Equal(t, len(m.Fields), 2)
	assert.Equal(t, m.Fields[0].Key, "aaa")
	assert.False(t, m.Fields[0].Required)
	assert.Equal(t, m.Fields[1].Key, "bbb")
	assert.False(t, m.Fields[1].Required)
}
