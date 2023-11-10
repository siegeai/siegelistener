package merge

import "github.com/getkin/kin-openapi/openapi3"

// TODO this representation kind of sucks. Would be nice to have an interface based
//   representation that could be backed by either a json doc or a dense tree.

func Doc(a, b *openapi3.T) *openapi3.T {
	if a == nil && b == nil {
		return nil
	}
	if a != nil && b == nil {
		return a
	}
	if a == nil && b != nil {
		return b
	}

	return &openapi3.T{
		Extensions:   Extensions(a.Extensions, b.Extensions),
		OpenAPI:      OpenAPI(a.OpenAPI, b.OpenAPI),
		Components:   Components(a.Components, b.Components),
		Info:         Info(a.Info, b.Info),
		Paths:        Paths(a.Paths, b.Paths),
		Security:     *Security(&a.Security, &b.Security),
		Servers:      *Servers(&a.Servers, &b.Servers),
		Tags:         Tags(a.Tags, b.Tags),
		ExternalDocs: ExternalDocs(a.ExternalDocs, b.ExternalDocs),
	}
}

func Extensions(a, b map[string]interface{}) map[string]interface{} {
	// merging interface{} is gross
	// TODO
	return a
}

func OpenAPI(a, b string) string {
	return mergeString(a, b)
}

func Components(a, b *openapi3.Components) *openapi3.Components {
	if a == nil && b == nil {
		return nil
	}
	if a != nil && b == nil {
		return a
	}
	if a == nil && b != nil {
		return b
	}

	// TODO
	return a
}

func Info(a, b *openapi3.Info) *openapi3.Info {
	if a == nil && b == nil {
		return nil
	}
	if a != nil && b == nil {
		return a
	}
	if a == nil && b != nil {
		return b
	}

	// TODO
	return a
}

func Paths(a, b openapi3.Paths) openapi3.Paths {
	res := make(openapi3.Paths, len(a))

	visited := make(map[string]struct{}, len(a))
	for k, v := range a {
		visited[k] = struct{}{}
		if w, in := b[k]; in {
			res[k] = PathItem(v, w)
		} else {
			res[k] = v
		}
	}

	for k, v := range b {
		if _, in := visited[k]; in {
			continue
		}
		res[k] = v
	}

	return res
}

func PathItem(a, b *openapi3.PathItem) *openapi3.PathItem {
	if a == nil && b == nil {
		return nil
	}
	if a != nil && b == nil {
		return a
	}
	if a == nil && b != nil {
		return b
	}

	// TODO needs a branch to handle the $ref?
	return &openapi3.PathItem{
		Ref:         Ref(a.Ref, b.Ref),
		Summary:     Summary(a.Summary, b.Summary),
		Description: Description(a.Description, b.Description),
		Connect:     Operation(a.Connect, b.Connect),
		Delete:      Operation(a.Delete, b.Delete),
		Get:         Operation(a.Get, b.Get),
		Head:        Operation(a.Head, b.Head),
		Options:     Operation(a.Options, b.Options),
		Patch:       Operation(a.Patch, b.Patch),
		Post:        Operation(a.Post, b.Post),
		Put:         Operation(a.Put, b.Put),
		Trace:       Operation(a.Trace, b.Trace),
		Servers:     *Servers(&a.Servers, &b.Servers),
		Parameters:  Parameters(a.Parameters, b.Parameters),
	}
}

func Ref(a, b string) string {
	return mergeString(a, b)
}

func Summary(a, b string) string {
	return mergeString(a, b)
}

func Description(a, b string) string {
	return mergeString(a, b)
}

func Operation(a, b *openapi3.Operation) *openapi3.Operation {
	if a == nil && b == nil {
		return nil
	}
	if a != nil && b == nil {
		return a
	}
	if a == nil && b != nil {
		return b
	}

	return &openapi3.Operation{
		Extensions:   Extensions(a.Extensions, b.Extensions),
		Tags:         operationTags(a.Tags, b.Tags), // different kind of tags
		Summary:      Summary(a.Summary, b.Summary),
		Description:  Description(a.Description, b.Description),
		OperationID:  operationID(a.OperationID, b.OperationID),
		Parameters:   Parameters(a.Parameters, b.Parameters),
		RequestBody:  RequestBodyRef(a.RequestBody, b.RequestBody),
		Responses:    Responses(a.Responses, b.Responses),
		Callbacks:    Callbacks(a.Callbacks, b.Callbacks),
		Deprecated:   deprecated(a.Deprecated, b.Deprecated),
		Security:     Security(a.Security, b.Security),
		Servers:      Servers(a.Servers, b.Servers),
		ExternalDocs: ExternalDocs(a.ExternalDocs, b.ExternalDocs),
	}
}

func operationID(a, b string) string {
	return mergeString(a, b)
}

func operationTags(a, b []string) []string {
	return a
}

func RequestBodyRef(a, b *openapi3.RequestBodyRef) *openapi3.RequestBodyRef {
	if a == nil && b == nil {
		return nil
	}
	if a != nil && b == nil {
		return a
	}
	if a == nil && b != nil {
		return b
	}
	if a.Ref != "" || b.Ref != "" {
		panic("need to handle this case")
	}
	if a.Value == nil || b.Value == nil {
		panic("need to handle this case")
	}

	return &openapi3.RequestBodyRef{Value: RequestBody(a.Value, b.Value)}
}

func RequestBody(a, b *openapi3.RequestBody) *openapi3.RequestBody {
	return &openapi3.RequestBody{
		Extensions:  Extensions(a.Extensions, b.Extensions),
		Description: Description(a.Description, b.Description),
		Required:    a.Required || b.Required,
		Content:     Content(a.Content, b.Content),
	}
}

func Content(a, b openapi3.Content) openapi3.Content {
	if len(a) == 0 && len(b) == 0 {
		return openapi3.Content{}
	}
	if len(a) == 0 && len(b) != 0 {
		return b
	}
	if len(a) != 0 && len(b) == 0 {
		return a
	}

	c := make(openapi3.Content, max(len(a), len(b)))

	visited := make(map[string]struct{}, len(a))
	for k, v := range a {
		visited[k] = struct{}{}
		if w, in := b[k]; in {
			c[k] = MediaType(v, w)
		} else {
			c[k] = v
		}
	}

	for k, v := range b {
		if _, in := visited[k]; in {
			continue
		}
		c[k] = v
	}

	return c
}

func MediaType(a, b *openapi3.MediaType) *openapi3.MediaType {
	if a == nil && b == nil {
		return nil
	}
	if a != nil && b == nil {
		return a
	}
	if a == nil && b != nil {
		return b
	}

	return &openapi3.MediaType{
		Extensions: Extensions(a.Extensions, b.Extensions),
		Schema:     SchemaRef(a.Schema, b.Schema),
		Example:    Example(a.Example, b.Example),
		Examples:   Examples(a.Examples, b.Examples),
		Encoding:   Encoding(a.Encoding, b.Encoding),
	}
}

func Example(a, b interface{}) interface{} {
	// TODO
	return a
}

func Examples(a, b openapi3.Examples) openapi3.Examples {
	// TODO
	return a
}

func Encoding(a, b map[string]*openapi3.Encoding) map[string]*openapi3.Encoding {
	// TODO
	return a
}

func Responses(a, b openapi3.Responses) openapi3.Responses {
	if len(a) == 0 && len(b) == 0 {
		return openapi3.Responses{}
	}
	if len(a) == 0 && len(b) != 0 {
		return b
	}
	if len(a) != 0 && len(b) == 0 {
		return a
	}

	rs := make(openapi3.Responses, max(len(a), len(b)))

	visited := make(map[string]struct{}, len(a))
	for k, v := range a {
		visited[k] = struct{}{}
		if w, in := b[k]; in {
			rs[k] = ResponseRef(v, w)
		} else {
			rs[k] = v
		}
	}

	for k, v := range b {
		if _, in := visited[k]; in {
			continue
		}
		rs[k] = v
	}

	return rs
}

func ResponseRef(a, b *openapi3.ResponseRef) *openapi3.ResponseRef {
	if a == nil && b == nil {
		return nil
	}
	if a != nil && b == nil {
		return a
	}
	if a == nil && b != nil {
		return b
	}
	if a.Ref != "" || b.Ref != "" {
		panic("need to handle this case")
	}
	if a.Value == nil || b.Value == nil {
		panic("need to handle this case")
	}

	return &openapi3.ResponseRef{Value: Response(a.Value, b.Value)}
}

func Response(a, b *openapi3.Response) *openapi3.Response {
	if a == nil && b == nil {
		return nil
	}
	if a != nil && b == nil {
		return a
	}
	if a == nil && b != nil {
		return b
	}

	dsc := mergeStringPtr(a.Description, b.Description)
	return &openapi3.Response{
		Extensions:  Extensions(a.Extensions, b.Extensions),
		Description: dsc,
		Headers:     Headers(a.Headers, b.Headers),
		Content:     Content(a.Content, b.Content),
		Links:       Links(a.Links, b.Links),
	}
}

func mergeStringPtr(a, b *string) *string {
	if a == nil && b == nil {
		return nil
	}
	if a != nil && b == nil {
		return a
	}
	if a == nil && b != nil {
		return b
	}

	s := mergeString(*a, *b)
	return &s
}

func Headers(a, b map[string]*openapi3.HeaderRef) map[string]*openapi3.HeaderRef {
	if len(a) == 0 && len(b) == 0 {
		return map[string]*openapi3.HeaderRef{}
	}
	if len(a) == 0 && len(b) != 0 {
		return b
	}
	if len(a) != 0 && len(b) == 0 {
		return a
	}

	rs := make(map[string]*openapi3.HeaderRef, max(len(a), len(b)))

	visited := make(map[string]struct{}, len(a))
	for k, v := range a {
		visited[k] = struct{}{}
		if w, in := b[k]; in {
			rs[k] = HeaderRef(v, w)
		} else {
			rs[k] = v
		}
	}

	for k, v := range b {
		if _, in := visited[k]; in {
			continue
		}
		rs[k] = v
	}

	return rs
}

func HeaderRef(a, b *openapi3.HeaderRef) *openapi3.HeaderRef {
	if a == nil && b == nil {
		return nil
	}
	if a != nil && b == nil {
		return a
	}
	if a == nil && b != nil {
		return b
	}
	if a.Ref != "" || b.Ref != "" {
		panic("need to handle this case")
	}
	if a.Value == nil || b.Value == nil {
		panic("need to handle this case")
	}

	return &openapi3.HeaderRef{Value: Header(a.Value, b.Value)}
}

func Header(a, b *openapi3.Header) *openapi3.Header {
	if a == nil && b == nil {
		return nil
	}
	if a != nil && b == nil {
		return a
	}
	if a == nil && b != nil {
		return b
	}

	return &openapi3.Header{
		Parameter: *Parameter(&a.Parameter, &b.Parameter),
	}
}

func Parameter(a, b *openapi3.Parameter) *openapi3.Parameter {
	if a == nil && b == nil {
		return nil
	}
	if a != nil && b == nil {
		return a
	}
	if a == nil && b != nil {
		return b
	}

	// TODO AHHH how much of this bs is there
	return &openapi3.Parameter{
		Extensions:      Extensions(a.Extensions, b.Extensions),
		Name:            mergeString(a.Name, b.Name),
		In:              mergeString(a.In, b.In),
		Description:     Description(a.Description, b.Description),
		Style:           mergeString(a.Style, b.Style),
		Explode:         nil,
		AllowEmptyValue: false,
		AllowReserved:   false,
		Deprecated:      false,
		Required:        false,
		Schema:          nil,
		Example:         nil,
		Examples:        nil,
		Content:         nil,
	}
}

func Links(a, b openapi3.Links) openapi3.Links {
	if len(a) == 0 && len(b) == 0 {
		return openapi3.Links{}
	}
	if len(a) == 0 && len(b) != 0 {
		return b
	}
	if len(a) != 0 && len(b) == 0 {
		return a
	}

	rs := make(openapi3.Links, max(len(a), len(b)))

	visited := make(map[string]struct{}, len(a))
	for k, v := range a {
		visited[k] = struct{}{}
		if w, in := b[k]; in {
			rs[k] = LinkRef(v, w)
		} else {
			rs[k] = v
		}
	}

	for k, v := range b {
		if _, in := visited[k]; in {
			continue
		}
		rs[k] = v
	}

	return rs
}

func LinkRef(a, b *openapi3.LinkRef) *openapi3.LinkRef {
	if a == nil && b == nil {
		return nil
	}
	if a != nil && b == nil {
		return a
	}
	if a == nil && b != nil {
		return b
	}
	if a.Ref != "" || b.Ref != "" {
		panic("need to handle this case")
	}
	if a.Value == nil || b.Value == nil {
		panic("need to handle this case")
	}

	return &openapi3.LinkRef{Value: Link(a.Value, b.Value)}
}

func Link(a, b *openapi3.Link) *openapi3.Link {
	if a == nil && b == nil {
		return nil
	}
	if a != nil && b == nil {
		return a
	}
	if a == nil && b != nil {
		return b
	}

	// TODO oh my god
	return &openapi3.Link{
		Extensions:   Extensions(a.Extensions, b.Extensions),
		OperationRef: "",
		OperationID:  "",
		Description:  "",
		Parameters:   nil,
		Server:       nil,
		RequestBody:  nil,
	}
}

func Callbacks(a, b openapi3.Callbacks) openapi3.Callbacks {
	return a
}

func Parameters(a, b openapi3.Parameters) openapi3.Parameters {
	return a
}

func deprecated(a, b bool) bool {
	return a || b
}

func Security(a, b *openapi3.SecurityRequirements) *openapi3.SecurityRequirements {
	if a == nil && b == nil {
		return nil
	}
	if a != nil && b == nil {
		return a
	}
	if a == nil && b != nil {
		return b
	}

	return a
}

func Servers(a, b *openapi3.Servers) *openapi3.Servers {
	if a == nil && b == nil {
		return nil
	}
	if a != nil && b == nil {
		return a
	}
	if a == nil && b != nil {
		return b
	}

	return a
}

func Tags(a, b openapi3.Tags) openapi3.Tags {
	return a
}

func ExternalDocs(a, b *openapi3.ExternalDocs) *openapi3.ExternalDocs {
	if a == nil && b == nil {
		return nil
	}
	if a != nil && b == nil {
		return a
	}
	if a == nil && b != nil {
		return b
	}

	// TODO
	return a
}

func SchemaRef(a, b *openapi3.SchemaRef) *openapi3.SchemaRef {
	if a == nil && b == nil {
		return nil
	}
	if a != nil && b == nil {
		return a
	}
	if a == nil && b != nil {
		return b
	}
	// TODO needs to be able to follow refs
	if a.Ref != "" || b.Ref != "" {
		panic("need to handle this case")
	}
	if a.Value == nil || b.Value == nil {
		panic("need to handle this case")
	}

	return Schema(a.Value, b.Value).NewRef()
}

func Schema(a, b *openapi3.Schema) *openapi3.Schema {
	if a == nil && b == nil {
		return nil
	}
	if a != nil && b == nil {
		return a
	}
	if a == nil && b != nil {
		return b
	}

	if a.Type == b.Type {
		return mergeSchemaSameType(a, b)
	} else {
		return mergeSchemaDifferentType(a, b)
	}
}

func mergeSchemaSameType(a, b *openapi3.Schema) *openapi3.Schema {
	return &openapi3.Schema{
		Extensions:           Extensions(a.Extensions, b.Extensions),
		OneOf:                mergeSchemaRefs(a.OneOf, b.OneOf),
		AnyOf:                mergeSchemaRefs(a.AnyOf, b.AnyOf),
		AllOf:                mergeSchemaRefs(a.AllOf, b.AllOf),
		Not:                  SchemaRef(a.Not, b.Not),
		Type:                 a.Type,
		Title:                mergeString(a.Title, b.Title),
		Format:               mergeString(a.Format, b.Format),
		Description:          mergeString(a.Description, b.Description),
		Enum:                 nil,
		Default:              nil,
		Example:              nil,
		ExternalDocs:         nil,
		UniqueItems:          false,
		ExclusiveMin:         false,
		ExclusiveMax:         false,
		Nullable:             a.Nullable || b.Nullable,
		ReadOnly:             a.ReadOnly || b.ReadOnly,
		WriteOnly:            a.WriteOnly || b.WriteOnly,
		AllowEmptyValue:      false,
		Deprecated:           false,
		XML:                  nil,
		Min:                  nil,
		Max:                  nil,
		MultipleOf:           nil,
		MinLength:            0,
		MaxLength:            nil,
		Pattern:              "",
		MinItems:             0,
		MaxItems:             nil,
		Items:                SchemaRef(a.Items, b.Items),
		Required:             mergeRequired(a.Required, b.Required),
		Properties:           Schemas(a.Properties, b.Properties),
		MinProps:             0,
		MaxProps:             nil,
		AdditionalProperties: openapi3.AdditionalProperties{},
		Discriminator:        nil,
	}
}

func mergeSchemaRefs(a, b openapi3.SchemaRefs) openapi3.SchemaRefs {
	return a
}

func mergeRequired(a, b []string) []string {
	keep := make(map[string]bool, len(a))
	for _, r := range a {
		keep[r] = false
	}
	n := 0
	for _, r := range b {
		if _, in := keep[r]; in {
			keep[r] = true
			n += 1
		}
	}

	res := make([]string, n)
	m := 0
	for k, v := range keep {
		if v {
			res[m] = k
			m += 1
		}
	}

	if n != m {
		panic("oh no")
	}

	return res
}

func Schemas(a, b openapi3.Schemas) openapi3.Schemas {
	if len(a) == 0 && len(b) == 0 {
		return openapi3.Schemas{}
	}
	if len(a) == 0 && len(b) != 0 {
		return b
	}
	if len(a) != 0 && len(b) == 0 {
		return a
	}

	rs := make(openapi3.Schemas, max(len(a), len(b)))

	visited := make(map[string]struct{}, len(a))
	for k, v := range a {
		visited[k] = struct{}{}
		if w, in := b[k]; in {
			rs[k] = SchemaRef(v, w)
		} else {
			rs[k] = v
		}
	}

	for k, v := range b {
		if _, in := visited[k]; in {
			continue
		}
		rs[k] = v
	}

	return rs
}

func mergeSchemaDifferentType(a, b *openapi3.Schema) *openapi3.Schema {
	af := flattenTypes(a)
	bf := flattenTypes(b)

	oneOf := mergeFlatParams(af.oneOf, bf.oneOf)
	anyOf := mergeFlatParams(af.anyOf, bf.anyOf)
	allOf := mergeFlatParams(af.allOf, bf.allOf)
	not := SchemaRef(af.not, bf.not)

	return &openapi3.Schema{
		Extensions:           nil,
		OneOf:                oneOf,
		AnyOf:                anyOf,
		AllOf:                allOf,
		Not:                  not,
		Type:                 "",
		Title:                mergeString(a.Title, b.Title),
		Format:               mergeString(a.Format, b.Format),
		Description:          mergeString(a.Description, b.Description),
		Enum:                 nil,
		Default:              nil,
		Example:              nil,
		ExternalDocs:         nil,
		UniqueItems:          false,
		ExclusiveMin:         false,
		ExclusiveMax:         false,
		Nullable:             false,
		ReadOnly:             false,
		WriteOnly:            false,
		AllowEmptyValue:      false,
		Deprecated:           false,
		XML:                  nil,
		Min:                  nil,
		Max:                  nil,
		MultipleOf:           nil,
		MinLength:            0,
		MaxLength:            nil,
		Pattern:              "",
		MinItems:             0,
		MaxItems:             nil,
		Items:                nil,
		Required:             nil,
		Properties:           nil,
		MinProps:             0,
		MaxProps:             nil,
		AdditionalProperties: openapi3.AdditionalProperties{},
		Discriminator:        nil,
	}
}

func mergeFlatParams(a, b map[string]*openapi3.SchemaRef) []*openapi3.SchemaRef {
	res := make([]*openapi3.SchemaRef, 0, max(len(a), len(b)))
	keys := make(map[string]struct{}, max(len(a), len(b)))
	for k := range a {
		keys[k] = struct{}{}
	}
	for k := range b {
		keys[k] = struct{}{}
	}
	for k := range keys {
		var ra *openapi3.SchemaRef
		if r, in := a[k]; in {
			ra = r
		}

		var rb *openapi3.SchemaRef
		if r, in := b[k]; in {
			rb = r
		}

		res = append(res, SchemaRef(ra, rb))
	}

	return res
}

type flat struct {
	oneOf map[string]*openapi3.SchemaRef
	anyOf map[string]*openapi3.SchemaRef
	allOf map[string]*openapi3.SchemaRef
	not   *openapi3.SchemaRef
}

func flattenTypes(s *openapi3.Schema) flat {
	if s.Type != "" {
		return flat{
			oneOf: map[string]*openapi3.SchemaRef{s.Type: s.NewRef()},
			anyOf: map[string]*openapi3.SchemaRef{},
			allOf: map[string]*openapi3.SchemaRef{},
			not:   nil,
		}
	} else {
		f := flat{
			oneOf: map[string]*openapi3.SchemaRef{s.Type: s.NewRef()},
			anyOf: map[string]*openapi3.SchemaRef{},
			allOf: map[string]*openapi3.SchemaRef{},
			not:   nil,
		}

		for _, v := range s.OneOf {
			f.oneOf[v.Value.Type] = v
		}
		for _, v := range s.AnyOf {
			f.anyOf[v.Value.Type] = v
		}
		for _, v := range s.AllOf {
			f.allOf[v.Value.Type] = v
		}
		f.not = s.Not

		return f
	}
}

func mergeString(a, b string) string {
	if a == "" && b == "" {
		return ""
	}
	if a == "" && b != "" {
		return b
	}
	if a != "" && b == "" {
		return a
	}
	if len(b) > len(a) {
		return b
	}
	return a
}
