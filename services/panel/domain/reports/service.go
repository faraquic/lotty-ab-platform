package reports

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	experimentsdomain "github.com/faraquic/lotty-ab-platform/services/panel/domain/experiments"
	"go.uber.org/zap"
)

var (
	ErrInvalidInterval  = errors.New("invalid interval")
	ErrInvalidDateRange = errors.New("end must be after start")
	ErrNoMetrics        = errors.New("at least one metric required")
)

type ExperimentProvider interface {
	GetByID(ctx context.Context, id string) (experimentsdomain.ExperimentResponse, error)
}

type ReportRepo interface {
	GetTimeSeries(ctx context.Context, experimentID uuid.UUID, variantID uuid.UUID, eventType string, start, end time.Time, interval string) ([]TimeSeriesPoint, error)
	GetVariantMetrics(ctx context.Context, experimentID uuid.UUID, eventType string, start, end time.Time) ([]VariantMetric, error)
	GetSampleSize(ctx context.Context, experimentID uuid.UUID, start, end time.Time) ([]VariantSample, error)
	GetAttributionLag(ctx context.Context, experimentID uuid.UUID, start, end time.Time) (float64, error)
	GetEventCounts(ctx context.Context, experimentID uuid.UUID, start, end time.Time) (map[string]int64, error)
}

type Service struct {
	repo        ReportRepo
	experiments ExperimentProvider
	log         *zap.Logger
}

func NewService(repo ReportRepo, experiments ExperimentProvider, log *zap.Logger) *Service {
	return &Service{repo: repo, experiments: experiments, log: log.Named("reports")}
}

func (s *Service) GetReport(ctx context.Context, experimentID uuid.UUID, query ReportQuery) (*Report, error) {
	if err := s.validateQuery(query); err != nil {
		return nil, err
	}

	exp, err := s.experiments.GetByID(ctx, experimentID.String())
	if err != nil {
		return nil, err
	}

	variantNames := make(map[string]string)
	for _, v := range exp.Variants {
		variantNames[v.ID] = v.Name
	}

	report := &Report{
		ExperimentID:   experimentID.String(),
		ExperimentName: exp.Name,
		Start:          query.Start,
		End:            query.End,
		Interval:       query.Interval,
	}

	for _, metricKey := range query.Metrics {
		mr, err := s.buildMetricReport(ctx, experimentID, metricKey, variantNames, query)
		if err != nil {
			s.log.Warn("metric report failed",
				zap.String("metric", metricKey),
				zap.Error(err),
			)
			continue
		}
		report.Metrics = append(report.Metrics, *mr)
	}

	sampleSize, err := s.repo.GetSampleSize(ctx, experimentID, query.Start, query.End)
	if err != nil {
		s.log.Warn("sample size query failed", zap.Error(err))
	} else {
		for i := range sampleSize {
			if name, ok := variantNames[sampleSize[i].VariantID]; ok {
				sampleSize[i].VariantName = name
			}
		}
		report.SampleSize = sampleSize
	}

	if len(sampleSize) >= 2 {
		report.AllocationSkew = s.calcAllocationSkew(sampleSize)
	}

	return report, nil
}

func (s *Service) buildMetricReport(ctx context.Context, experimentID uuid.UUID, metricKey string, variantNames map[string]string, query ReportQuery) (*MetricReport, error) {
	eventType := metricKey

	byVariant, err := s.repo.GetVariantMetrics(ctx, experimentID, eventType, query.Start, query.End)
	if err != nil {
		return nil, err
	}
	for i := range byVariant {
		if name, ok := variantNames[byVariant[i].VariantID]; ok {
			byVariant[i].VariantName = name
		}
	}

	timeSeries, err := s.repo.GetTimeSeries(ctx, experimentID, uuid.Nil, eventType, query.Start, query.End, query.Interval)
	if err != nil {
		return nil, err
	}

	return &MetricReport{
		Key:        metricKey,
		Type:       eventType,
		TimeSeries: timeSeries,
		ByVariant:  byVariant,
	}, nil
}

func (s *Service) calcAllocationSkew(samples []VariantSample) *AllocationSkew {
	if len(samples) < 2 {
		return nil
	}

	var total int64
	for _, s := range samples {
		total += s.Count
	}
	if total == 0 {
		return nil
	}

	expected := 1.0 / float64(len(samples))

	var maxDeviation float64
	for _, s := range samples {
		actual := float64(s.Count) / float64(total)
		deviation := actual - expected
		if deviation < 0 {
			deviation = -deviation
		}
		if deviation > maxDeviation {
			maxDeviation = deviation
		}
	}

	return &AllocationSkew{
		Expected: expected,
		Actual:   maxDeviation + expected,
		Skew:     maxDeviation,
	}
}

func (s *Service) GetDataQuality(ctx context.Context, experimentID uuid.UUID, start, end time.Time) (*DataQuality, error) {
	lag, err := s.repo.GetAttributionLag(ctx, experimentID, start, end)
	if err != nil {
		s.log.Warn("attribution lag query failed", zap.Error(err))
	}

	counts, err := s.repo.GetEventCounts(ctx, experimentID, start, end)
	if err != nil {
		s.log.Warn("event counts query failed", zap.Error(err))
	}

	var total int64
	for _, cnt := range counts {
		total += cnt
	}

	return &DataQuality{
		RejectedRate:       0,
		DuplicateRate:      0,
		PendingAttribution: 0,
		AttributionLagSec:  lag,
	}, nil
}

func (s *Service) validateQuery(q ReportQuery) error {
	if q.End.Before(q.Start) || q.End.Equal(q.Start) {
		return ErrInvalidDateRange
	}
	if q.Interval != "hour" && q.Interval != "day" && q.Interval != "week" && q.Interval != "month" {
		return ErrInvalidInterval
	}
	if len(q.Metrics) == 0 {
		return ErrNoMetrics
	}
	maxRange := time.Hour * 24 * 92
	if q.End.Sub(q.Start) > maxRange {
		return fmt.Errorf("%w: max range 92 days", ErrInvalidDateRange)
	}
	return nil
}
