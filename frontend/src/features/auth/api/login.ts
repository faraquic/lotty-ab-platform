import { PANEL_API_BASE_URL } from '@/shared/config/api';
import { LoginApiError } from '../types';
import type { LoginData, LoginRequest } from '../types';

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

function parseLoginData(payload: unknown): LoginData | null {
  if (!isRecord(payload)) {
    return null;
  }
  if (payload['success'] !== true) {
    return null;
  }
  const data = payload['data'];
  if (!isRecord(data)) {
    return null;
  }
  const token = data['token'];
  const expiresAt = data['expires_at'];
  if (typeof token !== 'string' || token.length === 0) {
    return null;
  }
  if (typeof expiresAt !== 'string' || expiresAt.length === 0) {
    return null;
  }
  return { token, expires_at: expiresAt };
}

function parseErrorPayload(payload: unknown): { code: string | null; message: string | null } {
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

export async function loginRequest(credentials: LoginRequest): Promise<LoginData> {
  let response: Response;
  try {
    response = await fetch(`${PANEL_API_BASE_URL}/login`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(credentials),
    });
  } catch (error) {
    throw new LoginApiError(
      'network',
      error instanceof Error ? error.message : 'Network request failed',
      null,
      null,
    );
  }

  let payload: unknown = null;
  try {
    payload = (await response.json()) as unknown;
  } catch {
    payload = null;
  }

  if (!response.ok) {
    const { code, message } = parseErrorPayload(payload);
    throw new LoginApiError(
      'http',
      message ?? `Request failed with status ${String(response.status)}`,
      response.status,
      code,
    );
  }

  const data = parseLoginData(payload);
  if (data === null) {
    throw new LoginApiError('unexpected', 'Unexpected response shape', response.status, null);
  }
  return data;
}
