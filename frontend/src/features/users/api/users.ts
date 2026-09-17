import { clearAuthSession, getAuthToken } from '@/features/auth/lib/authSession';
import { PANEL_API_BASE_URL } from '@/shared/config/api';
import {
  UsersApiError,
  parseErrorPayload,
  parseUserListResponse,
  parseUserResponse,
} from '../types';
import type {
  CreateUserRequest,
  UpdateUserRequest,
  User,
  UserListResponse,
} from '../types';

export const USERS_PAGE_SIZE = 20;

export function usersListQueryKey(limit: number, offset: number): [string, { limit: number; offset: number }] {
  return ['users', { limit, offset }];
}

export const USERS_LIST_KEY_PREFIX = 'users';

export const ME_QUERY_KEY = ['me'] as const;

export function userDetailQueryKey(id: string): [string, string] {
  return ['users', id];
}

function authHeaders(): Record<string, string> {
  const token = getAuthToken();
  if (token === null || token.length === 0) {
    throw new UsersApiError('unexpected', 'Missing auth token', null, null);
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

function toHttpError(response: Response, payload: unknown): UsersApiError {
  const { code, message } = parseErrorPayload(payload);
  if (response.status === 401) {
    clearAuthSession();
  }
  return new UsersApiError(
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
    throw new UsersApiError(
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

export async function listUsers(limit: number, offset: number): Promise<UserListResponse> {
  const params = new URLSearchParams({
    limit: String(limit),
    offset: String(offset),
  });
  const payload = await requestJson(`${PANEL_API_BASE_URL}/users?${params.toString()}`, {
    headers: { ...authHeaders() },
  });
  const parsed = parseUserListResponse(payload);
  if (parsed === null) {
    throw new UsersApiError('unexpected', 'Unexpected response shape', null, null);
  }
  return parsed;
}

export async function getUser(id: string): Promise<User> {
  const payload = await requestJson(
    `${PANEL_API_BASE_URL}/users/${encodeURIComponent(id)}`,
    { headers: { ...authHeaders() } },
  );
  const parsed = parseUserResponse(payload);
  if (parsed === null) {
    throw new UsersApiError('unexpected', 'Unexpected response shape', null, null);
  }
  return parsed.data;
}

export async function getMe(): Promise<User> {
  const payload = await requestJson(`${PANEL_API_BASE_URL}/me`, {
    headers: { ...authHeaders() },
  });
  const parsed = parseUserResponse(payload);
  if (parsed === null) {
    throw new UsersApiError('unexpected', 'Unexpected response shape', null, null);
  }
  return parsed.data;
}

export async function createUser(request: CreateUserRequest): Promise<User> {
  const payload = await requestJson(`${PANEL_API_BASE_URL}/users`, {
    method: 'POST',
    headers: { ...authHeaders(), 'Content-Type': 'application/json' },
    body: JSON.stringify(request),
  });
  const parsed = parseUserResponse(payload);
  if (parsed === null) {
    throw new UsersApiError('unexpected', 'Unexpected response shape', null, null);
  }
  return parsed.data;
}

export async function updateUser(id: string, request: UpdateUserRequest): Promise<User> {
  const payload = await requestJson(
    `${PANEL_API_BASE_URL}/users/${encodeURIComponent(id)}`,
    {
      method: 'PATCH',
      headers: { ...authHeaders(), 'Content-Type': 'application/json' },
      body: JSON.stringify(request),
    },
  );
  const parsed = parseUserResponse(payload);
  if (parsed === null) {
    throw new UsersApiError('unexpected', 'Unexpected response shape', null, null);
  }
  return parsed.data;
}

export async function deleteUser(id: string): Promise<void> {
  let response: Response;
  try {
    response = await fetch(`${PANEL_API_BASE_URL}/users/${encodeURIComponent(id)}`, {
      method: 'DELETE',
      headers: { ...authHeaders() },
    });
  } catch (error) {
    throw new UsersApiError(
      'network',
      error instanceof Error ? error.message : 'Network request failed',
      null,
      null,
    );
  }
  if (!response.ok) {
    throw toHttpError(response, await readPayload(response));
  }
}

export async function uploadUserAvatar(id: string, file: File | null): Promise<User> {
  const form = new FormData();
  if (file !== null) {
    form.append('avatar', file);
  }
  let response: Response;
  try {
    response = await fetch(
      `${PANEL_API_BASE_URL}/users/${encodeURIComponent(id)}/avatar`,
      {
        method: 'POST',
        headers: { ...authHeaders() },
        body: form,
      },
    );
  } catch (error) {
    throw new UsersApiError(
      'network',
      error instanceof Error ? error.message : 'Network request failed',
      null,
      null,
    );
  }
  if (!response.ok) {
    throw toHttpError(response, await readPayload(response));
  }
  const parsed = parseUserResponse(await readPayload(response));
  if (parsed === null) {
    throw new UsersApiError('unexpected', 'Unexpected response shape', null, null);
  }
  return parsed.data;
}
