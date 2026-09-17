import { aggregateServiceStatus } from '../lib/aggregate';
import type {
  ReadyComponent,
  ReadyPayload,
  ServiceName,
  ServiceProbe,
  ServiceProbeInput,
} from '../types';

export const PROBE_TIMEOUT_MS = 5_000;

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

function parseComponent(value: unknown): ReadyComponent | null {
  if (!isRecord(value)) {
    return null;
  }
  const status = value['status'];
  const criticality = value['criticality'];
  const message = value['message'];
  if (typeof status !== 'string') {
    return null;
  }
  if (criticality !== 'required' && criticality !== 'optional') {
    return null;
  }
  if (message !== undefined && message !== null && typeof message !== 'string') {
    return null;
  }
  return {
    status,
    criticality,
    message: typeof message === 'string' ? message : null,
  };
}

function parseReadyPayload(payload: unknown): ReadyPayload | null {
  if (!isRecord(payload)) {
    return null;
  }
  const service = payload['service'];
  const version = payload['version'];
  const environment = payload['environment'];
  const timestamp = payload['timestamp'];
  const components = payload['components'];
  if (
    typeof service !== 'string' ||
    typeof version !== 'string' ||
    typeof environment !== 'string' ||
    typeof timestamp !== 'string' ||
    !isRecord(components)
  ) {
    return null;
  }
  const parsed: Record<string, ReadyComponent> = {};
  for (const [name, raw] of Object.entries(components)) {
    const component = parseComponent(raw);
    if (component === null) {
      return null;
    }
    parsed[name] = component;
  }
  return { service, version, environment, timestamp, components: parsed };
}

function isAbortError(error: unknown): boolean {
  return error instanceof DOMException && error.name === 'AbortError';
}

async function fetchHealthStatus(baseUrl: string): Promise<number> {
  const response = await fetch(`${baseUrl}/health`, {
    signal: AbortSignal.timeout(PROBE_TIMEOUT_MS),
  });
  return response.status;
}

async function fetchReady(
  baseUrl: string,
): Promise<{ status: number; ready: ReadyPayload | null }> {
  const response = await fetch(`${baseUrl}/ready`, {
    signal: AbortSignal.timeout(PROBE_TIMEOUT_MS),
  });
  let payload: unknown = null;
  try {
    payload = (await response.json()) as unknown;
  } catch {
    payload = null;
  }
  return { status: response.status, ready: parseReadyPayload(payload) };
}

export async function probeService(baseUrl: string, service: ServiceName): Promise<ServiceProbe> {
  const startedAt = performance.now();
  const finish = (partial: Omit<ServiceProbe, 'service' | 'latencyMs' | 'checkedAt'>): ServiceProbe => ({
    service,
    latencyMs: Math.round(performance.now() - startedAt),
    checkedAt: Date.now(),
    ...partial,
  });

  let healthStatus: number;
  try {
    healthStatus = await fetchHealthStatus(baseUrl);
  } catch (error) {
    return finish({
      status: 'down',
      ready: null,
      failure: { reason: isAbortError(error) ? 'timeout' : 'network', httpStatus: null },
    });
  }
  if (healthStatus < 200 || healthStatus >= 300) {
    return finish({
      status: 'down',
      ready: null,
      failure: { reason: 'http', httpStatus: healthStatus },
    });
  }

  let readyStatus: number;
  let ready: ReadyPayload | null;
  try {
    ({ status: readyStatus, ready } = await fetchReady(baseUrl));
  } catch (error) {
    return finish({
      status: 'down',
      ready: null,
      failure: { reason: isAbortError(error) ? 'timeout' : 'network', httpStatus: null },
    });
  }

  const readyOk = readyStatus >= 200 && readyStatus < 300;
  if (ready === null) {
    return finish({
      status: 'down',
      ready: null,
      failure: readyOk
        ? { reason: 'invalid', httpStatus: null }
        : { reason: 'http', httpStatus: readyStatus },
    });
  }
  const input: ServiceProbeInput = { healthOk: true, ready };
  return finish({
    status: aggregateServiceStatus(input),
    ready,
    failure: null,
  });
}
