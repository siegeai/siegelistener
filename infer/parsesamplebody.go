package infer

import (
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/valyala/fastjson"
)

func ParseSampleBodyBytes(b []byte) (*openapi3.Schema, error) {
	return parseSampleBodyBytesUsingFastJson(b)
}

func ParseSampleBodyFastJson(v *fastjson.Value) (*openapi3.Schema, error) {
	n, err := parseFastJsonValue(v)
	if err != nil {
		return nil, err
	}
	return n, nil
}

func parseSampleBodyBytesUsingFastJson(b []byte) (*openapi3.Schema, error) {
	v, err := fastjson.ParseBytes(b)
	if err != nil {
		return nil, err
	}
	return ParseSampleBodyFastJson(v)
}

func parseFastJsonValue(v *fastjson.Value) (*openapi3.Schema, error) {
	switch v.Type() {
	case fastjson.TypeObject:
		o, err := v.Object()
		if err != nil {
			return nil, err
		}
		return parseFastJsonObject(o)
	case fastjson.TypeArray:
		a, err := v.Array()
		if err != nil {
			return nil, err
		}
		return parseFastJsonArray(a)
	case fastjson.TypeString:
		s := v.String()
		return parseFastJsonString(s)
	case fastjson.TypeNumber:
		return parseFastJsonNumber()
	case fastjson.TypeTrue:
		return parseFastJsonBool(true)
	case fastjson.TypeFalse:
		return parseFastJsonBool(false)
	case fastjson.TypeNull:
		return parseFastJsonNull()
	}

	panic("should be unreachable")
}

func parseFastJsonObject(o *fastjson.Object) (*openapi3.Schema, error) {
	ps := make(map[string]*openapi3.Schema)

	var visitErr error
	o.Visit(func(key []byte, v *fastjson.Value) {
		if visitErr != nil {
			return
		}
		child, childErr := parseFastJsonValue(v)
		if childErr != nil {
			visitErr = childErr
			return
		}

		ps[string(key)] = child
	})

	if visitErr != nil {
		return nil, visitErr
	}

	return NewObjectSchema(ps), nil
}

func parseFastJsonArray(vs []*fastjson.Value) (*openapi3.Schema, error) {
	es := make([]*openapi3.Schema, len(vs))
	for i, v := range vs {
		e, err := parseFastJsonValue(v)
		if err != nil {
			return nil, err
		}
		es[i] = e
	}
	return NewArraySchema(es), nil
}

func parseFastJsonString(s string) (*openapi3.Schema, error) {
	return NewStringSchema(s), nil
}

func parseFastJsonNumber() (*openapi3.Schema, error) {
	return NewNumberSchema(), nil
}

func parseFastJsonBool(b bool) (*openapi3.Schema, error) {
	return NewBooleanSchema(b), nil
}

func parseFastJsonNull() (*openapi3.Schema, error) {
	return NewNullSchema(), nil
}
