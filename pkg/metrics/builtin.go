package metrics

var BuiltinMetrics = map[string]Node{
	"exposure_count": {
		Type: NodeFunc,
		Name: "countIf",
		Children: []Node{
			{Type: NodeField, Field: "event_type"},
			{Type: NodeNumber, Value: 0},
		},
	},
	"conversion_count": {
		Type: NodeFunc,
		Name: "countIf",
		Children: []Node{
			{Type: NodeField, Field: "event_type"},
			{Type: NodeNumber, Value: 1},
		},
	},
	"error_count": {
		Type: NodeFunc,
		Name: "countIf",
		Children: []Node{
			{Type: NodeField, Field: "event_type"},
			{Type: NodeNumber, Value: 2},
		},
	},
	"avg_latency": {
		Type: NodeFunc,
		Name: "avg",
		Children: []Node{
			{Type: NodeField, Field: "latency"},
		},
	},
	"p95_latency": {
		Type: NodeFunc,
		Name: "quantile",
		Children: []Node{
			{Type: NodeNumber, Value: 0.95},
			{Type: NodeField, Field: "latency"},
		},
	},
	"p99_latency": {
		Type: NodeFunc,
		Name: "quantile",
		Children: []Node{
			{Type: NodeNumber, Value: 0.99},
			{Type: NodeField, Field: "latency"},
		},
	},
	"conversion_rate": {
		Type: NodeBinary,
		Op:   "/",
		Children: []Node{
			{
				Type: NodeFunc,
				Name: "countIf",
				Children: []Node{
					{Type: NodeField, Field: "event_type"},
					{Type: NodeNumber, Value: 1},
				},
			},
			{
				Type: NodeFunc,
				Name: "countIf",
				Children: []Node{
					{Type: NodeField, Field: "event_type"},
					{Type: NodeNumber, Value: 0},
				},
			},
		},
	},
	"error_rate": {
		Type: NodeBinary,
		Op:   "/",
		Children: []Node{
			{
				Type: NodeFunc,
				Name: "countIf",
				Children: []Node{
					{Type: NodeField, Field: "event_type"},
					{Type: NodeNumber, Value: 2},
				},
			},
			{
				Type: NodeFunc,
				Name: "count",
			},
		},
	},
}

func ExpandBuiltins(node Node) Node {
	if node.Type == NodeField {
		if builtin, ok := BuiltinMetrics[node.Field]; ok {
			return builtin
		}
		return node
	}

	if node.Type == NodeFunc {
		if builtin, ok := BuiltinMetrics[node.Name]; ok && len(node.Children) == 0 {
			return builtin
		}
		for i, child := range node.Children {
			node.Children[i] = ExpandBuiltins(child)
		}
		return node
	}

	if node.Type == NodeBinary {
		for i, child := range node.Children {
			node.Children[i] = ExpandBuiltins(child)
		}
		return node
	}

	return node
}
