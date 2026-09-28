package reports

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	experimentsdomain "github.com/faraquic/lotty-ab-platform/services/panel/domain/experiments"
	"go.uber.org/zap"
)

type fakeRepo struct{}

func (f *fakeRepo) GetTimeSeries(_ context.Context, _ uuid.UUID, _ uuid.UUID, _ string, _, _ time.Time, _ string) ([]TimeSeriesPoint, error) {
	return nil, nil
}

func (f *fakeRepo) GetVariantMetrics(_ context.Context, _ uuid.UUID, _ string, _, _ time.Time) ([]VariantMetric, error) {
	return nil, nil
}

func (f *fakeRepo) GetSampleSize(_ context.Context, _ uuid.UUID, _, _ time.Time) ([]VariantSample, error) {
	return []VariantSample{
		{VariantID: uuid.New().String(), VariantName: "control", Count: 100},
		{VariantID: uuid.New().String(), VariantName: "treatment", Count: 100},
	}, nil
}

func (f *fakeRepo) GetAttributionLag(_ context.Context, _ uuid.UUID, _, _ time.Time) (float64, error) {
	return 2.5, nil
}

func (f *fakeRepo) GetEventCounts(_ context.Context, _ uuid.UUID, _, _ time.Time) (map[string]int64, error) {
	return map[string]int64{"conversion": 50, "error": 5}, nil
}

type fakeExperiments struct{}

func (f *fakeExperiments) GetByID(_ context.Context, _ string) (experimentsdomain.ExperimentResponse, error) {
	return experimentsdomain.ExperimentResponse{
		Name: "Test Experiment",
		Variants: []experimentsdomain.VariantResponse{
			{ID: uuid.New().String(), Name: "control", IsControl: true},
			{ID: uuid.New().String(), Name: "treatment"},
		},
	}, nil
}

func TestValidateQuery_Valid(t *testing.T) {
	svc := NewService(&fakeRepo{}, &fakeExperiments{}, zap.NewNop())
	now := time.Now()

	err := svc.validateQuery(ReportQuery{
		Start:    now.Add(-7 * 24 * time.Hour),
		End:      now,
		Interval: "day",
		Metrics:  []string{"conversion"},
	})
	if err != nil {
		t.Errorf("validateQuery = %v, want nil", err)
	}
}

func TestValidateQuery_InvalidInterval(t *testing.T) {
	svc := NewService(&fakeRepo{}, &fakeExperiments{}, zap.NewNop())
	now := time.Now()

	err := svc.validateQuery(ReportQuery{
		Start:    now.Add(-7 * 24 * time.Hour),
		End:      now,
		Interval: "minute",
		Metrics:  []string{"conversion"},
	})
	if !errors.Is(err, ErrInvalidInterval) {
		t.Errorf("validateQuery = %v, want ErrInvalidInterval", err)
	}
}

func TestValidateQuery_EndBeforeStart(t *testing.T) {
	svc := NewService(&fakeRepo{}, &fakeExperiments{}, zap.NewNop())
	now := time.Now()

	err := svc.validateQuery(ReportQuery{
		Start:    now,
		End:      now.Add(-24 * time.Hour),
		Interval: "day",
		Metrics:  []string{"conversion"},
	})
	if !errors.Is(err, ErrInvalidDateRange) {
		t.Errorf("validateQuery = %v, want ErrInvalidDateRange", err)
	}
}

func TestValidateQuery_NoMetrics(t *testing.T) {
	svc := NewService(&fakeRepo{}, &fakeExperiments{}, zap.NewNop())
	now := time.Now()

	err := svc.validateQuery(ReportQuery{
		Start:    now.Add(-7 * 24 * time.Hour),
		End:      now,
		Interval: "day",
		Metrics:  []string{},
	})
	if !errors.Is(err, ErrNoMetrics) {
		t.Errorf("validateQuery = %v, want ErrNoMetrics", err)
	}
}

func TestValidateQuery_RangeTooLarge(t *testing.T) {
	svc := NewService(&fakeRepo{}, &fakeExperiments{}, zap.NewNop())
	now := time.Now()

	err := svc.validateQuery(ReportQuery{
		Start:    now.Add(-180 * 24 * time.Hour),
		End:      now,
		Interval: "day",
		Metrics:  []string{"conversion"},
	})
	if !errors.Is(err, ErrInvalidDateRange) {
		t.Errorf("validateQuery = %v, want ErrInvalidDateRange", err)
	}
}

func TestCalcAllocationSkew_Balanced(t *testing.T) {
	svc := NewService(&fakeRepo{}, &fakeExperiments{}, zap.NewNop())

	skew := svc.calcAllocationSkew([]VariantSample{
		{Count: 100},
		{Count: 100},
	})
	if skew == nil {
		t.Fatal("calcAllocationSkew = nil, want non-nil")
	}
	if skew.Skew > 0.01 {
		t.Errorf("skew = %f, want ~0 for balanced", skew.Skew)
	}
}

func TestCalcAllocationSkew_Skewed(t *testing.T) {
	svc := NewService(&fakeRepo{}, &fakeExperiments{}, zap.NewNop())

	skew := svc.calcAllocationSkew([]VariantSample{
		{Count: 90},
		{Count: 10},
	})
	if skew == nil {
		t.Fatal("calcAllocationSkew = nil, want non-nil")
	}
	if skew.Skew < 0.3 {
		t.Errorf("skew = %f, want > 0.3 for skewed", skew.Skew)
	}
}

func TestCalcAllocationSkew_SingleVariant(t *testing.T) {
	svc := NewService(&fakeRepo{}, &fakeExperiments{}, zap.NewNop())

	skew := svc.calcAllocationSkew([]VariantSample{
		{Count: 100},
	})
	if skew != nil {
		t.Errorf("calcAllocationSkew = %+v, want nil for single variant", skew)
	}
}
