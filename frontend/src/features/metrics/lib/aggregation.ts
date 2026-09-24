import type { MetricAggregation, MetricAttribution, MetricType } from '../types';

export const MIN_WINDOW_DAYS = 1;
export const MAX_WINDOW_DAYS = 30;
export const MAX_EVENT_TYPE_LENGTH = 128;
export const MAX_FIELD_LENGTH = 128;

export interface AggregationDraft {
  eventType: string;
  field: string;
  levelRaw: string;
  numeratorEventType: string;
  numeratorField: string;
  denominatorEventType: string;
  denominatorField: string;
}

export function emptyAggregationDraft(): AggregationDraft {
  return {
    eventType: '',
    field: '',
    levelRaw: '',
    numeratorEventType: '',
    numeratorField: '',
    denominatorEventType: '',
    denominatorField: '',
  };
}

export function aggregationDraftFromMetric(aggregation: MetricAggregation): AggregationDraft {
  return {
    eventType: aggregation.event_type ?? '',
    field: aggregation.field ?? '',
    levelRaw: aggregation.level !== undefined ? String(aggregation.level) : '',
    numeratorEventType: aggregation.numerator?.event_type ?? '',
    numeratorField: aggregation.numerator?.field ?? '',
    denominatorEventType: aggregation.denominator?.event_type ?? '',
    denominatorField: aggregation.denominator?.field ?? '',
  };
}

export type AggregationError =
  | 'eventType'
  | 'field'
  | 'level'
  | 'numerator'
  | 'denominator'
  | null;

function validName(value: string, max: number): boolean {
  const trimmed = value.trim();
  return trimmed.length > 0 && trimmed.length <= max;
}

function validOptionalName(value: string, max: number): boolean {
  const trimmed = value.trim();
  return trimmed.length === 0 || trimmed.length <= max;
}

export function buildAggregation(
  metricType: MetricType,
  draft: AggregationDraft,
): { aggregation: MetricAggregation | null; error: AggregationError } {
  switch (metricType) {
    case 'count':
    case 'unique_count': {
      if (!validName(draft.eventType, MAX_EVENT_TYPE_LENGTH)) {
        return { aggregation: null, error: 'eventType' };
      }
      return { aggregation: { event_type: draft.eventType.trim() }, error: null };
    }
    case 'sum':
    case 'average': {
      if (!validName(draft.eventType, MAX_EVENT_TYPE_LENGTH)) {
        return { aggregation: null, error: 'eventType' };
      }
      if (!validName(draft.field, MAX_FIELD_LENGTH)) {
        return { aggregation: null, error: 'field' };
      }
      return {
        aggregation: { event_type: draft.eventType.trim(), field: draft.field.trim() },
        error: null,
      };
    }
    case 'percentile': {
      if (!validName(draft.eventType, MAX_EVENT_TYPE_LENGTH)) {
        return { aggregation: null, error: 'eventType' };
      }
      if (!validName(draft.field, MAX_FIELD_LENGTH)) {
        return { aggregation: null, error: 'field' };
      }
      const level = Number(draft.levelRaw);
      if (!Number.isFinite(level) || level <= 0 || level >= 1) {
        return { aggregation: null, error: 'level' };
      }
      return {
        aggregation: {
          event_type: draft.eventType.trim(),
          field: draft.field.trim(),
          level,
        },
        error: null,
      };
    }
    case 'ratio': {
      if (!validName(draft.numeratorEventType, MAX_EVENT_TYPE_LENGTH)) {
        return { aggregation: null, error: 'numerator' };
      }
      if (!validOptionalName(draft.numeratorField, MAX_FIELD_LENGTH)) {
        return { aggregation: null, error: 'numerator' };
      }
      if (!validName(draft.denominatorEventType, MAX_EVENT_TYPE_LENGTH)) {
        return { aggregation: null, error: 'denominator' };
      }
      if (!validOptionalName(draft.denominatorField, MAX_FIELD_LENGTH)) {
        return { aggregation: null, error: 'denominator' };
      }
      const numeratorField = draft.numeratorField.trim();
      const denominatorField = draft.denominatorField.trim();
      return {
        aggregation: {
          numerator: {
            event_type: draft.numeratorEventType.trim(),
            ...(numeratorField.length > 0 ? { field: numeratorField } : {}),
          },
          denominator: {
            event_type: draft.denominatorEventType.trim(),
            ...(denominatorField.length > 0 ? { field: denominatorField } : {}),
          },
        },
        error: null,
      };
    }
  }
}

export interface AttributionDraft {
  requireExposure: boolean;
  windowDaysRaw: string;
  fallback: MetricAttribution['fallback'] | '';
}

export function attributionDraftFromMetric(attribution: MetricAttribution): AttributionDraft {
  return {
    requireExposure: attribution.require_exposure,
    windowDaysRaw: String(attribution.window_days),
    fallback: attribution.fallback,
  };
}

export function buildAttribution(draft: AttributionDraft): {
  attribution: MetricAttribution | null;
  error: 'windowDays' | 'fallback' | null;
} {
  const windowDays = Number(draft.windowDaysRaw);
  if (!Number.isInteger(windowDays) || windowDays < MIN_WINDOW_DAYS || windowDays > MAX_WINDOW_DAYS) {
    return { attribution: null, error: 'windowDays' };
  }
  if (draft.fallback !== 'none' && draft.fallback !== 'subject') {
    return { attribution: null, error: 'fallback' };
  }
  return {
    attribution: {
      require_exposure: draft.requireExposure,
      window_days: windowDays,
      fallback: draft.fallback,
    },
    error: null,
  };
}

export function metricTypeBadgeColor(metricType: MetricType): string {
  switch (metricType) {
    case 'count':
      return 'cyan';
    case 'sum':
      return 'blue';
    case 'unique_count':
      return 'violet';
    case 'ratio':
      return 'grape';
    case 'percentile':
      return 'orange';
    case 'average':
      return 'teal';
  }
}

export function summarizeAggregation(metricType: MetricType, aggregation: MetricAggregation): string {
  switch (metricType) {
    case 'count':
    case 'unique_count':
      return aggregation.event_type ?? '';
    case 'sum':
    case 'average':
      return `${aggregation.event_type ?? ''}.${aggregation.field ?? ''}`;
    case 'percentile': {
      const level = aggregation.level === undefined ? '' : String(aggregation.level);
      return `${aggregation.event_type ?? ''}.${aggregation.field ?? ''} p${level}`;
    }
    case 'ratio':
      return `${aggregation.numerator?.event_type ?? ''}/${aggregation.denominator?.event_type ?? ''}`;
  }
}
