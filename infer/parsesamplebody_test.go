package infer

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseObjectEmpty(t *testing.T) {
	bs := []byte("{}")
	s, err := ParseSampleBodyBytes(bs)
	assert.Nil(t, err)
	assert.NotNil(t, s)
}

func TestParseObjectOneFieldString(t *testing.T) {
	bs := []byte(`{"field": "string-val"}`)
	s, err := ParseSampleBodyBytes(bs)
	assert.Nil(t, err)
	assert.NotNil(t, s)
}

func TestParseObjectOneFieldNumber(t *testing.T) {
	bs := []byte(`{"field": 1234}`)
	s, err := ParseSampleBodyBytes(bs)
	assert.Nil(t, err)
	assert.NotNil(t, s)
}

func TestParseObjectOneFieldBool(t *testing.T) {
	bs := []byte(`{"field": true}`)
	s, err := ParseSampleBodyBytes(bs)
	assert.Nil(t, err)
	assert.NotNil(t, s)
}

func TestParseObjectOneFieldNull(t *testing.T) {
	bs := []byte(`{"field": null}`)
	s, err := ParseSampleBodyBytes(bs)
	assert.Nil(t, err)
	assert.NotNil(t, s)
}

func TestParseArrayEmpty(t *testing.T) {
	bs := []byte("[]")
	s, err := ParseSampleBodyBytes(bs)
	assert.Nil(t, err)
	assert.NotNil(t, s)
}

func TestParseArrayCompositeHomogeneous(t *testing.T) {
	bs := []byte(`[{"a": 123}, {"b": "hi"}]`)
	s, err := ParseSampleBodyBytes(bs)
	assert.Nil(t, err)
	assert.NotNil(t, s)
}

func TestParseArrayCompositeHeterogeneous(t *testing.T) {
	bs := []byte(`[{"a": 123}, null]`)
	s, err := ParseSampleBodyBytes(bs)
	assert.Nil(t, err)
	assert.NotNil(t, s)
}
