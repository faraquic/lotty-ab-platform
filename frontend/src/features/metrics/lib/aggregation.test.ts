import { describe, expect, it } from 'vitest';
import {
  attributionDraftFromMetric,
  buildAggregation,
  buildAttribution,
  metricTypeBadgeColor,
  summarizeAggregation,
} from './aggregation';

describe('aggregation builder', () => {
  it('builds count and unique_count from event type only', () => {
    const draft = {
      eventType: 'purchase',
      field: '',
      levelRaw: '',
      numeratorEventType: '',
      numeratorField: '',
      denominatorEventType: '',
      denominatorField: '',
    };
    expect(buildAggregation('count', draft)).toEqual({
      aggregation: { event_type: 'purchase' },
      error: null,
    });
    expect(buildAggregation('unique_count', { ...draft, eventType: '' }).error).toBe('eventType');
  });

  it('requires event type and field for sum and average', () => {
    const base = {
      eventType: 'purchase',
      field: 'revenue',
      levelRaw: '',
      numeratorEventType: '',
      numeratorField: '',
      denominatorEventType: '',
      denominatorField: '',
    };
    expect(buildAggregation('sum', base).aggregation).toEqual({
      event_type: 'purchase',
      field: 'revenue',
    });
    expect(buildAggregation('average', { ...base, field: '' }).error).toBe('field');
  });

  it('requires a level in (0, 1) for percentile', () => {
    const base = {
      eventType: 'purchase',
      field: 'revenue',
      levelRaw: '0.95',
      numeratorEventType: '',
      numeratorField: '',
      denominatorEventType: '',
      denominatorField: '',
    };
    expect(buildAggregation('percentile', base).aggregation).toEqual({
      event_type: 'purchase',
      field: 'revenue',
      level: 0.95,
    });
    expect(buildAggregation('percentile', { ...base, levelRaw: '1' }).error).toBe('level');
    expect(buildAggregation('percentile', { ...base, levelRaw: '' }).error).toBe('level');
  });

  it('builds ratio from numerator and denominator with optional fields', () => {
    const draft = {
      eventType: '',
      field: '',
      levelRaw: '',
      numeratorEventType: 'purchase',
      numeratorField: '',
      denominatorEventType: 'visit',
      denominatorField: 'page',
    };
    expect(buildAggregation('ratio', draft)).toEqual({
      aggregation: {
        numerator: { event_type: 'purchase' },
        denominator: { event_type: 'visit', field: 'page' },
      },
      error: null,
    });
    expect(
      buildAggregation('ratio', { ...draft, numeratorEventType: '' }).error,
    ).toBe('numerator');
    expect(
      buildAggregation('ratio', { ...draft, denominatorEventType: '' }).error,
    ).toBe('denominator');
  });
});

describe('attribution builder', () => {
  it('accepts window days 1..30 with a valid fallback', () => {
    expect(
      buildAttribution({ requireExposure: true, windowDaysRaw: '7', fallback: 'subject' }),
    ).toEqual({
      attribution: { require_exposure: true, window_days: 7, fallback: 'subject' },
      error: null,
    });
    expect(
      buildAttribution({ requireExposure: false, windowDaysRaw: '0', fallback: 'none' }).error,
    ).toBe('windowDays');
    expect(
      buildAttribution({ requireExposure: false, windowDaysRaw: '31', fallback: 'none' }).error,
    ).toBe('windowDays');
    expect(buildAttribution({ requireExposure: false, windowDaysRaw: '7', fallback: '' }).error).toBe(
      'fallback',
    );
  });

  it('round-trips backend attribution shapes', () => {
    expect(
      attributionDraftFromMetric({ require_exposure: true, window_days: 7, fallback: 'subject' }),
    ).toEqual({ requireExposure: true, windowDaysRaw: '7', fallback: 'subject' });
  });
});

describe('metric presentation helpers', () => {
  it('maps every metric type to a badge color', () => {
    for (const type of ['count', 'sum', 'unique_count', 'ratio', 'percentile', 'average'] as const) {
      expect(metricTypeBadgeColor(type).length).toBeGreaterThan(0);
    }
  });

  it('summarizes aggregations per type', () => {
    expect(summarizeAggregation('count', { event_type: 'purchase' })).toBe('purchase');
    expect(summarizeAggregation('sum', { event_type: 'purchase', field: 'revenue' })).toBe(
      'purchase.revenue',
    );
    expect(
      summarizeAggregation('ratio', {
        numerator: { event_type: 'purchase' },
        denominator: { event_type: 'visit' },
      }),
    ).toBe('purchase/visit');
  });
});
