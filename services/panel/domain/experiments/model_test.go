package experiments

import (
	"testing"

	"github.com/goccy/go-json"
)

func TestValidateTransition(t *testing.T) {
	valid := [][2]Status{
		{StatusDraft, StatusReview},
		{StatusReview, StatusApproved},
		{StatusReview, StatusDraft},
		{StatusApproved, StatusRunning},
		{StatusRunning, StatusPaused},
		{StatusRunning, StatusCompleted},
		{StatusPaused, StatusRunning},
		{StatusPaused, StatusCompleted},
		{StatusCompleted, StatusArchived},
	}
	for _, tc := range valid {
		if err := ValidateTransition(tc[0], tc[1]); err != nil {
			t.Errorf("%s -> %s: expected nil, got %v", tc[0], tc[1], err)
		}
	}

	invalid := [][2]Status{
		{StatusDraft, StatusRunning},
		{StatusDraft, StatusCompleted},
		{StatusReview, StatusRunning},
		{StatusApproved, StatusPaused},
		{StatusRunning, StatusDraft},
		{StatusRunning, StatusArchived},
		{StatusPaused, StatusDraft},
		{StatusCompleted, StatusRunning},
		{StatusArchived, StatusDraft},
	}
	for _, tc := range invalid {
		if err := ValidateTransition(tc[0], tc[1]); err == nil {
			t.Errorf("%s -> %s: expected error, got nil", tc[0], tc[1])
		}
	}
}

func TestValidateVariants(t *testing.T) {
	val := func(s string) VariantValue {
		var v VariantValue
		if err := json.Unmarshal([]byte(s), &v); err != nil {
			t.Fatalf("bad fixture: %v", err)
		}
		return v
	}

	valid := []VariantInput{
		{Name: "control", Value: val(`"a"`), WeightBP: 5000, IsControl: true},
		{Name: "treatment", Value: val(`"b"`), WeightBP: 5000},
	}
	if err := ValidateVariants(valid, 10000); err != nil {
		t.Errorf("valid variants rejected: %v", err)
	}

	cases := map[string][]VariantInput{
		"single variant": {
			{Name: "only", Value: val(`"a"`), WeightBP: 10000, IsControl: true},
		},
		"no control": {
			{Name: "a", Value: val(`"a"`), WeightBP: 5000},
			{Name: "b", Value: val(`"b"`), WeightBP: 5000},
		},
		"two controls": {
			{Name: "a", Value: val(`"a"`), WeightBP: 5000, IsControl: true},
			{Name: "b", Value: val(`"b"`), WeightBP: 5000, IsControl: true},
		},
		"weights mismatch": {
			{Name: "a", Value: val(`"a"`), WeightBP: 4000, IsControl: true},
			{Name: "b", Value: val(`"b"`), WeightBP: 5000},
		},
		"duplicate names": {
			{Name: "a", Value: val(`"a"`), WeightBP: 5000, IsControl: true},
			{Name: "a", Value: val(`"b"`), WeightBP: 5000},
		},
		"zero weight": {
			{Name: "a", Value: val(`"a"`), WeightBP: 10000, IsControl: true},
			{Name: "b", Value: val(`"b"`), WeightBP: 0},
		},
	}
	for name, inputs := range cases {
		if err := ValidateVariants(inputs, 10000); err == nil {
			t.Errorf("%s: expected error, got nil", name)
		}
	}
}

func TestTargetingValid(t *testing.T) {
	var nilT *Targeting
	if nilT != nil {
		t.Fatal("unreachable")
	}

	raw := Targeting(`{"country": "DE"}`)
	if !raw.Valid() {
		t.Error("object targeting should be valid")
	}

	for _, bad := range []string{"", "not-json", `[1,2]`, `"str"`} {
		if (Targeting(bad)).Valid() {
			t.Errorf("%q: expected invalid", bad)
		}
	}
}

func TestNormalizeTargeting(t *testing.T) {
	if normalizeTargeting(nil) != nil {
		t.Error("nil should stay nil")
	}
	for _, empty := range []string{"", "  ", "null", "{}", "  {}  "} {
		raw := Targeting(empty)
		if got := normalizeTargeting(&raw); got != nil {
			t.Errorf("%q: expected nil, got %q", empty, string(*got))
		}
	}
	raw := Targeting(`{"country": "DE"}`)
	if got := normalizeTargeting(&raw); got == nil || string(*got) != `{"country": "DE"}` {
		t.Errorf("non-empty object should pass through: %v", got)
	}
}
