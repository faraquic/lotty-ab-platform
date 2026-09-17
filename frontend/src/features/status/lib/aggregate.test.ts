import { describe, expect, it } from 'vitest';
import { aggregateOverallStatus, aggregateServiceStatus } from './aggregate';
import type { ReadyPayload, ServiceProbeInput } from '../types';

function readyWith(
  components: ReadyPayload['components'],
): ReadyPayload {
  return {
    service: 'labp-panel',
    version: 'dev',
    environment: 'local',
    timestamp: '2026-09-17T07:55:24.211Z',
    components,
  };
}

function input(components: ReadyPayload['components']): ServiceProbeInput {
  return { healthOk: true, ready: readyWith(components) };
}

describe('aggregateServiceStatus', () => {
  it('returns healthy when all components are ok', () => {
    expect(
      aggregateServiceStatus(
        input({
          service: { status: 'ok', criticality: 'required', message: null },
          database: { status: 'ok', criticality: 'required', message: null },
          storage: { status: 'ok', criticality: 'optional', message: null },
        }),
      ),
    ).toBe('healthy');
  });

  it('returns down when a required component is unhealthy', () => {
    expect(
      aggregateServiceStatus(
        input({
          service: { status: 'ok', criticality: 'required', message: null },
          database: { status: 'unavailable', criticality: 'required', message: 'database unavailable' },
          storage: { status: 'ok', criticality: 'optional', message: null },
        }),
      ),
    ).toBe('down');
  });

  it('returns degraded when an optional component is unhealthy', () => {
    expect(
      aggregateServiceStatus(
        input({
          service: { status: 'ok', criticality: 'required', message: null },
          storage: { status: 'unavailable', criticality: 'optional', message: 'storage unavailable' },
        }),
      ),
    ).toBe('degraded');
  });

  it('returns degraded (not down) on multiple optional failures', () => {
    expect(
      aggregateServiceStatus(
        input({
          service: { status: 'ok', criticality: 'required', message: null },
          cache: { status: 'unavailable', criticality: 'optional', message: 'cache unavailable' },
          storage: { status: 'unavailable', criticality: 'optional', message: 'storage unavailable' },
        }),
      ),
    ).toBe('degraded');
  });

  it('returns down on multiple required failures', () => {
    expect(
      aggregateServiceStatus(
        input({
          database: { status: 'unavailable', criticality: 'required', message: null },
          cache: { status: 'unavailable', criticality: 'required', message: null },
          storage: { status: 'unavailable', criticality: 'optional', message: null },
        }),
      ),
    ).toBe('down');
  });

  it('returns degraded on unknown component state', () => {
    expect(
      aggregateServiceStatus(
        input({
          service: { status: 'ok', criticality: 'required', message: null },
          database: { status: 'starting', criticality: 'required', message: null },
        }),
      ),
    ).toBe('degraded');
  });

  it('returns down when health probe failed', () => {
    expect(aggregateServiceStatus({ healthOk: false, ready: null })).toBe('down');
  });

  it('returns down when ready payload is missing', () => {
    expect(aggregateServiceStatus({ healthOk: true, ready: null })).toBe('down');
  });
});

describe('aggregateOverallStatus', () => {
  it('returns operational when all services are healthy', () => {
    expect(aggregateOverallStatus(['healthy', 'healthy', 'healthy'])).toBe('operational');
  });

  it('returns partial on degraded without down', () => {
    expect(aggregateOverallStatus(['healthy', 'degraded', 'healthy'])).toBe('partial');
  });

  it('returns outage when any service is down', () => {
    expect(aggregateOverallStatus(['healthy', 'down', 'healthy'])).toBe('outage');
  });

  it('returns outage (not partial) on mixed degraded and down', () => {
    expect(aggregateOverallStatus(['degraded', 'down', 'healthy'])).toBe('outage');
  });

  it('does not hide healthy services behind one down', () => {
    const statuses = ['healthy', 'down', 'healthy'] as const;
    expect(aggregateOverallStatus([...statuses])).toBe('outage');
    expect(statuses.filter((status) => status === 'healthy')).toHaveLength(2);
  });
});
