package jsonschema

import (
	"github.com/valyala/fastjson"
)

func ParseFastJson(v *fastjson.Value) (*Schema, error) {
	b := newBuilder()
	err := parseFastJsonValue(&b, v)
	if err != nil {
		return nil, err
	}
	return b.build()
}

func parseBytesUsingFastJson(b []byte) (*Schema, error) {
	v, err := fastjson.ParseBytes(b)
	if err != nil {
		return nil, err
	}
	return ParseFastJson(v)
}

func parseFastJsonValue(b *dfsBuilder, v *fastjson.Value) error {
	switch v.Type() {
	case fastjson.TypeNull:
		return parseFastJsonNull(b)
	case fastjson.TypeObject:
		o, err := v.Object()
		if err != nil {
			return err
		}
		return parseFastJsonObject(b, o)
	case fastjson.TypeArray:
		a, err := v.Array()
		if err != nil {
			return err
		}
		return parseFastJsonArray(b, a)
	case fastjson.TypeString:
		s := v.String()
		return parseFastJsonString(b, s)
	case fastjson.TypeNumber:
		return parseFastJsonNumber(b)
	case fastjson.TypeTrue:
		return parseFastJsonBool(b)
	case fastjson.TypeFalse:
		return parseFastJsonBool(b)
	}
	return nil
}

func parseFastJsonNull(b *dfsBuilder) error {
	return nil
}

func parseFastJsonObject(b *dfsBuilder, o *fastjson.Object) error {
	if err := b.beginObject(); err != nil {
		return err
	}
	if err := b.endObject(); err != nil {
		return err
	}
	return nil
}

func parseFastJsonArray(b *dfsBuilder, vs []*fastjson.Value) error {
	return nil
}

func parseFastJsonString(b *dfsBuilder, s string) error {
	return nil
}

func parseFastJsonNumber(b *dfsBuilder) error {
	return nil
}

func parseFastJsonBool(b *dfsBuilder) error {
	return nil
}
