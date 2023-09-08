package merge

import (
	"encoding/json"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestMergeWithTrivial(t *testing.T) {
	doc := openapi3.T{
		OpenAPI: "3.0.0",
		Info:    &openapi3.Info{Title: "Example", Version: "0.0.1"},
		Paths:   openapi3.Paths{},
	}

	bs, err := json.Marshal(doc)
	assert.Nil(t, err)

	docstr := string(bs)
	assert.NotEmpty(t, docstr)
}
