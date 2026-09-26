import { clearAuthSession, getAuthToken } from '@/features/auth/lib/authSession';
import { PANEL_API_BASE_URL } from '@/shared/config/api';
import {
  ExperimentsApiError,
  parseErrorPayload,
  parseExperimentListResponse,
  parseExperimentResponse,
} from '../types';
import type {
  CompleteExperimentRequest,
  CreateExperimentRequest,
  CreateExperimentVersionRequest,
  Experiment,
  ExperimentListResponse,
  ExperimentStatus,
  ExperimentTransitionRequest,
  RolloutExperimentRequest,
  SetExperimentVariantsRequest,
  UpdateExperimentRequest,
} from '../types';

export const EXPERIMENTS_PAGE_SIZE = 20;

export interface ExperimentsListFilters {
  limit: number;
  offset: number;
  status: ExperimentStatus | null;
  q?: string;
  flagId?: string;
}

export function experimentsListQueryKey(
  filters: ExperimentsListFilters,
): [string, ExperimentsListFilters] {
  return ['experiments', filters];
}

export const EXPERIMENTS_LIST_KEY_PREFIX = 'experiments';

export function experimentDetailQueryKey(id: string): [string, string] {
  return ['experiments', id];
}

function authHeaders(): Record<string, string> {
  const token = getAuthToken();
  if (token === null || token.length === 0) {
    throw new ExperimentsApiError('unexpected', 'Missing auth token', null, null);
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

function toHttpError(response: Response, payload: unknown): ExperimentsApiError {
  const { code, message } = parseErrorPayload(payload);
  if (response.status === 401) {
    clearAuthSession();
  }
  return new ExperimentsApiError(
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
    throw new ExperimentsApiError(
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

function parseSingle(payload: unknown): Experiment {
  const parsed = parseExperimentResponse(payload);
  if (parsed === null) {
    throw new ExperimentsApiError('unexpected', 'Unexpected response shape', null, null);
  }
  return parsed.data;
}

export type ListExperimentsParams = ExperimentsListFilters;

export async function listExperiments(params: ExperimentsListFilters): Promise<ExperimentListResponse> {
  const query = new URLSearchParams({
    limit: String(params.limit),
    offset: String(params.offset),
  });
  if (params.status !== null) {
    query.set('status', params.status);
  }
  if (params.q !== undefined && params.q.length > 0) {
    query.set('q', params.q);
  }
  if (params.flagId !== undefined && params.flagId.length > 0) {
    query.set('flag_id', params.flagId);
  }
  const payload = await requestJson(`${PANEL_API_BASE_URL}/experiments?${query.toString()}`, {
    headers: { ...authHeaders() },
  });
  const parsed = parseExperimentListResponse(payload);
  if (parsed === null) {
    throw new ExperimentsApiError('unexpected', 'Unexpected response shape', null, null);
  }
  return parsed;
}

export async function getExperiment(id: string): Promise<Experiment> {
  return parseSingle(
    await requestJson(`${PANEL_API_BASE_URL}/experiments/${encodeURIComponent(id)}`, {
      headers: { ...authHeaders() },
    }),
  );
}

function postJson(path: string, body: unknown): Promise<unknown> {
  return requestJson(`${PANEL_API_BASE_URL}${path}`, {
    method: 'POST',
    headers: { ...authHeaders(), 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  });
}

export async function createExperiment(request: CreateExperimentRequest): Promise<Experiment> {
  return parseSingle(await postJson('/experiments', request));
}

export async function updateExperiment(
  id: string,
  request: UpdateExperimentRequest,
): Promise<Experiment> {
  const payload = await requestJson(
    `${PANEL_API_BASE_URL}/experiments/${encodeURIComponent(id)}`,
    {
      method: 'PATCH',
      headers: { ...authHeaders(), 'Content-Type': 'application/json' },
      body: JSON.stringify(request),
    },
  );
  return parseSingle(payload);
}

export async function createExperimentVersion(
  id: string,
  request: CreateExperimentVersionRequest,
): Promise<Experiment> {
  return parseSingle(await postJson(`/experiments/${encodeURIComponent(id)}/versions`, request));
}

export async function setExperimentVariants(
  id: string,
  request: SetExperimentVariantsRequest,
): Promise<Experiment> {
  const payload = await requestJson(
    `${PANEL_API_BASE_URL}/experiments/${encodeURIComponent(id)}/variants`,
    {
      method: 'PUT',
      headers: { ...authHeaders(), 'Content-Type': 'application/json' },
      body: JSON.stringify(request),
    },
  );
  return parseSingle(payload);
}

export async function transitionExperiment(
  id: string,
  action:
    | 'submit'
    | 'start'
    | 'pause'
    | 'resume'
    | 'archive'
    | 'internal-pause'
    | 'internal-rollback',
  request: ExperimentTransitionRequest,
): Promise<Experiment> {
  const path =
    action === 'internal-pause'
      ? `/internal/experiments/${encodeURIComponent(id)}/pause`
      : action === 'internal-rollback'
        ? `/internal/experiments/${encodeURIComponent(id)}/rollback`
        : `/experiments/${encodeURIComponent(id)}/${action}`;
  return parseSingle(await postJson(path, request));
}

export async function completeExperiment(
  id: string,
  request: CompleteExperimentRequest,
): Promise<Experiment> {
  return parseSingle(await postJson(`/experiments/${encodeURIComponent(id)}/complete`, request));
}

export async function rolloutExperiment(
  id: string,
  request: RolloutExperimentRequest,
): Promise<Experiment> {
  return parseSingle(await postJson(`/experiments/${encodeURIComponent(id)}/rollout`, request));
}
