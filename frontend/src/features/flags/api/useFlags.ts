import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import type { UseMutationResult, UseQueryResult } from '@tanstack/react-query';
import {
  FLAGS_LIST_KEY_PREFIX,
  FLAGS_PAGE_SIZE,
  createFlag,
  deleteFlag,
  flagDetailQueryKey,
  flagsListQueryKey,
  getFlag,
  listFlags,
  updateFlag,
} from './flags';
import type { FlagsApiError } from '../types';
import type {
  CreateFlagRequest,
  Flag,
  FlagListResponse,
  UpdateFlagRequest,
} from '../types';

export { FLAGS_PAGE_SIZE };

export interface FlagsListParams {
  limit: number;
  offset: number;
}

export function useFlagsList(params: FlagsListParams): UseQueryResult<FlagListResponse, FlagsApiError> {
  const { limit, offset } = params;
  return useQuery<FlagListResponse, FlagsApiError>({
    queryKey: flagsListQueryKey(limit, offset),
    queryFn: () => listFlags(limit, offset),
    placeholderData: (previous) => previous,
  });
}

export function useFlag(id: string | null): UseQueryResult<Flag, FlagsApiError> {
  return useQuery<Flag, FlagsApiError>({
    queryKey: id === null ? ['flags', 'detail', 'none'] : flagDetailQueryKey(id),
    queryFn: () => getFlag(id ?? ''),
    enabled: id !== null,
  });
}

function invalidateFlagsLists(queryClient: ReturnType<typeof useQueryClient>): Promise<void> {
  return queryClient.invalidateQueries({ queryKey: [FLAGS_LIST_KEY_PREFIX] });
}

export function useCreateFlag(): UseMutationResult<Flag, FlagsApiError, CreateFlagRequest> {
  const queryClient = useQueryClient();
  return useMutation<Flag, FlagsApiError, CreateFlagRequest>({
    mutationFn: createFlag,
    onSuccess: () => {
      void invalidateFlagsLists(queryClient);
    },
  });
}

export interface UpdateFlagVariables {
  id: string;
  request: UpdateFlagRequest;
}

export function useUpdateFlag(): UseMutationResult<Flag, FlagsApiError, UpdateFlagVariables> {
  const queryClient = useQueryClient();
  return useMutation<Flag, FlagsApiError, UpdateFlagVariables>({
    mutationFn: ({ id, request }) => updateFlag(id, request),
    onSuccess: (flag) => {
      queryClient.setQueryData(flagDetailQueryKey(flag.id), flag);
      void queryClient.invalidateQueries({ queryKey: flagDetailQueryKey(flag.id) });
      void invalidateFlagsLists(queryClient);
    },
  });
}

export function useDeleteFlag(): UseMutationResult<undefined, FlagsApiError, string> {
  const queryClient = useQueryClient();
  return useMutation<undefined, FlagsApiError, string>({
    mutationFn: (id: string): Promise<undefined> => deleteFlag(id),
    onSuccess: (_data, id) => {
      queryClient.removeQueries({ queryKey: flagDetailQueryKey(id) });
      void invalidateFlagsLists(queryClient);
    },
  });
}
