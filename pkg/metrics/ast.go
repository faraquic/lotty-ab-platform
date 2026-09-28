package metrics

type NodeType string

const (
	NodeFunc   NodeType = "func"
	NodeField  NodeType = "field"
	NodeBinary NodeType = "binary"
	NodeNumber NodeType = "number"
)

type Node struct {
	Type     NodeType `json:"type"`
	Op       string   `json:"op,omitempty"`
	Name     string   `json:"name,omitempty"`
	Field    string   `json:"field,omitempty"`
	Value    float64  `json:"value,omitempty"`
	Level    float64  `json:"level,omitempty"`
	Children []Node   `json:"children,omitempty"`
}
