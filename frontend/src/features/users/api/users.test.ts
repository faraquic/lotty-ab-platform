import { afterEach, describe, expect, it, vi } from 'vitest';
import { clearAuthSession, setAuthSession } from '@/features/auth/lib/authSession';
import {
  createUser,
  deleteUser,
  getMe,
  getUser,
  listUsers,
  updateUser,
  uploadUserAvatar,
  userDetailQueryKey,
  usersListQueryKey,
} from './users';

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

function userBody(overrides: Record<string, unknown> = {}): Record<string, unknown> {
  return {
    id: '0196a2f0-0000-7000-8000-000000000001',
    full_name: 'John Doe',
    email: 'john@example.com',
    role: 'experimenter',
    avatar_url: null,
    created_at: '2026-09-17T07:55:24.211Z',
    updated_at: '2026-09-17T07:55:24.211Z',
    ...overrides,
  };
}

function listBody(): unknown {
  return {
    success: true,
    data: [userBody(), userBody({ id: '0196a2f0-0000-7000-8000-000000000002', role: 'admin' })],
    meta: { limit: 20, offset: 0, count: 2, total: 2, has_next: false },
  };
}

afterEach(() => {
  vi.unstubAllGlobals();
  clearAuthSession();
});

describe('users api', () => {
  it('includes pagination state in list query keys', () => {
    expect(usersListQueryKey(20, 0)).toEqual(['users', { limit: 20, offset: 0 }]);
    expect(usersListQueryKey(20, 40)).not.toEqual(usersListQueryKey(20, 0));
    expect(userDetailQueryKey('id-1')).toEqual(['users', 'id-1']);
  });

  it('lists users with limit/offset and parses pagination meta', async () => {
    setAuthSession('token', '2026-09-18T00:00:00.000Z');
    let seenUrl = '';
    let seenAuth: string | null = null;
    stubFetch((url, init) => {
      seenUrl = url;
      const headers = init.headers as Record<string, string>;
      seenAuth = headers['Authorization'] ?? null;
      return jsonResponse(200, listBody());
    });

    const result = await listUsers(20, 40);

    expect(seenUrl).toContain('/users?');
    expect(seenUrl).toContain('limit=20');
    expect(seenUrl).toContain('offset=40');
    expect(seenAuth).toBe('Bearer token');
    expect(result.data).toHaveLength(2);
    expect(result.meta).toEqual({ limit: 20, offset: 0, count: 2, total: 2, has_next: false });
    expect(result.data[1]?.role).toBe('admin');
  });

  it('creates a user with the create payload', async () => {
    setAuthSession('token', '2026-09-18T00:00:00.000Z');
    let seenBody: unknown = null;
    stubFetch((_url, init) => {
      seenBody = JSON.parse(init.body as string) as unknown;
      return jsonResponse(200, { success: true, data: userBody() });
    });

    const user = await createUser({
      full_name: 'John Doe',
      email: 'john@example.com',
      password: 'secret123',
      role: 'viewer',
    });

    expect(seenBody).toEqual({
      full_name: 'John Doe',
      email: 'john@example.com',
      password: 'secret123',
      role: 'viewer',
    });
    expect(user.email).toBe('john@example.com');
  });

  it('updates a user with only the changed fields', async () => {
    setAuthSession('token', '2026-09-18T00:00:00.000Z');
    let seenBody: unknown = null;
    stubFetch((_url, init) => {
      seenBody = JSON.parse(init.body as string) as unknown;
      return jsonResponse(200, {
        success: true,
        data: userBody({ role: 'approver' }),
      });
    });

    const user = await updateUser('uid-1', { role: 'approver' });

    expect(seenBody).toEqual({ role: 'approver' });
    expect(user.role).toBe('approver');
  });

  it('treats delete as success on 204 without a body', async () => {
    setAuthSession('token', '2026-09-18T00:00:00.000Z');
    let seenMethod = '';
    stubFetch((_url, init) => {
      seenMethod = init.method ?? '';
      return { ok: true, status: 204, json: () => Promise.reject(new SyntaxError('empty')) };
    });

    await expect(deleteUser('uid-1')).resolves.toBeUndefined();
    expect(seenMethod).toBe('DELETE');
  });

  it('uploads avatar as multipart form-data', async () => {
    setAuthSession('token', '2026-09-18T00:00:00.000Z');
    let seenBody: unknown = null;
    stubFetch((_url, init) => {
      seenBody = init.body;
      return jsonResponse(200, {
        success: true,
        data: userBody({ avatar_url: 'https://cdn.example/a.png' }),
      });
    });

    const file = new File(['bytes'], 'a.png', { type: 'image/png' });
    const user = await uploadUserAvatar('uid-1', file);

    expect(seenBody).toBeInstanceOf(FormData);
    expect((seenBody as FormData).get('avatar')).toBe(file);
    expect(user.avatar_url).toBe('https://cdn.example/a.png');
  });

  it('deletes avatar by posting an empty form', async () => {
    setAuthSession('token', '2026-09-18T00:00:00.000Z');
    let seenBody: unknown = null;
    stubFetch((_url, init) => {
      seenBody = init.body;
      return jsonResponse(200, { success: true, data: userBody() });
    });

    const user = await uploadUserAvatar('uid-1', null);

    expect(seenBody).toBeInstanceOf(FormData);
    expect((seenBody as FormData).has('avatar')).toBe(false);
    expect(user.avatar_url).toBeNull();
  });

  it('reads the current user via /me', async () => {
    setAuthSession('token', '2026-09-18T00:00:00.000Z');
    let seenUrl = '';
    stubFetch((url) => {
      seenUrl = url;
      return jsonResponse(200, { success: true, data: userBody({ role: 'admin' }) });
    });

    const me = await getMe();

    expect(seenUrl.endsWith('/me')).toBe(true);
    expect(me.role).toBe('admin');
  });

  it('fetches a single user by id', async () => {
    setAuthSession('token', '2026-09-18T00:00:00.000Z');
    stubFetch(() => jsonResponse(200, { success: true, data: userBody() }));

    const user = await getUser('uid-1');

    expect(user.id).toBe('0196a2f0-0000-7000-8000-000000000001');
  });

  it('maps 409 conflict with backend code and message', async () => {
    setAuthSession('token', '2026-09-18T00:00:00.000Z');
    stubFetch(() =>
      jsonResponse(409, {
        success: false,
        error: { code: 'CONFLICT', message: 'user already exists' },
      }),
    );

    const error = await createUser({
      full_name: 'John Doe',
      email: 'john@example.com',
      password: 'secret123',
      role: 'viewer',
    }).then(
      () => null,
      (e: unknown) => e,
    );

    expect(error).toMatchObject({ name: 'UsersApiError', kind: 'http', status: 409, code: 'CONFLICT' });
  });

  it('maps 403 forbidden for self-protection and other denials', async () => {
    setAuthSession('token', '2026-09-18T00:00:00.000Z');
    stubFetch(() =>
      jsonResponse(403, {
        success: false,
        error: { code: 'FORBIDDEN', message: 'cannot delete your own account' },
      }),
    );

    const error = await deleteUser('uid-1').then(
      () => null,
      (e: unknown) => e,
    );

    expect(error).toMatchObject({ kind: 'http', status: 403, code: 'FORBIDDEN' });
  });

  it('rejects payloads with an unknown role enum value', async () => {
    setAuthSession('token', '2026-09-18T00:00:00.000Z');
    stubFetch(() => jsonResponse(200, { success: true, data: userBody({ role: 'superadmin' }) }));

    const error = await getMe().then(
      () => null,
      (e: unknown) => e,
    );

    expect(error).toMatchObject({ kind: 'unexpected' });
  });

  it('maps network failures to network errors', async () => {
    setAuthSession('token', '2026-09-18T00:00:00.000Z');
    stubFetch(() => Promise.reject(new TypeError('Failed to fetch')));

    const error = await listUsers(20, 0).then(
      () => null,
      (e: unknown) => e,
    );

    expect(error).toMatchObject({ kind: 'network' });
  });

  it('requires an auth token for requests', async () => {
    const error = await listUsers(20, 0).then(
      () => null,
      (e: unknown) => e,
    );

    expect(error).toMatchObject({ kind: 'unexpected' });
  });
});
