package infer

import (
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/google/uuid"
	"github.com/valyala/fastjson"
)

func ParseSampleBodyBytes(b []byte, eventLog *EventLog) (*openapi3.Schema, error) {
	return parseSampleBodyBytesUsingFastJson(b, eventLog)
}

func ParseSampleBodyFastJson(v *fastjson.Value, eventLog *EventLog) (*openapi3.Schema, error) {
	n, err := parseFastJsonValue(v, 0, "", eventLog)
	if err != nil {
		return nil, err
	}
	return n, nil
}

func parseSampleBodyBytesUsingFastJson(b []byte, eventLog *EventLog) (*openapi3.Schema, error) {
	v, err := fastjson.ParseBytes(b)
	if err != nil {
		return nil, err
	}
	return ParseSampleBodyFastJson(v, eventLog)
}

func parseFastJsonValue(v *fastjson.Value, depth int, key string, eventLog *EventLog) (*openapi3.Schema, error) {
	switch v.Type() {
	case fastjson.TypeObject:
		o, err := v.Object()
		if err != nil {
			return nil, err
		}
		return parseFastJsonObject(o, depth, key, eventLog)
	case fastjson.TypeArray:
		a, err := v.Array()
		if err != nil {
			return nil, err
		}
		return parseFastJsonArray(a, depth, key, eventLog)
	case fastjson.TypeString:
		s := v.String()
		return parseFastJsonString(s, depth, key, eventLog)
	case fastjson.TypeNumber:
		n, err := v.Float64()
		if err != nil {
			return parseFastJsonNumber(&n, depth, key, eventLog)
		} else {
			i, err := v.Int64()
			if err != nil {
				return parseFastJsonNumberInt(&i, depth, key, eventLog)
			}
		}
		return parseFastJsonNumber(nil, depth, key, eventLog)
	case fastjson.TypeTrue:
		return parseFastJsonBool(true, depth, key, eventLog)
	case fastjson.TypeFalse:
		return parseFastJsonBool(false, depth, key, eventLog)
	case fastjson.TypeNull:
		return parseFastJsonNull(depth, key, eventLog)
	}

	panic("should be unreachable")
}

func parseFastJsonObject(o *fastjson.Object, depth int, _key string, eventLog *EventLog) (*openapi3.Schema, error) {
	ps := make(map[string]*openapi3.Schema)

	var visitErr error
	o.Visit(func(key []byte, v *fastjson.Value) {
		if visitErr != nil {
			return
		}
		child, childErr := parseFastJsonValue(v, depth+1, string(key), eventLog)
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

func parseFastJsonArray(vs []*fastjson.Value, depth int, key string, eventLog *EventLog) (*openapi3.Schema, error) {
	es := make([]*openapi3.Schema, len(vs))
	for i, v := range vs {
		// cheese pass through of key
		e, err := parseFastJsonValue(v, depth+1, key, eventLog)
		if err != nil {
			return nil, err
		}
		es[i] = e
	}
	return NewArraySchema(es), nil
}

func keyLooksLikeID(key string) bool {
	lowerKey := strings.ToLower(key)
	if strings.HasSuffix(lowerKey, "id") {
		l := len(key)
		if l <= 2 {
			return true
		}
		if key[l-3] == '-' || key[l-3] == '_' {
			// some-id or some_id
			return true
		}
		if key[l-2] == 'I' && !('A' <= key[l-3] && key[l-3] <= 'Z') {
			// Hack attempt at someID
			return true
		}
		return false
	}
	return false
}

func handleEventLogValue(eventLog *EventLog, depth int, key string, valueLooksLikeID bool, value any) {
	if key == "" {
		// don't add data that doesn't have a key, should only happen if the root of the
		// doc is not an object
		return
	}
	var isID = valueLooksLikeID
	if keyLooksLikeID(key) {
		isID = true
	}
	if isID {
		if depth <= 1 {
			eventLog.PrimaryID[key] = value
		} else {
			eventLog.IDs[key] = value
		}
	} else {
		eventLog.Data[key] = value
	}
}

func parseFastJsonString(s string, depth int, key string, eventLog *EventLog) (*openapi3.Schema, error) {
	if eventLog != nil {
		var isID = false
		if _, err := uuid.Parse(s); err == nil {
			isID = true
		}
		handleEventLogValue(eventLog, depth, key, isID, s)
	}
	return NewStringSchema(s), nil
}

func parseFastJsonNumber(n *float64, depth int, key string, eventLog *EventLog) (*openapi3.Schema, error) {
	if eventLog != nil && n != nil {
		handleEventLogValue(eventLog, depth, key, false, *n)
	}
	return NewNumberSchema(), nil
}

func parseFastJsonNumberInt(n *int64, depth int, key string, eventLog *EventLog) (*openapi3.Schema, error) {
	if eventLog != nil && n != nil {
		handleEventLogValue(eventLog, depth, key, false, *n)
	}
	return NewNumberSchema(), nil
}

func parseFastJsonBool(b bool, depth int, key string, eventLog *EventLog) (*openapi3.Schema, error) {
	if eventLog != nil {
		handleEventLogValue(eventLog, depth, key, false, b)
	}
	return NewBooleanSchema(b), nil
}

func parseFastJsonNull(depth int, key string, eventLog *EventLog) (*openapi3.Schema, error) {
	if eventLog != nil {
		handleEventLogValue(eventLog, depth, key, false, nil)
	}
	return NewNullSchema(), nil
}
