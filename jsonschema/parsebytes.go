package jsonschema

func ParseBytes(b []byte) (*Schema, error) {
	return parseBytesUsingFastJson(b)
}
