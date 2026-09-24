import { clearAuthSession, getAuthToken } from '@/features/auth/lib/authSession';
import { PANEL_API_BASE_URL } from '@/shared/config/api';
import {
  ReviewsApiError,
  parseApproverGroupListResponse,
  parseApproverGroupResponse,
  parseErrorPayload,
  parseReviewCommentResponse,
  parseReviewListResponse,
  parseReviewResponse,
  parseSuccessPayload,
} from '../types';
import type {
  ActOnReviewRequest,
  AddGroupMemberRequest,
  AddReviewCommentRequest,
  ApproverGroup,
  ApproverGroupListResponse,
  CreateApproverGroupRequest,
  ResolveReviewCommentRequest,
  Review,
  ReviewComment,
  ReviewListResponse,
  ReviewStatus,
  SetExperimenterGroupRequest,
  UpdateApproverGroupRequest,
} from '../types';

export const REVIEWS_PAGE_SIZE = 20;
export const GROUPS_PAGE_SIZE = 20;

export function reviewsListQueryKey(
  limit: number,
  offset: number,
  status: ReviewStatus | null,
): [string, { limit: number; offset: number; status: string | null }] {
  return ['reviews', { limit, offset, status }];
}

export function groupsListQueryKey(
  limit: number,
  offset: number,
): [string, { limit: number; offset: number }] {
  return ['approver-groups', { limit, offset }];
}

export const REVIEWS_LIST_KEY_PREFIX = 'reviews';
export const GROUPS_LIST_KEY_PREFIX = 'approver-groups';

export function reviewDetailQueryKey(id: string): [string, string] {
  return ['reviews', id];
}

export function groupDetailQueryKey(id: string): [string, string] {
  return ['approver-groups', id];
}

function authHeaders(): Record<string, string> {
  const token = getAuthToken();
  if (token === null || token.length === 0) {
    throw new ReviewsApiError('unexpected', 'Missing auth token', null, null);
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

function toHttpError(response: Response, payload: unknown): ReviewsApiError {
  const { code, message } = parseErrorPayload(payload);
  if (response.status === 401) {
    clearAuthSession();
  }
  return new ReviewsApiError(
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
    throw new ReviewsApiError(
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

export interface ListReviewsParams {
  limit: number;
  offset: number;
  status: ReviewStatus | null;
}

export async function listReviews(params: ListReviewsParams): Promise<ReviewListResponse> {
  const query = new URLSearchParams({
    limit: String(params.limit),
    offset: String(params.offset),
  });
  if (params.status !== null) {
    query.set('status', params.status);
  }
  const payload = await requestJson(`${PANEL_API_BASE_URL}/reviews?${query.toString()}`, {
    headers: { ...authHeaders() },
  });
  const parsed = parseReviewListResponse(payload);
  if (parsed === null) {
    throw new ReviewsApiError('unexpected', 'Unexpected response shape', null, null);
  }
  return parsed;
}

export async function getReview(id: string): Promise<Review> {
  const payload = await requestJson(
    `${PANEL_API_BASE_URL}/reviews/${encodeURIComponent(id)}`,
    { headers: { ...authHeaders() } },
  );
  const parsed = parseReviewResponse(payload);
  if (parsed === null) {
    throw new ReviewsApiError('unexpected', 'Unexpected response shape', null, null);
  }
  return parsed.data;
}

export async function actOnReview(id: string, request: ActOnReviewRequest): Promise<Review> {
  const payload = await requestJson(
    `${PANEL_API_BASE_URL}/reviews/${encodeURIComponent(id)}/approvals`,
    {
      method: 'POST',
      headers: { ...authHeaders(), 'Content-Type': 'application/json' },
      body: JSON.stringify(request),
    },
  );
  const parsed = parseReviewResponse(payload);
  if (parsed === null) {
    throw new ReviewsApiError('unexpected', 'Unexpected response shape', null, null);
  }
  return parsed.data;
}

export async function addReviewComment(
  id: string,
  request: AddReviewCommentRequest,
): Promise<ReviewComment> {
  const payload = await requestJson(
    `${PANEL_API_BASE_URL}/reviews/${encodeURIComponent(id)}/comments`,
    {
      method: 'POST',
      headers: { ...authHeaders(), 'Content-Type': 'application/json' },
      body: JSON.stringify(request),
    },
  );
  const parsed = parseReviewCommentResponse(payload);
  if (parsed === null) {
    throw new ReviewsApiError('unexpected', 'Unexpected response shape', null, null);
  }
  return parsed.data;
}

export async function resolveReviewComment(
  commentId: string,
  request: ResolveReviewCommentRequest,
): Promise<ReviewComment> {
  const payload = await requestJson(
    `${PANEL_API_BASE_URL}/reviews/comments/${encodeURIComponent(commentId)}/resolve`,
    {
      method: 'PATCH',
      headers: { ...authHeaders(), 'Content-Type': 'application/json' },
      body: JSON.stringify(request),
    },
  );
  const parsed = parseReviewCommentResponse(payload);
  if (parsed === null) {
    throw new ReviewsApiError('unexpected', 'Unexpected response shape', null, null);
  }
  return parsed.data;
}

export async function deleteReviewComment(commentId: string): Promise<undefined> {
  let response: Response;
  try {
    response = await fetch(
      `${PANEL_API_BASE_URL}/reviews/comments/${encodeURIComponent(commentId)}`,
      { method: 'DELETE', headers: { ...authHeaders() } },
    );
  } catch (error) {
    throw new ReviewsApiError(
      'network',
      error instanceof Error ? error.message : 'Network request failed',
      null,
      null,
    );
  }
  if (!response.ok) {
    throw toHttpError(response, await readPayload(response));
  }
  return undefined;
}

export async function listApproverGroups(
  limit: number,
  offset: number,
): Promise<ApproverGroupListResponse> {
  const params = new URLSearchParams({ limit: String(limit), offset: String(offset) });
  const payload = await requestJson(
    `${PANEL_API_BASE_URL}/approver-groups?${params.toString()}`,
    { headers: { ...authHeaders() } },
  );
  const parsed = parseApproverGroupListResponse(payload);
  if (parsed === null) {
    throw new ReviewsApiError('unexpected', 'Unexpected response shape', null, null);
  }
  return parsed;
}

export async function getApproverGroup(id: string): Promise<ApproverGroup> {
  const payload = await requestJson(
    `${PANEL_API_BASE_URL}/approver-groups/${encodeURIComponent(id)}`,
    { headers: { ...authHeaders() } },
  );
  const parsed = parseApproverGroupResponse(payload);
  if (parsed === null) {
    throw new ReviewsApiError('unexpected', 'Unexpected response shape', null, null);
  }
  return parsed.data;
}

export async function createApproverGroup(
  request: CreateApproverGroupRequest,
): Promise<ApproverGroup> {
  const payload = await requestJson(`${PANEL_API_BASE_URL}/approver-groups`, {
    method: 'POST',
    headers: { ...authHeaders(), 'Content-Type': 'application/json' },
    body: JSON.stringify(request),
  });
  const parsed = parseApproverGroupResponse(payload);
  if (parsed === null) {
    throw new ReviewsApiError('unexpected', 'Unexpected response shape', null, null);
  }
  return parsed.data;
}

export async function updateApproverGroup(
  id: string,
  request: UpdateApproverGroupRequest,
): Promise<ApproverGroup> {
  const payload = await requestJson(
    `${PANEL_API_BASE_URL}/approver-groups/${encodeURIComponent(id)}`,
    {
      method: 'PATCH',
      headers: { ...authHeaders(), 'Content-Type': 'application/json' },
      body: JSON.stringify(request),
    },
  );
  const parsed = parseApproverGroupResponse(payload);
  if (parsed === null) {
    throw new ReviewsApiError('unexpected', 'Unexpected response shape', null, null);
  }
  return parsed.data;
}

export async function addGroupMember(
  id: string,
  request: AddGroupMemberRequest,
): Promise<ApproverGroup> {
  const payload = await requestJson(
    `${PANEL_API_BASE_URL}/approver-groups/${encodeURIComponent(id)}/members`,
    {
      method: 'POST',
      headers: { ...authHeaders(), 'Content-Type': 'application/json' },
      body: JSON.stringify(request),
    },
  );
  const parsed = parseApproverGroupResponse(payload);
  if (parsed === null) {
    throw new ReviewsApiError('unexpected', 'Unexpected response shape', null, null);
  }
  return parsed.data;
}

export async function removeGroupMember(id: string, userId: string): Promise<ApproverGroup> {
  let response: Response;
  try {
    response = await fetch(
      `${PANEL_API_BASE_URL}/approver-groups/${encodeURIComponent(id)}/members/${encodeURIComponent(userId)}`,
      { method: 'DELETE', headers: { ...authHeaders() } },
    );
  } catch (error) {
    throw new ReviewsApiError(
      'network',
      error instanceof Error ? error.message : 'Network request failed',
      null,
      null,
    );
  }
  if (!response.ok) {
    throw toHttpError(response, await readPayload(response));
  }
  const parsed = parseApproverGroupResponse(await readPayload(response));
  if (parsed === null) {
    throw new ReviewsApiError('unexpected', 'Unexpected response shape', null, null);
  }
  return parsed.data;
}

export async function setExperimenterGroup(
  experimenterId: string,
  request: SetExperimenterGroupRequest,
): Promise<undefined> {
  const payload = await requestJson(
    `${PANEL_API_BASE_URL}/experimenters/${encodeURIComponent(experimenterId)}/group`,
    {
      method: 'PUT',
      headers: { ...authHeaders(), 'Content-Type': 'application/json' },
      body: JSON.stringify(request),
    },
  );
  if (!parseSuccessPayload(payload)) {
    throw new ReviewsApiError('unexpected', 'Unexpected response shape', null, null);
  }
  return undefined;
}
