import { afterEach, describe, expect, it, vi } from 'vitest';
import { clearAuthSession, setAuthSession } from '@/features/auth/lib/authSession';
import {
  actOnReview,
  addGroupMember,
  addReviewComment,
  createApproverGroup,
  deleteReviewComment,
  getApproverGroup,
  getReview,
  groupDetailQueryKey,
  groupsListQueryKey,
  listApproverGroups,
  listReviews,
  removeGroupMember,
  resolveReviewComment,
  reviewDetailQueryKey,
  reviewsListQueryKey,
  setExperimenterGroup,
  updateApproverGroup,
} from './reviews';

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

function groupBody(overrides: Record<string, unknown> = {}): Record<string, unknown> {
  return {
    id: '0196a2f0-0000-7000-8000-000000000041',
    name: 'Growth approvers',
    description: null,
    min_approvals: 2,
    status: 'active',
    members: [
      {
        id: '0196a2f0-0000-7000-8000-000000000001',
        full_name: 'Root',
        email: 'root@labp.net',
        role: 'admin',
        avatar_url: null,
      },
    ],
    created_at: '2026-09-17T07:55:24.211Z',
    updated_at: '2026-09-17T07:55:24.211Z',
    ...overrides,
  };
}

function reviewBody(overrides: Record<string, unknown> = {}): Record<string, unknown> {
  return {
    id: '0196a2f0-0000-7000-8000-000000000051',
    experiment_id: '0196a2f0-0000-7000-8000-000000000020',
    version_id: '0196a2f0-0000-7000-8000-000000000021',
    version_num: 1,
    status: 'open',
    approvals: [
      {
        id: '0196a2f0-0000-7000-8000-000000000061',
        reviewer_id: '0196a2f0-0000-7000-8000-000000000002',
        decision: 'approve',
        comment: 'looks good',
        created_at: '2026-09-17T07:55:24.211Z',
      },
    ],
    comments: [
      {
        id: '0196a2f0-0000-7000-8000-000000000071',
        author_id: '0196a2f0-0000-7000-8000-000000000001',
        parent_id: null,
        body: 'Please check weights',
        resolved: false,
        created_at: '2026-09-17T07:55:24.211Z',
        updated_at: '2026-09-17T07:55:24.211Z',
        replies: [
          {
            id: '0196a2f0-0000-7000-8000-000000000072',
            author_id: '0196a2f0-0000-7000-8000-000000000002',
            parent_id: '0196a2f0-0000-7000-8000-000000000071',
            body: 'Fixed',
            resolved: false,
            created_at: '2026-09-17T07:55:24.211Z',
            updated_at: '2026-09-17T07:55:24.211Z',
            replies: [],
          },
        ],
      },
    ],
    created_by: '0196a2f0-0000-7000-8000-000000000001',
    created_at: '2026-09-17T07:55:24.211Z',
    updated_at: '2026-09-17T07:55:24.211Z',
    ...overrides,
  };
}

afterEach(() => {
  vi.unstubAllGlobals();
  clearAuthSession();
});

describe('reviews api', () => {
  it('builds list and detail query keys', () => {
    expect(reviewsListQueryKey(20, 0, null)).toEqual([
      'reviews',
      { limit: 20, offset: 0, status: null },
    ]);
    expect(reviewsListQueryKey(20, 0, 'open')).not.toEqual(reviewsListQueryKey(20, 0, null));
    expect(reviewDetailQueryKey('r-1')).toEqual(['reviews', 'r-1']);
    expect(groupsListQueryKey(20, 0)).toEqual(['approver-groups', { limit: 20, offset: 0 }]);
    expect(groupDetailQueryKey('g-1')).toEqual(['approver-groups', 'g-1']);
  });

  it('lists reviews with status filter and parses approvals plus threaded comments', async () => {
    setAuthSession('token', '2026-09-18T00:00:00.000Z');
    let seenUrl = '';
    stubFetch((url) => {
      seenUrl = url;
      return jsonResponse(200, {
        success: true,
        data: [reviewBody()],
        meta: { limit: 20, offset: 0, count: 1, total: 1, has_next: false },
      });
    });

    const result = await listReviews({ limit: 20, offset: 0, status: 'open' });

    expect(seenUrl).toContain('status=open');
    expect(result.data).toHaveLength(1);
    expect(result.data[0]?.approvals[0]?.decision).toBe('approve');
    expect(result.data[0]?.comments[0]?.replies).toHaveLength(1);
  });

  it('acts on a review with decision, version and optional comment', async () => {
    setAuthSession('token', '2026-09-18T00:00:00.000Z');
    let seenBody: unknown = null;
    let seenUrl = '';
    stubFetch((url, init) => {
      seenUrl = url;
      seenBody = JSON.parse(init.body as string) as unknown;
      return jsonResponse(200, { success: true, data: reviewBody({ status: 'approved' }) });
    });

    const review = await actOnReview('rev-1', {
      decision: 'approve',
      version: 2,
      comment: 'ship it',
    });

    expect(seenUrl.endsWith('/reviews/rev-1/approvals')).toBe(true);
    expect(seenBody).toEqual({ decision: 'approve', version: 2, comment: 'ship it' });
    expect(review.status).toBe('approved');
  });

  it('adds, resolves and deletes comments through nested routes', async () => {
    setAuthSession('token', '2026-09-18T00:00:00.000Z');
    const seenUrls: string[] = [];
    stubFetch((url, init) => {
      seenUrls.push(`${init.method ?? 'GET'} ${url}`);
      if ((init.method ?? 'GET') === 'DELETE') {
        return { ok: true, status: 204, json: () => Promise.reject(new SyntaxError('empty')) };
      }
      return jsonResponse(200, {
        success: true,
        data: {
          id: 'c-1',
          author_id: 'u-1',
          parent_id: null,
          body: 'note',
          resolved: false,
          created_at: '2026-09-17T07:55:24.211Z',
          updated_at: '2026-09-17T07:55:24.211Z',
          replies: [],
        },
      });
    });

    await addReviewComment('rev-1', { body: 'note' });
    await resolveReviewComment('c-1', { resolved: true });
    await deleteReviewComment('c-1');

    expect(seenUrls.some((entry) => entry.endsWith('/reviews/rev-1/comments'))).toBe(true);
    expect(seenUrls.some((entry) => entry.endsWith('/reviews/comments/c-1/resolve'))).toBe(true);
    expect(seenUrls.some((entry) => entry.startsWith('DELETE'))).toBe(true);
  });

  it('manages approver groups and members', async () => {
    setAuthSession('token', '2026-09-18T00:00:00.000Z');
    const seen: { method: string; url: string; body: unknown }[] = [];
    stubFetch((url, init) => {
      let body: unknown = null;
      try {
        body = init.body ? (JSON.parse(init.body as string) as unknown) : null;
      } catch {
        body = null;
      }
      seen.push({ method: init.method ?? 'GET', url, body });
      if ((init.method ?? 'GET') === 'DELETE') {
        return jsonResponse(200, { success: true, data: groupBody({ members: [] }) });
      }
      return jsonResponse(200, { success: true, data: groupBody() });
    });

    await createApproverGroup({ name: 'Growth approvers', min_approvals: 2 });
    await updateApproverGroup('g-1', { min_approvals: 3 });
    await getApproverGroup('g-1');
    await addGroupMember('g-1', { user_id: 'u-9' });
    const afterRemove = await removeGroupMember('g-1', 'u-9');

    expect(seen[0]).toMatchObject({ method: 'POST', url: expect.stringContaining('/approver-groups') as unknown });
    expect(seen[0]?.body).toEqual({ name: 'Growth approvers', min_approvals: 2 });
    expect(seen[1]?.body).toEqual({ min_approvals: 3 });
    expect(afterRemove.members).toHaveLength(0);
  });

  it('lists groups and assigns experimenters to groups', async () => {
    setAuthSession('token', '2026-09-18T00:00:00.000Z');
    let seenUrl = '';
    let seenBody: unknown = null;
    stubFetch((url, init) => {
      seenUrl = url;
      if (url.includes('/experimenters/')) {
        seenBody = JSON.parse(init.body as string) as unknown;
        return jsonResponse(200, { success: true, data: null });
      }
      return jsonResponse(200, {
        success: true,
        data: [groupBody()],
        meta: { limit: 20, offset: 0, count: 1, total: 1, has_next: false },
      });
    });

    const groups = await listApproverGroups(20, 0);
    expect(seenUrl).toContain('/approver-groups?');
    expect(groups.data).toHaveLength(1);

    await setExperimenterGroup('u-5', { group_id: 'g-1' });
    expect(seenBody).toEqual({ group_id: 'g-1' });
    await setExperimenterGroup('u-5', { group_id: null });
    expect(seenBody).toEqual({ group_id: null });
  });

  it('fetches a single review', async () => {
    setAuthSession('token', '2026-09-18T00:00:00.000Z');
    stubFetch(() => jsonResponse(200, { success: true, data: reviewBody() }));

    const review = await getReview('rev-1');
    expect(review.version_num).toBe(1);
  });

  it('maps duplicate actions to conflicts and closed reviews', async () => {
    setAuthSession('token', '2026-09-18T00:00:00.000Z');
    stubFetch(() =>
      jsonResponse(409, {
        success: false,
        error: { code: 'CONFLICT', message: 'reviewer already acted' },
      }),
    );

    const error = await actOnReview('rev-1', { decision: 'approve', version: 2 }).then(
      () => null,
      (e: unknown) => e,
    );

    expect(error).toMatchObject({ name: 'ReviewsApiError', kind: 'http', status: 409 });
  });

  it('maps forbidden review actions', async () => {
    setAuthSession('token', '2026-09-18T00:00:00.000Z');
    stubFetch(() =>
      jsonResponse(403, {
        success: false,
        error: { code: 'FORBIDDEN', message: 'not eligible to review' },
      }),
    );

    const error = await actOnReview('rev-1', { decision: 'reject', version: 2 }).then(
      () => null,
      (e: unknown) => e,
    );

    expect(error).toMatchObject({ kind: 'http', status: 403, code: 'FORBIDDEN' });
  });

  it('rejects payloads with unknown decisions or malformed comments', async () => {
    setAuthSession('token', '2026-09-18T00:00:00.000Z');
    stubFetch(() =>
      jsonResponse(200, {
        success: true,
        data: reviewBody({
          approvals: [{ id: 'a', reviewer_id: 'u', decision: 'maybe', comment: null, created_at: 'x' }],
        }),
      }),
    );

    const error = await getReview('rev-1').then(
      () => null,
      (e: unknown) => e,
    );
    expect(error).toMatchObject({ kind: 'unexpected' });
  });

  it('maps network failures and missing tokens', async () => {
    setAuthSession('token', '2026-09-18T00:00:00.000Z');
    stubFetch(() => Promise.reject(new TypeError('Failed to fetch')));

    const networkError = await listReviews({ limit: 20, offset: 0, status: null }).then(
      () => null,
      (e: unknown) => e,
    );
    expect(networkError).toMatchObject({ kind: 'network' });

    clearAuthSession();
    const missingToken = await listReviews({ limit: 20, offset: 0, status: null }).then(
      () => null,
      (e: unknown) => e,
    );
    expect(missingToken).toMatchObject({ kind: 'unexpected' });
  });
});
