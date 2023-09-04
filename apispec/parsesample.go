package apispec

import (
	"github.com/valyala/fastjson"
)

func ParseSampleBodyBytes(b []byte) (*Schema, error) {
	return parseSampleBodyBytesUsingFastJson(b)
}

func ParseSampleBodyFastJson(v *fastjson.Value) (*Schema, error) {
	n, err := parseFastJsonValue(v)
	if err != nil {
		return nil, err
	}
	s := Schema{
		Root: n,
	}
	return &s, nil
}

func parseSampleBodyBytesUsingFastJson(b []byte) (*Schema, error) {
	v, err := fastjson.ParseBytes(b)
	if err != nil {
		return nil, err
	}
	return ParseSampleBodyFastJson(v)
}

func parseFastJsonValue(v *fastjson.Value) (Node, error) {
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
		return parseFastJsonBool()
	case fastjson.TypeFalse:
		return parseFastJsonBool()
	case fastjson.TypeNull:
		return parseFastJsonNull()
	}

	panic("should be unreachable")
}

func parseFastJsonObject(o *fastjson.Object) (Node, error) {
	n := ObjectNode{
		Fields: make([]ObjectNodeField, 0),
	}

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

		f := ObjectNodeField{
			Key:      string(key),
			Value:    child,
			Required: true,
			Nullable: false,
		}

		n.Fields = append(n.Fields, f)
	})

	return &n, visitErr
}

func parseFastJsonArray(vs []*fastjson.Value) (Node, error) {
	n := ArrayNode{
		Element: nil,
	}
	return &n, nil
}

func parseFastJsonString(s string) (Node, error) {
	n := ValueNode{
		MaybeString: true,
		MaybeNumber: false,
		MaybeBool:   false,
		MaybeNull:   false,
	}
	return &n, nil
}

func parseFastJsonNumber() (Node, error) {
	n := ValueNode{
		MaybeString: false,
		MaybeNumber: true,
		MaybeBool:   false,
		MaybeNull:   false,
	}
	return &n, nil
}

func parseFastJsonBool() (Node, error) {
	n := ValueNode{
		MaybeString: false,
		MaybeNumber: false,
		MaybeBool:   true,
		MaybeNull:   false,
	}
	return &n, nil
}

func parseFastJsonNull() (Node, error) {
	n := ValueNode{
		MaybeString: false,
		MaybeNumber: false,
		MaybeBool:   false,
		MaybeNull:   true,
	}
	return &n, nil
}
