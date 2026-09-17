import { afterEach, describe, expect, it, vi } from 'vitest';
import { clearAuthSession, setAuthSession } from '@/features/auth/lib/authSession';
import {
  createFlag,
  deleteFlag,
  flagDetailQueryKey,
  flagsListQueryKey,
  getFlag,
  listFlags,
  updateFlag,
} from './flags';

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

function flagBody(overrides: Record<string, unknown> = {}): Record<string, unknown> {
  return {
    id: '0196a2f0-0000-7000-8000-000000000011',
    key: 'checkout_redesign',
    name: 'Checkout redesign',
    type: 'bool',
    default_value: false,
    description: null,
    created_by: null,
    updated_by: null,
    created_at: '2026-09-17T07:55:24.211Z',
    updated_at: '2026-09-17T07:55:24.211Z',
    ...overrides,
  };
}

function listBody(): unknown {
  return {
    success: true,
    data: [
      flagBody(),
      flagBody({
        id: '0196a2f0-0000-7000-8000-000000000012',
        key: 'price_cap',
        name: 'Price cap',
        type: 'number',
        default_value: 99.5,
      }),
      flagBody({
        id: '0196a2f0-0000-7000-8000-000000000013',
        key: 'welcome_text',
        name: 'Welcome text',
        type: 'string',
        default_value: 'hello',
      }),
    ],
    meta: { limit: 20, offset: 0, count: 3, total: 3, has_next: false },
  };
}

afterEach(() => {
  vi.unstubAllGlobals();
  clearAuthSession();
});

describe('flags api', () => {
  it('includes pagination state in list query keys', () => {
    expect(flagsListQueryKey(20, 0)).toEqual(['flags', { limit: 20, offset: 0 }]);
    expect(flagsListQueryKey(20, 40)).not.toEqual(flagsListQueryKey(20, 0));
    expect(flagDetailQueryKey('id-1')).toEqual(['flags', 'id-1']);
  });

  it('lists flags with limit/offset and parses typed defaults', async () => {
    setAuthSession('token', '2026-09-18T00:00:00.000Z');
    let seenUrl = '';
    let seenAuth: string | null = null;
    stubFetch((url, init) => {
      seenUrl = url;
      const headers = init.headers as Record<string, string>;
      seenAuth = headers['Authorization'] ?? null;
      return jsonResponse(200, listBody());
    });

    const result = await listFlags(20, 0);

    expect(seenUrl).toContain('/flags?');
    expect(seenUrl).toContain('limit=20');
    expect(seenUrl).toContain('offset=0');
    expect(seenAuth).toBe('Bearer token');
    expect(result.data).toHaveLength(3);
    expect(result.data[0]?.default_value).toBe(false);
    expect(result.data[1]?.default_value).toBe(99.5);
    expect(result.data[2]?.default_value).toBe('hello');
    expect(result.meta).toEqual({ limit: 20, offset: 0, count: 3, total: 3, has_next: false });
  });

  it('creates a flag with the create payload', async () => {
    setAuthSession('token', '2026-09-18T00:00:00.000Z');
    let seenBody: unknown = null;
    stubFetch((_url, init) => {
      seenBody = JSON.parse(init.body as string) as unknown;
      return jsonResponse(200, { success: true, data: flagBody() });
    });

    const flag = await createFlag({
      key: 'checkout_redesign',
      name: 'Checkout redesign',
      type: 'bool',
      default_value: false,
    });

    expect(seenBody).toEqual({
      key: 'checkout_redesign',
      name: 'Checkout redesign',
      type: 'bool',
      default_value: false,
    });
    expect(flag.key).toBe('checkout_redesign');
  });

  it('updates a flag with only the changed fields', async () => {
    setAuthSession('token', '2026-09-18T00:00:00.000Z');
    let seenBody: unknown = null;
    stubFetch((_url, init) => {
      seenBody = JSON.parse(init.body as string) as unknown;
      return jsonResponse(200, { success: true, data: flagBody({ default_value: true }) });
    });

    const flag = await updateFlag('fid-1', { default_value: true });

    expect(seenBody).toEqual({ default_value: true });
    expect(flag.default_value).toBe(true);
  });

  it('treats delete as success on 204 without a body', async () => {
    setAuthSession('token', '2026-09-18T00:00:00.000Z');
    let seenMethod = '';
    stubFetch((_url, init) => {
      seenMethod = init.method ?? '';
      return { ok: true, status: 204, json: () => Promise.reject(new SyntaxError('empty')) };
    });

    await expect(deleteFlag('fid-1')).resolves.toBeUndefined();
    expect(seenMethod).toBe('DELETE');
  });

  it('parses populated created_by/updated_by on detail responses', async () => {
    setAuthSession('token', '2026-09-18T00:00:00.000Z');
    const actor = {
      id: '0196a2f0-0000-7000-8000-000000000001',
      full_name: 'Root',
      email: 'root@labp.net',
      role: 'admin',
      avatar_url: null,
      created_at: '2026-09-17T07:55:24.211Z',
      updated_at: '2026-09-17T07:55:24.211Z',
    };
    stubFetch(() =>
      jsonResponse(200, { success: true, data: flagBody({ created_by: actor, updated_by: actor }) }),
    );

    const flag = await getFlag('fid-1');

    expect(flag.created_by?.email).toBe('root@labp.net');
    expect(flag.updated_by?.full_name).toBe('Root');
  });

  it('maps 409 conflict with backend code and message', async () => {
    setAuthSession('token', '2026-09-18T00:00:00.000Z');
    stubFetch(() =>
      jsonResponse(409, {
        success: false,
        error: { code: 'CONFLICT', message: 'flag key already exists' },
      }),
    );

    const error = await createFlag({
      key: 'checkout_redesign',
      name: 'Checkout redesign',
      type: 'bool',
      default_value: false,
    }).then(
      () => null,
      (e: unknown) => e,
    );

    expect(error).toMatchObject({
      name: 'FlagsApiError',
      kind: 'http',
      status: 409,
      code: 'CONFLICT',
    });
  });

  it('rejects payloads with an unknown flag type', async () => {
    setAuthSession('token', '2026-09-18T00:00:00.000Z');
    stubFetch(() => jsonResponse(200, { success: true, data: flagBody({ type: 'uuid' }) }));

    const error = await getFlag('fid-1').then(
      () => null,
      (e: unknown) => e,
    );

    expect(error).toMatchObject({ kind: 'unexpected' });
  });

  it('rejects payloads where default_value mismatches the type', async () => {
    setAuthSession('token', '2026-09-18T00:00:00.000Z');
    stubFetch(() => jsonResponse(200, { success: true, data: flagBody({ default_value: 'nope' }) }));

    const error = await getFlag('fid-1').then(
      () => null,
      (e: unknown) => e,
    );

    expect(error).toMatchObject({ kind: 'unexpected' });
  });

  it('maps 404 for unknown flags', async () => {
    setAuthSession('token', '2026-09-18T00:00:00.000Z');
    stubFetch(() =>
      jsonResponse(404, {
        success: false,
        error: { code: 'NOT_FOUND', message: 'flag not found' },
      }),
    );

    const error = await deleteFlag('missing').then(
      () => null,
      (e: unknown) => e,
    );

    expect(error).toMatchObject({ kind: 'http', status: 404, code: 'NOT_FOUND' });
  });

  it('maps network failures to network errors', async () => {
    setAuthSession('token', '2026-09-18T00:00:00.000Z');
    stubFetch(() => Promise.reject(new TypeError('Failed to fetch')));

    const error = await listFlags(20, 0).then(
      () => null,
      (e: unknown) => e,
    );

    expect(error).toMatchObject({ kind: 'network' });
  });

  it('requires an auth token for requests', async () => {
    const error = await listFlags(20, 0).then(
      () => null,
      (e: unknown) => e,
    );

    expect(error).toMatchObject({ kind: 'unexpected' });
  });
});
