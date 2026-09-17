import { useQuery } from '@tanstack/react-query';
import type { UseQueryResult } from '@tanstack/react-query';
import { probeService } from './probes';
import type { ServiceName, ServiceProbe } from '../types';

export const STATUS_POLL_INTERVAL_MS = 20_000;

export function useServiceStatus(
  service: ServiceName,
  baseUrl: string,
): UseQueryResult<ServiceProbe, never> {
  return useQuery<ServiceProbe, never>({
    queryKey: ['service-status', service],
    queryFn: () => probeService(baseUrl, service),
    refetchInterval: STATUS_POLL_INTERVAL_MS,
    placeholderData: (previous) => previous,
    retry: false,
  });
}
