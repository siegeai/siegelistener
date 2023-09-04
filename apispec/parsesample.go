package apispec

import (
	"github.com/valyala/fastjson"
)

func ParseSampleBodyBytes(b []byte) (Schema, error) {
	return parseSampleBodyBytesUsingFastJson(b)
}

func ParseSampleBodyFastJson(v *fastjson.Value) (Schema, error) {
	n, err := parseFastJsonValue(v)
	if err != nil {
		return nil, err
	}
	return n, nil
}

func parseSampleBodyBytesUsingFastJson(b []byte) (Schema, error) {
	v, err := fastjson.ParseBytes(b)
	if err != nil {
		return nil, err
	}
	return ParseSampleBodyFastJson(v)
}

func parseFastJsonValue(v *fastjson.Value) (Schema, error) {
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

func parseFastJsonObject(o *fastjson.Object) (Schema, error) {
	n := ObjectSchema{
		Fields: make([]ObjectSchemaField, 0),
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

		f := ObjectSchemaField{
			Key:      string(key),
			Value:    child,
			Required: true,
		}

		n.Fields = append(n.Fields, f)
	})

	return &n, visitErr
}

func parseFastJsonArray(vs []*fastjson.Value) (Schema, error) {
	var err error
	var res Schema

	for _, v := range vs {
		var tmp Schema
		tmp, err = parseFastJsonValue(v)
		if err != nil {
			break
		}
		res = Merge(res, tmp)
	}

	if err != nil {
		return nil, err
	}

	n := ArraySchema{Element: res}
	return &n, nil
}

func parseFastJsonString(s string) (Schema, error) {
	n := ValueSchema{
		MaybeString: true,
		MaybeNumber: false,
		MaybeBool:   false,
		MaybeNull:   false,
	}
	return &n, nil
}

func parseFastJsonNumber() (Schema, error) {
	n := ValueSchema{
		MaybeString: false,
		MaybeNumber: true,
		MaybeBool:   false,
		MaybeNull:   false,
	}
	return &n, nil
}

func parseFastJsonBool() (Schema, error) {
	n := ValueSchema{
		MaybeString: false,
		MaybeNumber: false,
		MaybeBool:   true,
		MaybeNull:   false,
	}
	return &n, nil
}

func parseFastJsonNull() (Schema, error) {
	n := ValueSchema{
		MaybeString: false,
		MaybeNumber: false,
		MaybeBool:   false,
		MaybeNull:   true,
	}
	return &n, nil
}
