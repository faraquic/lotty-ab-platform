import { afterEach, describe, expect, it, vi } from 'vitest';
import { clearAuthSession, setAuthSession } from '@/features/auth/lib/authSession';
import {
  completeExperiment,
  createExperiment,
  createExperimentVersion,
  experimentDetailQueryKey,
  experimentsListQueryKey,
  getExperiment,
  listExperiments,
  rolloutExperiment,
  setExperimentVariants,
  transitionExperiment,
  updateExperiment,
} from './experiments';

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

function versionBody(overrides: Record<string, unknown> = {}): Record<string, unknown> {
  return {
    id: '0196a2f0-0000-7000-8000-000000000021',
    experiment_id: '0196a2f0-0000-7000-8000-000000000020',
    version_num: 1,
    review_id: null,
    weights_total: 10000,
    targeting: null,
    distribution_salt: 'abc123',
    created_by: '0196a2f0-0000-7000-8000-000000000001',
    created_at: '2026-09-17T07:55:24.211Z',
    updated_at: '2026-09-17T07:55:24.211Z',
    ...overrides,
  };
}

function experimentBody(overrides: Record<string, unknown> = {}): Record<string, unknown> {
  return {
    id: '0196a2f0-0000-7000-8000-000000000020',
    flag_id: '0196a2f0-0000-7000-8000-000000000011',
    name: 'Checkout test',
    description: null,
    status: 'draft',
    current_version_id: '0196a2f0-0000-7000-8000-000000000021',
    owner_id: '0196a2f0-0000-7000-8000-000000000001',
    version: 1,
    guardrail_paused: false,
    completion_decision: null,
    completion_reason: null,
    created_by: null,
    updated_by: null,
    current_version: versionBody(),
    variants: [
      {
        id: '0196a2f0-0000-7000-8000-000000000031',
        name: 'control',
        value: false,
        weight_bp: 5000,
        is_control: true,
      },
      {
        id: '0196a2f0-0000-7000-8000-000000000032',
        name: 'treatment',
        value: true,
        weight_bp: 5000,
        is_control: false,
      },
    ],
    created_at: '2026-09-17T07:55:24.211Z',
    updated_at: '2026-09-17T07:55:24.211Z',
    ...overrides,
  };
}

afterEach(() => {
  vi.unstubAllGlobals();
  clearAuthSession();
});

describe('experiments api', () => {
  it('includes pagination and status filter in list query keys', () => {
    expect(experimentsListQueryKey(20, 0, null)).toEqual([
      'experiments',
      { limit: 20, offset: 0, status: null },
    ]);
    expect(experimentsListQueryKey(20, 0, 'running')).not.toEqual(
      experimentsListQueryKey(20, 0, null),
    );
    expect(experimentDetailQueryKey('id-1')).toEqual(['experiments', 'id-1']);
  });

  it('lists experiments with limit/offset/status and parses the shape', async () => {
    setAuthSession('token', '2026-09-18T00:00:00.000Z');
    let seenUrl = '';
    stubFetch((url) => {
      seenUrl = url;
      return jsonResponse(200, {
        success: true,
        data: [experimentBody()],
        meta: { limit: 20, offset: 0, count: 1, total: 1, has_next: false },
      });
    });

    const result = await listExperiments({ limit: 20, offset: 0, status: 'running' });

    expect(seenUrl).toContain('/experiments?');
    expect(seenUrl).toContain('status=running');
    expect(result.data).toHaveLength(1);
    expect(result.data[0]?.status).toBe('draft');
    expect(result.data[0]?.variants).toHaveLength(2);
    expect(result.data[0]?.current_version?.weights_total).toBe(10000);
  });

  it('creates an experiment with the create payload', async () => {
    setAuthSession('token', '2026-09-18T00:00:00.000Z');
    let seenBody: unknown = null;
    stubFetch((_url, init) => {
      seenBody = JSON.parse(init.body as string) as unknown;
      return jsonResponse(200, { success: true, data: experimentBody() });
    });

    const experiment = await createExperiment({ flag_id: 'flag-1', name: 'Checkout test' });

    expect(seenBody).toEqual({ flag_id: 'flag-1', name: 'Checkout test' });
    expect(experiment.name).toBe('Checkout test');
  });

  it('updates a draft with version for optimistic locking', async () => {
    setAuthSession('token', '2026-09-18T00:00:00.000Z');
    let seenBody: unknown = null;
    stubFetch((_url, init) => {
      seenBody = JSON.parse(init.body as string) as unknown;
      return jsonResponse(200, { success: true, data: experimentBody({ name: 'Renamed' }) });
    });

    const experiment = await updateExperiment('exp-1', { version: 3, name: 'Renamed' });

    expect(seenBody).toEqual({ version: 3, name: 'Renamed' });
    expect(experiment.name).toBe('Renamed');
  });

  it('creates versions and sets variants against nested routes', async () => {
    setAuthSession('token', '2026-09-18T00:00:00.000Z');
    const seenUrls: string[] = [];
    stubFetch((url) => {
      seenUrls.push(url);
      return jsonResponse(200, { success: true, data: experimentBody() });
    });

    await createExperimentVersion('exp-1', { version: 2, weights_total: 10000 });
    await setExperimentVariants('exp-1', {
      version: 2,
      variants: [
        { name: 'control', value: false, weight_bp: 5000, is_control: true },
        { name: 'treatment', value: true, weight_bp: 5000, is_control: false },
      ],
    });

    expect(seenUrls.some((url) => url.endsWith('/experiments/exp-1/versions'))).toBe(true);
    expect(seenUrls.some((url) => url.endsWith('/experiments/exp-1/variants'))).toBe(true);
  });

  it('posts transitions, complete and rollout to their action routes', async () => {
    setAuthSession('token', '2026-09-18T00:00:00.000Z');
    const seen: { url: string; body: unknown }[] = [];
    stubFetch((url, init) => {
      seen.push({ url, body: JSON.parse(init.body as string) as unknown });
      return jsonResponse(200, { success: true, data: experimentBody({ status: 'running' }) });
    });

    await transitionExperiment('exp-1', 'start', { version: 2 });
    await completeExperiment('exp-1', { version: 2, decision: 'no_effect', reason: 'flat' });
    await rolloutExperiment('exp-1', {
      version: 2,
      reason: 'winner',
      winner_variant_id: 'var-2',
    });
    await transitionExperiment('exp-1', 'internal-pause', { version: 2 });

    expect(seen[0]?.url.endsWith('/experiments/exp-1/start')).toBe(true);
    expect(seen[0]?.body).toEqual({ version: 2 });
    expect(seen[1]?.url.endsWith('/experiments/exp-1/complete')).toBe(true);
    expect(seen[2]?.url.endsWith('/experiments/exp-1/rollout')).toBe(true);
    expect(seen[3]?.url.endsWith('/internal/experiments/exp-1/pause')).toBe(true);
  });

  it('maps version conflicts to http errors with backend detail', async () => {
    setAuthSession('token', '2026-09-18T00:00:00.000Z');
    stubFetch(() =>
      jsonResponse(409, {
        success: false,
        error: { code: 'CONFLICT', message: 'experiment version conflict' },
      }),
    );

    const error = await transitionExperiment('exp-1', 'start', { version: 1 }).then(
      () => null,
      (e: unknown) => e,
    );

    expect(error).toMatchObject({
      name: 'ExperimentsApiError',
      kind: 'http',
      status: 409,
      code: 'CONFLICT',
    });
  });

  it('maps forbidden owner errors', async () => {
    setAuthSession('token', '2026-09-18T00:00:00.000Z');
    stubFetch(() =>
      jsonResponse(403, {
        success: false,
        error: { code: 'FORBIDDEN', message: 'not experiment owner' },
      }),
    );

    const error = await getExperiment('exp-1').then(
      () => null,
      (e: unknown) => e,
    );

    expect(error).toMatchObject({ kind: 'http', status: 403, code: 'FORBIDDEN' });
  });

  it('maps unprocessable variant errors', async () => {
    setAuthSession('token', '2026-09-18T00:00:00.000Z');
    stubFetch(() =>
      jsonResponse(422, {
        success: false,
        error: { code: 'UNPROCESSABLE_ENTITY', message: 'invalid variants' },
      }),
    );

    const error = await setExperimentVariants('exp-1', { version: 1, variants: [] }).then(
      () => null,
      (e: unknown) => e,
    );

    expect(error).toMatchObject({ kind: 'http', status: 422 });
  });

  it('rejects payloads with unknown status or malformed variants', async () => {
    setAuthSession('token', '2026-09-18T00:00:00.000Z');
    stubFetch(() =>
      jsonResponse(200, { success: true, data: experimentBody({ status: 'flying' }) }),
    );

    const badStatus = await getExperiment('exp-1').then(
      () => null,
      (e: unknown) => e,
    );
    expect(badStatus).toMatchObject({ kind: 'unexpected' });

    stubFetch(() =>
      jsonResponse(200, { success: true, data: experimentBody({ variants: [{ nope: true }] }) }),
    );
    const badVariants = await getExperiment('exp-1').then(
      () => null,
      (e: unknown) => e,
    );
    expect(badVariants).toMatchObject({ kind: 'unexpected' });
  });

  it('maps network failures and missing tokens', async () => {
    setAuthSession('token', '2026-09-18T00:00:00.000Z');
    stubFetch(() => Promise.reject(new TypeError('Failed to fetch')));

    const networkError = await listExperiments({ limit: 20, offset: 0, status: null }).then(
      () => null,
      (e: unknown) => e,
    );
    expect(networkError).toMatchObject({ kind: 'network' });

    clearAuthSession();
    const missingToken = await listExperiments({ limit: 20, offset: 0, status: null }).then(
      () => null,
      (e: unknown) => e,
    );
    expect(missingToken).toMatchObject({ kind: 'unexpected' });
  });
});
