package apispec

type Spec struct {
	Paths map[string]Path
}

type Path struct {
	Summary     string
	Description string

	Get     *Operation
	Put     *Operation
	Post    *Operation
	Delete  *Operation
	Options *Operation
	Head    *Operation
	Patch   *Operation
	Trace   *Operation

	Servers    []string
	Parameters []string
}

type EndpointVerb int

const (
	EndpointVerbGet  EndpointVerb = 0
	EndpointVerbPost EndpointVerb = 1
)

type Operation struct {
	Tags         []string
	Summary      string
	Description  string
	ExternalDocs string
	OperationID  string
	Parameters   []string
	RequestBody  RequestBody
	Responses    []Schema
	Callbacks    []string
	Deprecated   bool
	Security     bool
	Servers      []string
}

type RequestBody struct {
	Description string
	Content     map[string]MediaType
	Required    bool
}

type MediaType struct {
	Schema   Schema
	Example  any
	Examples map[string]any
	Encoding map[string]any
}
