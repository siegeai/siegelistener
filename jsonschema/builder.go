package jsonschema

// dfsBuilder is used to construct a Schema from a representation that is easiest to
// traverse in a depth-first order, such as JSON itself.
type dfsBuilder struct {
}

func newBuilder() dfsBuilder {
	return dfsBuilder{}
}

func (b *dfsBuilder) build() (*Schema, error) {
	return nil, nil
}

func (b *dfsBuilder) beginObject() error {
	return nil
}

func (b *dfsBuilder) endObject() error {
	return nil
}
