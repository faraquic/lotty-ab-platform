import { clearAuthSession, getAuthToken } from '@/features/auth/lib/authSession';
import { PANEL_API_BASE_URL } from '@/shared/config/api';
import {
  FlagsApiError,
  parseErrorPayload,
  parseFlagListResponse,
  parseFlagResponse,
} from '../types';
import type {
  CreateFlagRequest,
  Flag,
  FlagListResponse,
  UpdateFlagRequest,
} from '../types';

export const FLAGS_PAGE_SIZE = 20;

export function flagsListQueryKey(
  limit: number,
  offset: number,
): [string, { limit: number; offset: number }] {
  return ['flags', { limit, offset }];
}

export const FLAGS_LIST_KEY_PREFIX = 'flags';

export function flagDetailQueryKey(id: string): [string, string] {
  return ['flags', id];
}

function authHeaders(): Record<string, string> {
  const token = getAuthToken();
  if (token === null || token.length === 0) {
    throw new FlagsApiError('unexpected', 'Missing auth token', null, null);
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

function toHttpError(response: Response, payload: unknown): FlagsApiError {
  const { code, message } = parseErrorPayload(payload);
  if (response.status === 401) {
    clearAuthSession();
  }
  return new FlagsApiError(
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
    throw new FlagsApiError(
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

export async function listFlags(limit: number, offset: number): Promise<FlagListResponse> {
  const params = new URLSearchParams({
    limit: String(limit),
    offset: String(offset),
  });
  const payload = await requestJson(`${PANEL_API_BASE_URL}/flags?${params.toString()}`, {
    headers: { ...authHeaders() },
  });
  const parsed = parseFlagListResponse(payload);
  if (parsed === null) {
    throw new FlagsApiError('unexpected', 'Unexpected response shape', null, null);
  }
  return parsed;
}

export async function getFlag(id: string): Promise<Flag> {
  const payload = await requestJson(
    `${PANEL_API_BASE_URL}/flags/${encodeURIComponent(id)}`,
    { headers: { ...authHeaders() } },
  );
  const parsed = parseFlagResponse(payload);
  if (parsed === null) {
    throw new FlagsApiError('unexpected', 'Unexpected response shape', null, null);
  }
  return parsed.data;
}

export async function createFlag(request: CreateFlagRequest): Promise<Flag> {
  const payload = await requestJson(`${PANEL_API_BASE_URL}/flags`, {
    method: 'POST',
    headers: { ...authHeaders(), 'Content-Type': 'application/json' },
    body: JSON.stringify(request),
  });
  const parsed = parseFlagResponse(payload);
  if (parsed === null) {
    throw new FlagsApiError('unexpected', 'Unexpected response shape', null, null);
  }
  return parsed.data;
}

export async function updateFlag(id: string, request: UpdateFlagRequest): Promise<Flag> {
  const payload = await requestJson(
    `${PANEL_API_BASE_URL}/flags/${encodeURIComponent(id)}`,
    {
      method: 'PATCH',
      headers: { ...authHeaders(), 'Content-Type': 'application/json' },
      body: JSON.stringify(request),
    },
  );
  const parsed = parseFlagResponse(payload);
  if (parsed === null) {
    throw new FlagsApiError('unexpected', 'Unexpected response shape', null, null);
  }
  return parsed.data;
}

export async function deleteFlag(id: string): Promise<undefined> {
  let response: Response;
  try {
    response = await fetch(`${PANEL_API_BASE_URL}/flags/${encodeURIComponent(id)}`, {
      method: 'DELETE',
      headers: { ...authHeaders() },
    });
  } catch (error) {
    throw new FlagsApiError(
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
