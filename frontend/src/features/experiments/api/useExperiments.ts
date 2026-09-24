import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import type { UseMutationResult, UseQueryResult } from '@tanstack/react-query';
import {
  EXPERIMENTS_LIST_KEY_PREFIX,
  EXPERIMENTS_PAGE_SIZE,
  completeExperiment,
  createExperiment,
  createExperimentVersion,
  experimentDetailQueryKey,
  experimentsListQueryKey,
  getExperiment,
  listExperiments,
  rolloutExperiment,
  setExperimentVariants,
  transitionExperiment,
  updateExperiment,
} from './experiments';
import type { ExperimentsApiError, ExperimentStatus } from '../types';
import type {
  CompleteExperimentRequest,
  CreateExperimentRequest,
  CreateExperimentVersionRequest,
  Experiment,
  ExperimentListResponse,
  ExperimentTransitionRequest,
  RolloutExperimentRequest,
  SetExperimentVariantsRequest,
  UpdateExperimentRequest,
} from '../types';

export { EXPERIMENTS_PAGE_SIZE };

export interface ExperimentsListParams {
  limit: number;
  offset: number;
  status: ExperimentStatus | null;
}

export function useExperimentsList(
  params: ExperimentsListParams,
): UseQueryResult<ExperimentListResponse, ExperimentsApiError> {
  const { limit, offset, status } = params;
  return useQuery<ExperimentListResponse, ExperimentsApiError>({
    queryKey: experimentsListQueryKey(limit, offset, status),
    queryFn: () => listExperiments({ limit, offset, status }),
    placeholderData: (previous) => previous,
  });
}

export function useExperiment(id: string | null): UseQueryResult<Experiment, ExperimentsApiError> {
  return useQuery<Experiment, ExperimentsApiError>({
    queryKey: id === null ? ['experiments', 'detail', 'none'] : experimentDetailQueryKey(id),
    queryFn: () => getExperiment(id ?? ''),
    enabled: id !== null,
  });
}

function invalidateExperimentsLists(
  queryClient: ReturnType<typeof useQueryClient>,
): Promise<void> {
  return queryClient.invalidateQueries({ queryKey: [EXPERIMENTS_LIST_KEY_PREFIX] });
}

function onExperimentChanged(
  queryClient: ReturnType<typeof useQueryClient>,
  experiment: Experiment,
): void {
  queryClient.setQueryData(experimentDetailQueryKey(experiment.id), experiment);
  void queryClient.invalidateQueries({ queryKey: experimentDetailQueryKey(experiment.id) });
  void invalidateExperimentsLists(queryClient);
}

export function useCreateExperiment(): UseMutationResult<
  Experiment,
  ExperimentsApiError,
  CreateExperimentRequest
> {
  const queryClient = useQueryClient();
  return useMutation<Experiment, ExperimentsApiError, CreateExperimentRequest>({
    mutationFn: createExperiment,
    onSuccess: (experiment) => {
      onExperimentChanged(queryClient, experiment);
    },
  });
}

export interface UpdateExperimentVariables {
  id: string;
  request: UpdateExperimentRequest;
}

export function useUpdateExperiment(): UseMutationResult<
  Experiment,
  ExperimentsApiError,
  UpdateExperimentVariables
> {
  const queryClient = useQueryClient();
  return useMutation<Experiment, ExperimentsApiError, UpdateExperimentVariables>({
    mutationFn: ({ id, request }) => updateExperiment(id, request),
    onSuccess: (experiment) => {
      onExperimentChanged(queryClient, experiment);
    },
  });
}

export interface CreateVersionVariables {
  id: string;
  request: CreateExperimentVersionRequest;
}

export function useCreateExperimentVersion(): UseMutationResult<
  Experiment,
  ExperimentsApiError,
  CreateVersionVariables
> {
  const queryClient = useQueryClient();
  return useMutation<Experiment, ExperimentsApiError, CreateVersionVariables>({
    mutationFn: ({ id, request }) => createExperimentVersion(id, request),
    onSuccess: (experiment) => {
      onExperimentChanged(queryClient, experiment);
    },
  });
}

export interface SetVariantsVariables {
  id: string;
  request: SetExperimentVariantsRequest;
}

export function useSetExperimentVariants(): UseMutationResult<
  Experiment,
  ExperimentsApiError,
  SetVariantsVariables
> {
  const queryClient = useQueryClient();
  return useMutation<Experiment, ExperimentsApiError, SetVariantsVariables>({
    mutationFn: ({ id, request }) => setExperimentVariants(id, request),
    onSuccess: (experiment) => {
      onExperimentChanged(queryClient, experiment);
    },
  });
}

export type TransitionAction =
  | 'submit'
  | 'start'
  | 'pause'
  | 'resume'
  | 'archive'
  | 'internal-pause'
  | 'internal-rollback';

export interface TransitionVariables {
  id: string;
  action: TransitionAction;
  request: ExperimentTransitionRequest;
}

export function useTransitionExperiment(): UseMutationResult<
  Experiment,
  ExperimentsApiError,
  TransitionVariables
> {
  const queryClient = useQueryClient();
  return useMutation<Experiment, ExperimentsApiError, TransitionVariables>({
    mutationFn: ({ id, action, request }) => transitionExperiment(id, action, request),
    onSuccess: (experiment) => {
      onExperimentChanged(queryClient, experiment);
    },
  });
}

export interface CompleteVariables {
  id: string;
  request: CompleteExperimentRequest;
}

export function useCompleteExperiment(): UseMutationResult<
  Experiment,
  ExperimentsApiError,
  CompleteVariables
> {
  const queryClient = useQueryClient();
  return useMutation<Experiment, ExperimentsApiError, CompleteVariables>({
    mutationFn: ({ id, request }) => completeExperiment(id, request),
    onSuccess: (experiment) => {
      onExperimentChanged(queryClient, experiment);
    },
  });
}

export interface RolloutVariables {
  id: string;
  request: RolloutExperimentRequest;
}

export function useRolloutExperiment(): UseMutationResult<
  Experiment,
  ExperimentsApiError,
  RolloutVariables
> {
  const queryClient = useQueryClient();
  return useMutation<Experiment, ExperimentsApiError, RolloutVariables>({
    mutationFn: ({ id, request }) => rolloutExperiment(id, request),
    onSuccess: (experiment) => {
      onExperimentChanged(queryClient, experiment);
    },
  });
}
