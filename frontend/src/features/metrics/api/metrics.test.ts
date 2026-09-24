import { afterEach, describe, expect, it, vi } from 'vitest';
import { clearAuthSession, setAuthSession } from '@/features/auth/lib/authSession';
import {
  createMetric,
  getMetric,
  listMetrics,
  metricDetailQueryKey,
  metricsListQueryKey,
  updateMetric,
} from './metrics';

interface StubResponse {
  ok: boolean;
  status: number;
  json: () => Promise<unknown>;
}

function stubFetch(
  handler: (url: string, init: RequestInit) => Promise<StubResponse> | StubResponse,
): void {
  vi.stubGlobal('fetch', (url: unknown, init: unknown) =>
    handler(url as string, init as RequestInit),
  );
}

function jsonResponse(status: number, payload: unknown): StubResponse {
  return {
    ok: status >= 200 && status < 300,
    status,
    json: () => Promise.resolve(payload),
  };
}

function metricBody(overrides: Record<string, unknown> = {}): Record<string, unknown> {
  return {
    id: '0196a2f0-0000-7000-8000-000000000061',
    key: 'purchase_count',
    name: 'Purchase Count',
    description: null,
    metric_type: 'count',
    aggregation: { event_type: 'purchase' },
    attribution: { require_exposure: true, window_days: 7, fallback: 'subject' },
    is_builtin: false,
    status: 'active',
    created_by: null,
    updated_by: null,
    created_at: '2026-09-17T07:55:24.211Z',
    updated_at: '2026-09-17T07:55:24.211Z',
    ...overrides,
  };
}

afterEach(() => {
  vi.unstubAllGlobals();
  clearAuthSession();
});

describe('metrics api', () => {
  it('includes pagination and archived flag in list query keys', () => {
    expect(metricsListQueryKey(20, 0, false)).toEqual([
      'metrics',
      { limit: 20, offset: 0, archived: false },
    ]);
    expect(metricsListQueryKey(20, 0, true)).not.toEqual(metricsListQueryKey(20, 0, false));
    expect(metricDetailQueryKey('m-1')).toEqual(['metrics', 'm-1']);
  });

  it('lists metrics with the archived flag and parses aggregation shapes', async () => {
    setAuthSession('token', '2026-09-18T00:00:00.000Z');
    let seenUrl = '';
    stubFetch((url) => {
      seenUrl = url;
      return jsonResponse(200, {
        success: true,
        data: [
          metricBody(),
          metricBody({
            id: '0196a2f0-0000-7000-8000-000000000062',
            key: 'conversion_rate',
            name: 'Conversion Rate',
            metric_type: 'ratio',
            aggregation: {
              numerator: { event_type: 'purchase' },
              denominator: { event_type: 'visit' },
            },
          }),
        ],
        meta: { limit: 20, offset: 0, count: 2, total: 2, has_next: false },
      });
    });

    const result = await listMetrics({ limit: 20, offset: 0, archived: true });

    expect(seenUrl).toContain('archived=true');
    expect(result.data).toHaveLength(2);
    expect(result.data[0]?.aggregation).toEqual({ event_type: 'purchase' });
    expect(result.data[1]?.aggregation.denominator?.event_type).toBe('visit');
  });

  it('omits the archived flag when inactive', async () => {
    setAuthSession('token', '2026-09-18T00:00:00.000Z');
    let seenUrl = '';
    stubFetch((url) => {
      seenUrl = url;
      return jsonResponse(200, {
        success: true,
        data: [],
        meta: { limit: 20, offset: 0, count: 0, total: 0, has_next: false },
      });
    });

    await listMetrics({ limit: 20, offset: 0, archived: false });
    expect(seenUrl).not.toContain('archived');
  });

  it('creates a metric with the full config payload', async () => {
    setAuthSession('token', '2026-09-18T00:00:00.000Z');
    let seenBody: unknown = null;
    stubFetch((_url, init) => {
      seenBody = JSON.parse(init.body as string) as unknown;
      return jsonResponse(200, { success: true, data: metricBody() });
    });

    const metric = await createMetric({
      key: 'purchase_count',
      name: 'Purchase Count',
      metric_type: 'count',
      aggregation: { event_type: 'purchase' },
      attribution: { require_exposure: true, window_days: 7, fallback: 'subject' },
    });

    expect(seenBody).toEqual({
      key: 'purchase_count',
      name: 'Purchase Count',
      metric_type: 'count',
      aggregation: { event_type: 'purchase' },
      attribution: { require_exposure: true, window_days: 7, fallback: 'subject' },
    });
    expect(metric.key).toBe('purchase_count');
  });

  it('archives a metric through a status patch', async () => {
    setAuthSession('token', '2026-09-18T00:00:00.000Z');
    let seenBody: unknown = null;
    stubFetch((_url, init) => {
      seenBody = JSON.parse(init.body as string) as unknown;
      return jsonResponse(200, { success: true, data: metricBody({ status: 'archived' }) });
    });

    const metric = await updateMetric('m-1', { status: 'archived' });

    expect(seenBody).toEqual({ status: 'archived' });
    expect(metric.status).toBe('archived');
  });

  it('fetches archived metrics with the archived flag', async () => {
    setAuthSession('token', '2026-09-18T00:00:00.000Z');
    let seenUrl = '';
    stubFetch((url) => {
      seenUrl = url;
      return jsonResponse(200, { success: true, data: metricBody({ status: 'archived' }) });
    });

    const metric = await getMetric('m-1', true);

    expect(seenUrl).toContain('archived=true');
    expect(metric.status).toBe('archived');
  });

  it('maps builtin protection to forbidden errors', async () => {
    setAuthSession('token', '2026-09-18T00:00:00.000Z');
    stubFetch(() =>
      jsonResponse(403, {
        success: false,
        error: { code: 'FORBIDDEN', message: 'built-in metric fields cannot be modified' },
      }),
    );

    const error = await updateMetric('m-1', {
      aggregation: { event_type: 'other' },
    }).then(
      () => null,
      (e: unknown) => e,
    );

    expect(error).toMatchObject({ name: 'MetricsApiError', kind: 'http', status: 403 });
  });

  it('maps invalid configs to bad requests and conflicts to 409', async () => {
    setAuthSession('token', '2026-09-18T00:00:00.000Z');
    stubFetch(() =>
      jsonResponse(400, {
        success: false,
        error: { code: 'BAD_REQUEST', message: 'invalid metric aggregation or attribution' },
      }),
    );

    const badRequest = await createMetric({
      key: 'x',
      name: 'X',
      metric_type: 'count',
      aggregation: {},
      attribution: { require_exposure: true, window_days: 7, fallback: 'subject' },
    }).then(
      () => null,
      (e: unknown) => e,
    );
    expect(badRequest).toMatchObject({ kind: 'http', status: 400 });

    stubFetch(() =>
      jsonResponse(409, {
        success: false,
        error: { code: 'CONFLICT', message: 'metric key already exists' },
      }),
    );
    const conflict = await createMetric({
      key: 'purchase_count',
      name: 'Purchase Count',
      metric_type: 'count',
      aggregation: { event_type: 'purchase' },
      attribution: { require_exposure: true, window_days: 7, fallback: 'subject' },
    }).then(
      () => null,
      (e: unknown) => e,
    );
    expect(conflict).toMatchObject({ kind: 'http', status: 409, code: 'CONFLICT' });
  });

  it('rejects payloads with unknown metric types', async () => {
    setAuthSession('token', '2026-09-18T00:00:00.000Z');
    stubFetch(() =>
      jsonResponse(200, { success: true, data: metricBody({ metric_type: 'histogram' }) }),
    );

    const error = await getMetric('m-1', false).then(
      () => null,
      (e: unknown) => e,
    );
    expect(error).toMatchObject({ kind: 'unexpected' });
  });

  it('maps network failures and missing tokens', async () => {
    setAuthSession('token', '2026-09-18T00:00:00.000Z');
    stubFetch(() => Promise.reject(new TypeError('Failed to fetch')));

    const networkError = await listMetrics({ limit: 20, offset: 0, archived: false }).then(
      () => null,
      (e: unknown) => e,
    );
    expect(networkError).toMatchObject({ kind: 'network' });

    clearAuthSession();
    const missingToken = await listMetrics({ limit: 20, offset: 0, archived: false }).then(
      () => null,
      (e: unknown) => e,
    );
    expect(missingToken).toMatchObject({ kind: 'unexpected' });
  });
});
