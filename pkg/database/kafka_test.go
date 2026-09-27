package database

import (
	"reflect"
	"testing"
)

func TestParseBrokers(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{name: "single broker", input: "localhost:9092", want: []string{"localhost:9092"}},
		{name: "multiple brokers", input: "kafka-1:9092,kafka-2:9092", want: []string{"kafka-1:9092", "kafka-2:9092"}},
		{name: "trims spaces", input: " kafka-1:9092 , kafka-2:9092 ", want: []string{"kafka-1:9092", "kafka-2:9092"}},
		{name: "drops empties", input: "kafka-1:9092,,", want: []string{"kafka-1:9092"}},
		{name: "empty", input: "", want: nil},
		{name: "blank", input: "  ", want: nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ParseBrokers(tt.input); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ParseBrokers(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}
