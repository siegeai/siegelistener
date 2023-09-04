package apispec

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestParseObjectEmpty(t *testing.T) {
	bs := []byte("{}")
	s, err := ParseSampleBodyBytes(bs)
	assert.Nil(t, err)
	assert.Equal(t, s.Kind(), SchemaKindObject)

	obj := s.AsObject()
	assert.Equal(t, len(obj.Fields), 0)
}

func TestParseObjectOneFieldString(t *testing.T) {
	bs := []byte(`{"field": "string-val"}`)
	s, err := ParseSampleBodyBytes(bs)
	assert.Nil(t, err)
	assert.Equal(t, s.Kind(), SchemaKindObject)

	obj := s.AsObject()
	assert.Equal(t, len(obj.Fields), 1)
	assert.Equal(t, obj.Fields[0].Key, "field")
	assert.NotNil(t, obj.Fields[0].Value)
	assert.Equal(t, obj.Fields[0].Value.Kind(), SchemaKindValue)
	assert.True(t, obj.Fields[0].Value.AsValue().MaybeString)
	assert.False(t, obj.Fields[0].Value.AsValue().MaybeNumber)
	assert.False(t, obj.Fields[0].Value.AsValue().MaybeNull)
	assert.False(t, obj.Fields[0].Value.AsValue().MaybeBool)
}

func TestParseObjectOneFieldNumber(t *testing.T) {
	bs := []byte(`{"field": 1234}`)
	s, err := ParseSampleBodyBytes(bs)
	assert.Nil(t, err)
	assert.Equal(t, s.Kind(), SchemaKindObject)

	obj := s.AsObject()
	assert.Equal(t, len(obj.Fields), 1)
	assert.Equal(t, obj.Fields[0].Key, "field")
	assert.NotNil(t, obj.Fields[0].Value)
	assert.Equal(t, obj.Fields[0].Value.Kind(), SchemaKindValue)
	assert.False(t, obj.Fields[0].Value.AsValue().MaybeString)
	assert.True(t, obj.Fields[0].Value.AsValue().MaybeNumber)
	assert.False(t, obj.Fields[0].Value.AsValue().MaybeNull)
	assert.False(t, obj.Fields[0].Value.AsValue().MaybeBool)
}

func TestParseObjectOneFieldBool(t *testing.T) {
	bs := []byte(`{"field": true}`)
	s, err := ParseSampleBodyBytes(bs)
	assert.Nil(t, err)
	assert.Equal(t, s.Kind(), SchemaKindObject)

	obj := s.AsObject()
	assert.Equal(t, len(obj.Fields), 1)
	assert.Equal(t, obj.Fields[0].Key, "field")
	assert.NotNil(t, obj.Fields[0].Value)
	assert.Equal(t, obj.Fields[0].Value.Kind(), SchemaKindValue)
	assert.False(t, obj.Fields[0].Value.AsValue().MaybeString)
	assert.False(t, obj.Fields[0].Value.AsValue().MaybeNumber)
	assert.False(t, obj.Fields[0].Value.AsValue().MaybeNull)
	assert.True(t, obj.Fields[0].Value.AsValue().MaybeBool)
}

func TestParseObjectOneFieldNull(t *testing.T) {
	bs := []byte(`{"field": null}`)
	s, err := ParseSampleBodyBytes(bs)
	assert.Nil(t, err)
	assert.Equal(t, s.Kind(), SchemaKindObject)

	obj := s.AsObject()
	assert.Equal(t, len(obj.Fields), 1)
	assert.Equal(t, obj.Fields[0].Key, "field")
	assert.NotNil(t, obj.Fields[0].Value)
	assert.Equal(t, obj.Fields[0].Value.Kind(), SchemaKindValue)
	assert.False(t, obj.Fields[0].Value.AsValue().MaybeString)
	assert.False(t, obj.Fields[0].Value.AsValue().MaybeNumber)
	assert.True(t, obj.Fields[0].Value.AsValue().MaybeNull)
	assert.False(t, obj.Fields[0].Value.AsValue().MaybeBool)
}

func TestParseArrayEmpty(t *testing.T) {
	bs := []byte("[]")
	s, err := ParseSampleBodyBytes(bs)
	assert.Nil(t, err)
	assert.Equal(t, s.Kind(), SchemaKindArray)

	arr := s.AsArray()
	assert.Nil(t, arr.Element)
}

func TestParseArrayComposite(t *testing.T) {
	bs := []byte(`[{"a": 123}, {"b": "hi"}, null]`)
	s, err := ParseSampleBodyBytes(bs)
	assert.Nil(t, err)
	assert.Equal(t, s.Kind(), SchemaKindArray)

	arr := s.AsArray()
	assert.NotNil(t, arr.Element)
	assert.Equal(t, arr.Element.Kind(), SchemaKindUnion)

	u := arr.Element.AsUnion()
	assert.NotNil(t, u.O)
	assert.Nil(t, u.A)
	assert.NotNil(t, u.V)
}
