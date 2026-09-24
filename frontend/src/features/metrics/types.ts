import { parseUser } from '@/features/users/types';
import type { User } from '@/features/users/types';

export const METRIC_TYPES = [
  'count',
  'sum',
  'unique_count',
  'ratio',
  'percentile',
  'average',
] as const;

export type MetricType = (typeof METRIC_TYPES)[number];

export function isMetricType(value: unknown): value is MetricType {
  return typeof value === 'string' && (METRIC_TYPES as readonly string[]).includes(value);
}

export const METRIC_STATUSES = ['active', 'archived'] as const;

export type MetricStatus = (typeof METRIC_STATUSES)[number];

export function isMetricStatus(value: unknown): value is MetricStatus {
  return typeof value === 'string' && (METRIC_STATUSES as readonly string[]).includes(value);
}

export const ATTRIBUTION_FALLBACKS = ['none', 'subject'] as const;

export type AttributionFallback = (typeof ATTRIBUTION_FALLBACKS)[number];

export function isAttributionFallback(value: unknown): value is AttributionFallback {
  return (
    typeof value === 'string' && (ATTRIBUTION_FALLBACKS as readonly string[]).includes(value)
  );
}

export interface EventRef {
  event_type: string;
  field?: string;
}

export interface MetricAggregation {
  event_type?: string;
  field?: string;
  level?: number;
  numerator?: EventRef;
  denominator?: EventRef;
}

export interface MetricAttribution {
  require_exposure: boolean;
  window_days: number;
  fallback: AttributionFallback;
}

export interface Metric {
  id: string;
  key: string;
  name: string;
  description: string | null;
  metric_type: MetricType;
  aggregation: MetricAggregation;
  attribution: MetricAttribution;
  is_builtin: boolean;
  status: MetricStatus;
  created_by: User | null;
  updated_by: User | null;
  created_at: string;
  updated_at: string;
}

export interface PaginationMeta {
  limit: number;
  offset: number;
  count: number;
  total: number;
  has_next: boolean;
}

export interface MetricListResponse {
  success: true;
  data: Metric[];
  meta: PaginationMeta;
}

export interface MetricResponse {
  success: true;
  data: Metric;
}

export interface CreateMetricRequest {
  key: string;
  name: string;
  description?: string;
  metric_type: MetricType;
  aggregation: MetricAggregation;
  attribution: MetricAttribution;
}

export interface UpdateMetricRequest {
  key?: string;
  name?: string;
  description?: string;
  aggregation?: MetricAggregation;
  attribution?: MetricAttribution;
  status?: MetricStatus;
}

export type MetricsFailureKind = 'http' | 'network' | 'unexpected';

export class MetricsApiError extends Error {
  readonly kind: MetricsFailureKind;
  readonly status: number | null;
  readonly code: string | null;

  constructor(
    kind: MetricsFailureKind,
    message: string,
    status: number | null,
    code: string | null,
  ) {
    super(message);
    this.name = 'MetricsApiError';
    this.kind = kind;
    this.status = status;
    this.code = code;
  }
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

function parseNullableUser(value: unknown): User | null | undefined {
  if (value === null || value === undefined) {
    return null;
  }
  return parseUser(value);
}

function parseOptionalString(value: unknown): string | undefined {
  if (value === null || value === undefined) {
    return undefined;
  }
  if (typeof value !== 'string') {
    return undefined;
  }
  return value;
}

export function parseEventRef(value: unknown): EventRef | null {
  if (!isRecord(value)) {
    return null;
  }
  const { event_type } = value;
  if (typeof event_type !== 'string') {
    return null;
  }
  const field = parseOptionalString(value['field']);
  if (value['field'] !== null && value['field'] !== undefined && field === undefined) {
    return null;
  }
  return field === undefined ? { event_type } : { event_type, field };
}

export function parseAggregation(value: unknown): MetricAggregation | null {
  if (!isRecord(value)) {
    return null;
  }
  const aggregation: MetricAggregation = {};
  const eventType = parseOptionalString(value['event_type']);
  if (value['event_type'] !== null && value['event_type'] !== undefined && eventType === undefined) {
    return null;
  }
  if (eventType !== undefined) {
    aggregation.event_type = eventType;
  }
  const field = parseOptionalString(value['field']);
  if (value['field'] !== null && value['field'] !== undefined && field === undefined) {
    return null;
  }
  if (field !== undefined) {
    aggregation.field = field;
  }
  const level = value['level'];
  if (level !== null && level !== undefined) {
    if (typeof level !== 'number') {
      return null;
    }
    aggregation.level = level;
  }
  if (value['numerator'] !== null && value['numerator'] !== undefined) {
    const numerator = parseEventRef(value['numerator']);
    if (numerator === null) {
      return null;
    }
    aggregation.numerator = numerator;
  }
  if (value['denominator'] !== null && value['denominator'] !== undefined) {
    const denominator = parseEventRef(value['denominator']);
    if (denominator === null) {
      return null;
    }
    aggregation.denominator = denominator;
  }
  return aggregation;
}

export function parseAttribution(value: unknown): MetricAttribution | null {
  if (!isRecord(value)) {
    return null;
  }
  const { require_exposure, window_days, fallback } = value;
  if (
    typeof require_exposure !== 'boolean' ||
    typeof window_days !== 'number' ||
    !isAttributionFallback(fallback)
  ) {
    return null;
  }
  return { require_exposure, window_days, fallback };
}

export function parseMetric(value: unknown): Metric | null {
  if (!isRecord(value)) {
    return null;
  }
  const {
    id,
    key,
    name,
    metric_type,
    is_builtin,
    status,
    created_at,
    updated_at,
  } = value;
  if (
    typeof id !== 'string' ||
    id.length === 0 ||
    typeof key !== 'string' ||
    typeof name !== 'string' ||
    !isMetricType(metric_type) ||
    typeof is_builtin !== 'boolean' ||
    !isMetricStatus(status) ||
    typeof created_at !== 'string' ||
    typeof updated_at !== 'string'
  ) {
    return null;
  }
  const description = value['description'];
  if (description !== null && description !== undefined && typeof description !== 'string') {
    return null;
  }
  const aggregation = parseAggregation(value['aggregation']);
  const attribution = parseAttribution(value['attribution']);
  if (aggregation === null || attribution === null) {
    return null;
  }
  const createdBy = parseNullableUser(value['created_by']);
  const updatedBy = parseNullableUser(value['updated_by']);
  if (createdBy === undefined || updatedBy === undefined) {
    return null;
  }
  return {
    id,
    key,
    name,
    description: typeof description === 'string' ? description : null,
    metric_type,
    aggregation,
    attribution,
    is_builtin,
    status,
    created_by: createdBy,
    updated_by: updatedBy,
    created_at,
    updated_at,
  };
}

function parsePaginationMeta(value: unknown): PaginationMeta | null {
  if (!isRecord(value)) {
    return null;
  }
  const { limit, offset, count, total, has_next } = value;
  if (
    typeof limit !== 'number' ||
    typeof offset !== 'number' ||
    typeof count !== 'number' ||
    typeof total !== 'number' ||
    typeof has_next !== 'boolean'
  ) {
    return null;
  }
  return { limit, offset, count, total, has_next };
}

export function parseMetricResponse(payload: unknown): MetricResponse | null {
  if (!isRecord(payload) || payload['success'] !== true) {
    return null;
  }
  const metric = parseMetric(payload['data']);
  if (metric === null) {
    return null;
  }
  return { success: true, data: metric };
}

export function parseMetricListResponse(payload: unknown): MetricListResponse | null {
  if (!isRecord(payload) || payload['success'] !== true) {
    return null;
  }
  const data = payload['data'];
  if (!Array.isArray(data)) {
    return null;
  }
  const metrics: Metric[] = [];
  for (const item of data) {
    const metric = parseMetric(item);
    if (metric === null) {
      return null;
    }
    metrics.push(metric);
  }
  const meta = parsePaginationMeta(payload['meta']);
  if (meta === null) {
    return null;
  }
  return { success: true, data: metrics, meta };
}

export function parseErrorPayload(payload: unknown): {
  code: string | null;
  message: string | null;
} {
  if (!isRecord(payload)) {
    return { code: null, message: null };
  }
  const error = payload['error'];
  if (!isRecord(error)) {
    return { code: null, message: null };
  }
  const code = error['code'];
  const message = error['message'];
  return {
    code: typeof code === 'string' ? code : null,
    message: typeof message === 'string' ? message : null,
  };
}
