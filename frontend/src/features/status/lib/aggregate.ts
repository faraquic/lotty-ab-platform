import type { OverallStatus, ServiceProbeInput, ServiceStatus } from '../types';

const STATUS_OK = 'ok';
const STATUS_UNAVAILABLE = 'unavailable';

export function aggregateServiceStatus(input: ServiceProbeInput): ServiceStatus {
  if (!input.healthOk) {
    return 'down';
  }
  if (input.ready === null) {
    return 'down';
  }

  let degraded = false;
  for (const component of Object.values(input.ready.components)) {
    if (component.status === STATUS_OK) {
      continue;
    }
    if (component.status === STATUS_UNAVAILABLE) {
      if (component.criticality === 'required') {
        return 'down';
      }
      degraded = true;
      continue;
    }
    degraded = true;
  }
  return degraded ? 'degraded' : 'healthy';
}

export function aggregateOverallStatus(statuses: ServiceStatus[]): OverallStatus {
  if (statuses.includes('down')) {
    return 'outage';
  }
  if (statuses.includes('degraded')) {
    return 'partial';
  }
  return 'operational';
}
