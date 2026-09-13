package metrics

import (
	"strings"
	"testing"
)

func strP(s string) *string  { return &s }
func flP(f float64) *float64 { return &f }

func TestAggregationValidate(t *testing.T) {
	cases := []struct {
		name    string
		typ     MetricType
		agg     Aggregation
		wantErr bool
	}{
		{"count ok", MetricTypeCount, Aggregation{EventType: strP("purchase_completed")}, false},
		{"count missing event", MetricTypeCount, Aggregation{}, true},
		{"count rejects field", MetricTypeCount, Aggregation{EventType: strP("e"), Field: strP("f")}, true},
		{"unique ok", MetricTypeUniqueCount, Aggregation{EventType: strP("e")}, false},
		{"sum ok", MetricTypeSum, Aggregation{EventType: strP("e"), Field: strP("order_value")}, false},
		{"sum missing field", MetricTypeSum, Aggregation{EventType: strP("e")}, true},
		{"average ok", MetricTypeAverage, Aggregation{EventType: strP("e"), Field: strP("v")}, false},
		{"percentile ok", MetricTypePercentile, Aggregation{EventType: strP("e"), Field: strP("v"), Level: flP(0.95)}, false},
		{"percentile missing level", MetricTypePercentile, Aggregation{EventType: strP("e"), Field: strP("v")}, true},
		{"percentile level boundary", MetricTypePercentile, Aggregation{EventType: strP("e"), Field: strP("v"), Level: flP(1)}, true},
		{
			"ratio ok",
			MetricTypeRatio,
			Aggregation{Numerator: &EventRef{EventType: "a"}, Denominator: &EventRef{EventType: "b"}},
			false,
		},
		{
			"ratio with field legs",
			MetricTypeRatio,
			Aggregation{Numerator: &EventRef{EventType: "a", Field: strP("x")}, Denominator: &EventRef{EventType: "b"}},
			false,
		},
		{"ratio missing denominator", MetricTypeRatio, Aggregation{Numerator: &EventRef{EventType: "a"}}, true},
		{"ratio rejects top-level event", MetricTypeRatio, Aggregation{EventType: strP("a"), Numerator: &EventRef{EventType: "a"}, Denominator: &EventRef{EventType: "b"}}, true},
		{"unknown type", MetricType("nope"), Aggregation{EventType: strP("a")}, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.agg.Validate(tc.typ)
			if tc.wantErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Errorf("expected nil, got %v", err)
			}
		})
	}
}

func TestAttributionValidate(t *testing.T) {
	if err := (Attribution{RequireExposure: true, WindowDays: 7, Fallback: AttributionFallbackSubject}).Validate(); err != nil {
		t.Errorf("valid attribution rejected: %v", err)
	}

	for name, a := range map[string]Attribution{
		"window zero":      {RequireExposure: true, WindowDays: 0, Fallback: "subject"},
		"window too big":   {RequireExposure: false, WindowDays: 31, Fallback: "none"},
		"bad fallback":     {RequireExposure: true, WindowDays: 7, Fallback: "last_touch"},
		"missing fallback": {RequireExposure: true, WindowDays: 7},
	} {
		if err := a.Validate(); err == nil {
			t.Errorf("%s: expected error, got nil", name)
		} else if !strings.Contains(err.Error(), "attribution.") {
			t.Errorf("%s: error should mention attribution, got %v", name, err)
		}
	}
}
