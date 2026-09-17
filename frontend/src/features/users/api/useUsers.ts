import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import type { UseMutationResult, UseQueryResult } from '@tanstack/react-query';
import {
  ME_QUERY_KEY,
  USERS_LIST_KEY_PREFIX,
  USERS_PAGE_SIZE,
  createUser,
  deleteUser,
  getMe,
  getUser,
  listUsers,
  updateUser,
  uploadUserAvatar,
  userDetailQueryKey,
  usersListQueryKey,
} from './users';
import type { UsersApiError } from '../types';
import type {
  CreateUserRequest,
  UpdateUserRequest,
  User,
  UserListResponse,
} from '../types';

export { USERS_PAGE_SIZE };

export interface UsersListParams {
  limit: number;
  offset: number;
}

export function useUsersList(params: UsersListParams): UseQueryResult<UserListResponse, UsersApiError> {
  const { limit, offset } = params;
  return useQuery<UserListResponse, UsersApiError>({
    queryKey: usersListQueryKey(limit, offset),
    queryFn: () => listUsers(limit, offset),
    placeholderData: (previous) => previous,
  });
}

export function useUser(id: string | null): UseQueryResult<User, UsersApiError> {
  return useQuery<User, UsersApiError>({
    queryKey: id === null ? ['users', 'detail', 'none'] : userDetailQueryKey(id),
    queryFn: () => getUser(id ?? ''),
    enabled: id !== null,
  });
}

export function useMe(): UseQueryResult<User, UsersApiError> {
  return useQuery<User, UsersApiError>({
    queryKey: ME_QUERY_KEY,
    queryFn: getMe,
    staleTime: 60_000,
  });
}

function invalidateUsersLists(queryClient: ReturnType<typeof useQueryClient>): Promise<void> {
  return queryClient.invalidateQueries({ queryKey: [USERS_LIST_KEY_PREFIX] });
}

export function useCreateUser(): UseMutationResult<User, UsersApiError, CreateUserRequest> {
  const queryClient = useQueryClient();
  return useMutation<User, UsersApiError, CreateUserRequest>({
    mutationFn: createUser,
    onSuccess: () => {
      void invalidateUsersLists(queryClient);
    },
  });
}

export interface UpdateUserVariables {
  id: string;
  request: UpdateUserRequest;
}

export function useUpdateUser(): UseMutationResult<User, UsersApiError, UpdateUserVariables> {
  const queryClient = useQueryClient();
  return useMutation<User, UsersApiError, UpdateUserVariables>({
    mutationFn: ({ id, request }) => updateUser(id, request),
    onSuccess: (user) => {
      queryClient.setQueryData(userDetailQueryKey(user.id), user);
      void queryClient.invalidateQueries({ queryKey: userDetailQueryKey(user.id) });
      void invalidateUsersLists(queryClient);
    },
  });
}

export function useDeleteUser(): UseMutationResult<undefined, UsersApiError, string> {
  const queryClient = useQueryClient();
  return useMutation<undefined, UsersApiError, string>({
    mutationFn: (id: string): Promise<undefined> =>
      deleteUser(id).then(() => undefined),
    onSuccess: (_data, id) => {
      queryClient.removeQueries({ queryKey: userDetailQueryKey(id) });
      void invalidateUsersLists(queryClient);
    },
  });
}

export interface UploadAvatarVariables {
  id: string;
  file: File | null;
}

export function useUploadAvatar(): UseMutationResult<User, UsersApiError, UploadAvatarVariables> {
  const queryClient = useQueryClient();
  return useMutation<User, UsersApiError, UploadAvatarVariables>({
    mutationFn: ({ id, file }) => uploadUserAvatar(id, file),
    onSuccess: (user) => {
      queryClient.setQueryData(userDetailQueryKey(user.id), user);
      void queryClient.invalidateQueries({ queryKey: userDetailQueryKey(user.id) });
      void invalidateUsersLists(queryClient);
    },
  });
}
