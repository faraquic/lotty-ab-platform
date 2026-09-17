export type ServiceName = 'panel' | 'runtime' | 'analytics';

export type ServiceStatus = 'healthy' | 'degraded' | 'down';

export type OverallStatus = 'operational' | 'partial' | 'outage';

export type ComponentCriticality = 'required' | 'optional';

export interface ReadyComponent {
  status: string;
  criticality: ComponentCriticality;
  message: string | null;
}

export interface ReadyPayload {
  service: string;
  version: string;
  environment: string;
  timestamp: string;
  components: Record<string, ReadyComponent>;
}

export type ProbeFailureReason = 'network' | 'timeout' | 'http' | 'invalid';

export interface ProbeFailure {
  reason: ProbeFailureReason;
  httpStatus: number | null;
}

export interface ServiceProbe {
  service: ServiceName;
  status: ServiceStatus;
  latencyMs: number | null;
  checkedAt: number;
  ready: ReadyPayload | null;
  failure: ProbeFailure | null;
}

export interface ServiceProbeInput {
  healthOk: boolean;
  ready: ReadyPayload | null;
}
