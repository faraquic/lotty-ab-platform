import { afterEach, describe, expect, it, vi } from 'vitest';
import { probeService } from './probes';

interface StubResponse {
  ok: boolean;
  status: number;
  json: () => Promise<unknown>;
}

function stubFetch(handler: (url: string, init: RequestInit) => Promise<StubResponse> | StubResponse): void {
  vi.stubGlobal('fetch', (url: unknown, init: unknown) =>
    handler(url as string, init as RequestInit),
  );
}

function hanging(init: RequestInit): Promise<StubResponse> {
  return new Promise<StubResponse>((_resolve, reject) => {
    init.signal?.addEventListener('abort', () => {
      reject(new DOMException('The operation was aborted.', 'AbortError'));
    });
  });
}

function jsonResponse(status: number, payload: unknown): StubResponse {
  return {
    ok: status >= 200 && status < 300,
    status,
    json: () => Promise.resolve(payload),
  };
}

function readyBody(overrides: Record<string, unknown> = {}): unknown {
  return {
    service: 'labp-panel',
    version: 'dev',
    environment: 'local',
    timestamp: '2026-09-17T07:55:24.211Z',
    components: {
      service: { status: 'ok', criticality: 'required' },
      ...overrides,
    },
  };
}

afterEach(() => {
  vi.unstubAllGlobals();
});

describe('probeService', () => {
  it('returns healthy with latency when health succeeds', async () => {
    stubFetch(() => jsonResponse(200, readyBody()));

    const probe = await probeService('http://x', 'panel');
    expect(probe.status).toBe('healthy');
    expect(probe.failure).toBeNull();
    expect(probe.ready?.service).toBe('labp-panel');
    expect(typeof probe.latencyMs).toBe('number');
  });

  it('requests the health endpoint only', async () => {
    const seen: string[] = [];
    stubFetch((url) => {
      seen.push(url);
      return jsonResponse(200, readyBody());
    });

    await probeService('http://x', 'panel');
    expect(seen).toHaveLength(1);
    expect(seen[0]?.endsWith('/health')).toBe(true);
  });

  it('returns down on network error', async () => {
    stubFetch(() => Promise.reject(new TypeError('Failed to fetch')));

    const probe = await probeService('http://x', 'panel');
    expect(probe.status).toBe('down');
    expect(probe.failure?.reason).toBe('network');
    expect(probe.ready).toBeNull();
  });

  it('returns down on timeout', async () => {
    stubFetch((_url, init) => hanging(init));

    const probe = await probeService('http://x', 'panel');
    expect(probe.status).toBe('down');
    expect(probe.failure?.reason).toBe('timeout');
  }, 10_000);

  it('returns down with http status on 500', async () => {
    stubFetch(() => ({ ok: false, status: 500, json: () => Promise.resolve(null) }));

    const probe = await probeService('http://x', 'panel');
    expect(probe.status).toBe('down');
    expect(probe.failure).toEqual({ reason: 'http', httpStatus: 500 });
  });

  it('returns down on invalid JSON', async () => {
    stubFetch(() => ({
      ok: true,
      status: 200,
      json: () => Promise.reject(new SyntaxError('Unexpected token')),
    }));

    const probe = await probeService('http://x', 'analytics');
    expect(probe.status).toBe('down');
    expect(probe.failure?.reason).toBe('invalid');
  });

  it('marks required failure as down on 503', async () => {
    stubFetch(() =>
      jsonResponse(
        503,
        readyBody({
          database: { status: 'unavailable', criticality: 'required', message: 'database unavailable' },
        }),
      ),
    );

    const probe = await probeService('http://x', 'panel');
    expect(probe.status).toBe('down');
    expect(probe.ready?.components['database']?.status).toBe('unavailable');
  });

  it('marks optional-only failure as degraded even on 200', async () => {
    stubFetch(() =>
      jsonResponse(
        200,
        readyBody({
          storage: { status: 'unavailable', criticality: 'optional', message: 'storage unavailable' },
        }),
      ),
    );

    const probe = await probeService('http://x', 'panel');
    expect(probe.status).toBe('degraded');
    expect(probe.failure).toBeNull();
  });
});
