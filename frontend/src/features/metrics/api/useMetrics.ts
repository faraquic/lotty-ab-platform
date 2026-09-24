import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import type { UseMutationResult, UseQueryResult } from '@tanstack/react-query';
import {
  METRICS_LIST_KEY_PREFIX,
  METRICS_PAGE_SIZE,
  createMetric,
  getMetric,
  listMetrics,
  metricDetailQueryKey,
  metricsListQueryKey,
  updateMetric,
} from './metrics';
import type { MetricsApiError } from '../types';
import type {
  CreateMetricRequest,
  Metric,
  MetricListResponse,
  UpdateMetricRequest,
} from '../types';

export { METRICS_PAGE_SIZE };

export interface MetricsListParams {
  limit: number;
  offset: number;
  archived: boolean;
}

export function useMetricsList(
  params: MetricsListParams,
): UseQueryResult<MetricListResponse, MetricsApiError> {
  const { limit, offset, archived } = params;
  return useQuery<MetricListResponse, MetricsApiError>({
    queryKey: metricsListQueryKey(limit, offset, archived),
    queryFn: () => listMetrics({ limit, offset, archived }),
    placeholderData: (previous) => previous,
  });
}

export function useMetric(
  id: string | null,
  archived = false,
): UseQueryResult<Metric, MetricsApiError> {
  return useQuery<Metric, MetricsApiError>({
    queryKey: id === null ? ['metrics', 'detail', 'none'] : metricDetailQueryKey(id),
    queryFn: () => getMetric(id ?? '', archived),
    enabled: id !== null,
  });
}

function invalidateMetricsLists(queryClient: ReturnType<typeof useQueryClient>): Promise<void> {
  return queryClient.invalidateQueries({ queryKey: [METRICS_LIST_KEY_PREFIX] });
}

export function useCreateMetric(): UseMutationResult<Metric, MetricsApiError, CreateMetricRequest> {
  const queryClient = useQueryClient();
  return useMutation<Metric, MetricsApiError, CreateMetricRequest>({
    mutationFn: createMetric,
    onSuccess: (metric) => {
      queryClient.setQueryData(metricDetailQueryKey(metric.id), metric);
      void invalidateMetricsLists(queryClient);
    },
  });
}

export interface UpdateMetricVariables {
  id: string;
  request: UpdateMetricRequest;
}

export function useUpdateMetric(): UseMutationResult<Metric, MetricsApiError, UpdateMetricVariables> {
  const queryClient = useQueryClient();
  return useMutation<Metric, MetricsApiError, UpdateMetricVariables>({
    mutationFn: ({ id, request }) => updateMetric(id, request),
    onSuccess: (metric) => {
      queryClient.setQueryData(metricDetailQueryKey(metric.id), metric);
      void queryClient.invalidateQueries({ queryKey: metricDetailQueryKey(metric.id) });
      void invalidateMetricsLists(queryClient);
    },
  });
}
