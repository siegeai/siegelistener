package infer

type EventLog struct {
	PrimaryID     map[string]any `json:"primaryID"`
	IDs           map[string]any `json:"ids"`
	OperationName string         `json:"operationName"`
	Timestamp     int64          `json:"timestamp"`
	Data          map[string]any `json:"data"`
	API           map[string]any `json:"api"`
}

func NewEventLog() *EventLog {
	return &EventLog{
		PrimaryID:     make(map[string]any),
		IDs:           make(map[string]any),
		OperationName: "",
		Timestamp:     0,
		Data:          make(map[string]any),
		API:           make(map[string]any),
	}
}
