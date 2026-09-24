import { clearAuthSession, getAuthToken } from '@/features/auth/lib/authSession';
import { PANEL_API_BASE_URL } from '@/shared/config/api';
import {
  MetricsApiError,
  parseErrorPayload,
  parseMetricListResponse,
  parseMetricResponse,
} from '../types';
import type {
  CreateMetricRequest,
  Metric,
  MetricListResponse,
  UpdateMetricRequest,
} from '../types';

export const METRICS_PAGE_SIZE = 20;

export function metricsListQueryKey(
  limit: number,
  offset: number,
  archived: boolean,
): [string, { limit: number; offset: number; archived: boolean }] {
  return ['metrics', { limit, offset, archived }];
}

export const METRICS_LIST_KEY_PREFIX = 'metrics';

export function metricDetailQueryKey(id: string): [string, string] {
  return ['metrics', id];
}

function authHeaders(): Record<string, string> {
  const token = getAuthToken();
  if (token === null || token.length === 0) {
    throw new MetricsApiError('unexpected', 'Missing auth token', null, null);
  }
  return { Authorization: `Bearer ${token}` };
}

async function readPayload(response: Response): Promise<unknown> {
  try {
    return (await response.json()) as unknown;
  } catch {
    return null;
  }
}

function toHttpError(response: Response, payload: unknown): MetricsApiError {
  const { code, message } = parseErrorPayload(payload);
  if (response.status === 401) {
    clearAuthSession();
  }
  return new MetricsApiError(
    'http',
    message ?? `Request failed with status ${String(response.status)}`,
    response.status,
    code,
  );
}

async function requestJson(input: string, init: RequestInit): Promise<unknown> {
  let response: Response;
  try {
    response = await fetch(input, init);
  } catch (error) {
    throw new MetricsApiError(
      'network',
      error instanceof Error ? error.message : 'Network request failed',
      null,
      null,
    );
  }
  if (!response.ok) {
    throw toHttpError(response, await readPayload(response));
  }
  return readPayload(response);
}

export interface ListMetricsParams {
  limit: number;
  offset: number;
  archived: boolean;
}

export async function listMetrics(params: ListMetricsParams): Promise<MetricListResponse> {
  const query = new URLSearchParams({
    limit: String(params.limit),
    offset: String(params.offset),
  });
  if (params.archived) {
    query.set('archived', 'true');
  }
  const payload = await requestJson(`${PANEL_API_BASE_URL}/metrics?${query.toString()}`, {
    headers: { ...authHeaders() },
  });
  const parsed = parseMetricListResponse(payload);
  if (parsed === null) {
    throw new MetricsApiError('unexpected', 'Unexpected response shape', null, null);
  }
  return parsed;
}

export async function getMetric(id: string, archived: boolean): Promise<Metric> {
  const query = archived ? '?archived=true' : '';
  const payload = await requestJson(
    `${PANEL_API_BASE_URL}/metrics/${encodeURIComponent(id)}${query}`,
    { headers: { ...authHeaders() } },
  );
  const parsed = parseMetricResponse(payload);
  if (parsed === null) {
    throw new MetricsApiError('unexpected', 'Unexpected response shape', null, null);
  }
  return parsed.data;
}

export async function createMetric(request: CreateMetricRequest): Promise<Metric> {
  const payload = await requestJson(`${PANEL_API_BASE_URL}/metrics`, {
    method: 'POST',
    headers: { ...authHeaders(), 'Content-Type': 'application/json' },
    body: JSON.stringify(request),
  });
  const parsed = parseMetricResponse(payload);
  if (parsed === null) {
    throw new MetricsApiError('unexpected', 'Unexpected response shape', null, null);
  }
  return parsed.data;
}

export async function updateMetric(id: string, request: UpdateMetricRequest): Promise<Metric> {
  const payload = await requestJson(
    `${PANEL_API_BASE_URL}/metrics/${encodeURIComponent(id)}`,
    {
      method: 'PATCH',
      headers: { ...authHeaders(), 'Content-Type': 'application/json' },
      body: JSON.stringify(request),
    },
  );
  const parsed = parseMetricResponse(payload);
  if (parsed === null) {
    throw new MetricsApiError('unexpected', 'Unexpected response shape', null, null);
  }
  return parsed.data;
}
