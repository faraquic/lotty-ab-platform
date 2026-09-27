package experiments

import (
	"errors"
	"testing"

	flagsdomain "github.com/faraquic/lotty-ab-platform/services/panel/domain/flags"
	"github.com/goccy/go-json"
)

func TestValidateTransition(t *testing.T) {
	statuses := []Status{
		StatusDraft,
		StatusReview,
		StatusApproved,
		StatusRunning,
		StatusPaused,
		StatusCompleted,
		StatusArchived,
		StatusRejected,
	}
	allowed := map[[2]Status]struct{}{
		{StatusDraft, StatusReview}:       {},
		{StatusReview, StatusApproved}:    {},
		{StatusReview, StatusDraft}:       {},
		{StatusReview, StatusRejected}:    {},
		{StatusApproved, StatusRunning}:   {},
		{StatusRunning, StatusPaused}:     {},
		{StatusRunning, StatusCompleted}:  {},
		{StatusPaused, StatusRunning}:     {},
		{StatusPaused, StatusCompleted}:   {},
		{StatusCompleted, StatusArchived}: {},
	}

	for _, from := range statuses {
		for _, to := range statuses {
			_, wantAllowed := allowed[[2]Status{from, to}]
			err := ValidateTransition(from, to)
			if wantAllowed && err != nil {
				t.Errorf("%s -> %s: expected allowed transition, got %v", from, to, err)
			} else if !wantAllowed && !errors.Is(err, ErrInvalidTransition) {
				t.Errorf("%s -> %s: expected ErrInvalidTransition, got %v", from, to, err)
			}
		}
	}

	for _, tc := range [][2]Status{{"unknown", StatusDraft}, {StatusDraft, "unknown"}} {
		if err := ValidateTransition(tc[0], tc[1]); !errors.Is(err, ErrInvalidTransition) {
			t.Errorf("%s -> %s: expected ErrInvalidTransition, got %v", tc[0], tc[1], err)
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
	if err := ValidateVariants(valid, 10000, flagsdomain.TypeFlagString); err != nil {
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
		if err := ValidateVariants(inputs, 10000, flagsdomain.TypeFlagString); err == nil {
			t.Errorf("%s: expected error, got nil", name)
		}
	}
}

func TestValidateVariantsFlagType(t *testing.T) {
	val := func(s string) VariantValue {
		var v VariantValue
		if err := json.Unmarshal([]byte(s), &v); err != nil {
			t.Fatalf("bad fixture: %v", err)
		}
		return v
	}

	stringVariants := []VariantInput{
		{Name: "control", Value: val(`123`), WeightBP: 5000, IsControl: true},
		{Name: "treatment", Value: val(`"b"`), WeightBP: 5000},
	}
	if err := ValidateVariants(stringVariants, 10000, flagsdomain.TypeFlagString); err == nil {
		t.Error("number value should be rejected for string flag")
	}

	boolVariants := []VariantInput{
		{Name: "control", Value: val(`false`), WeightBP: 5000, IsControl: true},
		{Name: "treatment", Value: val(`true`), WeightBP: 5000},
	}
	if err := ValidateVariants(boolVariants, 10000, flagsdomain.TypeFlagBool); err != nil {
		t.Errorf("bool values should be accepted for bool flag: %v", err)
	}
}

func TestParseTargeting(t *testing.T) {
	if tgt, err := ParseTargeting(""); err != nil || tgt != nil {
		t.Errorf("empty targeting should return nil: %v %v", tgt, err)
	}
	if tgt, err := ParseTargeting("   "); err != nil || tgt != nil {
		t.Errorf("blank targeting should return nil: %v %v", tgt, err)
	}
	tgt, err := ParseTargeting(`country == "DE" AND NOT (plan IN ["free"])`)
	if err != nil {
		t.Fatalf("valid DSL rejected: %v", err)
	}
	if tgt == nil || !tgt.Valid() {
		t.Fatalf("expected valid canonical targeting, got %v", tgt)
	}
	if _, err := ParseTargeting(`country = "DE"`); err != nil {
		t.Errorf("single '=' should be accepted as '==': %v", err)
	}
	if _, err := ParseTargeting(`country ~`); err == nil {
		t.Error("invalid DSL should be rejected")
	}
}

func TestFormatTargeting(t *testing.T) {
	tgt, err := ParseTargeting(`country == "DE"`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got := FormatTargeting(tgt); got != `country == "DE"` {
		t.Errorf("unexpected formatted DSL: %q", got)
	}
	if got := FormatTargeting(nil); got != "" {
		t.Errorf("nil targeting should format to empty, got %q", got)
	}
}
