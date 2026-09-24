import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import type { UseMutationResult, UseQueryResult } from '@tanstack/react-query';
import {
  GROUPS_LIST_KEY_PREFIX,
  GROUPS_PAGE_SIZE,
  REVIEWS_LIST_KEY_PREFIX,
  REVIEWS_PAGE_SIZE,
  actOnReview,
  addGroupMember,
  addReviewComment,
  createApproverGroup,
  deleteReviewComment,
  getApproverGroup,
  getReview,
  groupDetailQueryKey,
  groupsListQueryKey,
  listApproverGroups,
  listReviews,
  removeGroupMember,
  resolveReviewComment,
  reviewDetailQueryKey,
  reviewsListQueryKey,
  setExperimenterGroup,
  updateApproverGroup,
} from './reviews';
import type { ReviewsApiError, ReviewStatus } from '../types';
import type {
  ActOnReviewRequest,
  AddGroupMemberRequest,
  AddReviewCommentRequest,
  ApproverGroup,
  ApproverGroupListResponse,
  CreateApproverGroupRequest,
  ResolveReviewCommentRequest,
  Review,
  ReviewComment,
  ReviewListResponse,
  SetExperimenterGroupRequest,
  UpdateApproverGroupRequest,
} from '../types';

export { REVIEWS_PAGE_SIZE, GROUPS_PAGE_SIZE };

export interface ReviewsListParams {
  limit: number;
  offset: number;
  status: ReviewStatus | null;
}

export function useReviewsList(
  params: ReviewsListParams,
): UseQueryResult<ReviewListResponse, ReviewsApiError> {
  const { limit, offset, status } = params;
  return useQuery<ReviewListResponse, ReviewsApiError>({
    queryKey: reviewsListQueryKey(limit, offset, status),
    queryFn: () => listReviews({ limit, offset, status }),
    placeholderData: (previous) => previous,
  });
}

export function useReview(id: string | null): UseQueryResult<Review, ReviewsApiError> {
  return useQuery<Review, ReviewsApiError>({
    queryKey: id === null ? ['reviews', 'detail', 'none'] : reviewDetailQueryKey(id),
    queryFn: () => getReview(id ?? ''),
    enabled: id !== null,
  });
}

export interface ActOnReviewVariables {
  id: string;
  request: ActOnReviewRequest;
}

export function useActOnReview(): UseMutationResult<Review, ReviewsApiError, ActOnReviewVariables> {
  const queryClient = useQueryClient();
  return useMutation<Review, ReviewsApiError, ActOnReviewVariables>({
    mutationFn: ({ id, request }) => actOnReview(id, request),
    onSuccess: (review) => {
      queryClient.setQueryData(reviewDetailQueryKey(review.id), review);
      void queryClient.invalidateQueries({ queryKey: reviewDetailQueryKey(review.id) });
      void queryClient.invalidateQueries({ queryKey: [REVIEWS_LIST_KEY_PREFIX] });
      void queryClient.invalidateQueries({ queryKey: ['experiments'] });
    },
  });
}

export interface AddCommentVariables {
  id: string;
  request: AddReviewCommentRequest;
}

export function useAddReviewComment(): UseMutationResult<
  ReviewComment,
  ReviewsApiError,
  AddCommentVariables
> {
  const queryClient = useQueryClient();
  return useMutation<ReviewComment, ReviewsApiError, AddCommentVariables>({
    mutationFn: ({ id, request }) => addReviewComment(id, request),
    onSuccess: (_comment, variables) => {
      void queryClient.invalidateQueries({ queryKey: reviewDetailQueryKey(variables.id) });
      void queryClient.invalidateQueries({ queryKey: [REVIEWS_LIST_KEY_PREFIX] });
    },
  });
}

export interface ResolveCommentVariables {
  reviewId: string;
  commentId: string;
  request: ResolveReviewCommentRequest;
}

export function useResolveReviewComment(): UseMutationResult<
  ReviewComment,
  ReviewsApiError,
  ResolveCommentVariables
> {
  const queryClient = useQueryClient();
  return useMutation<ReviewComment, ReviewsApiError, ResolveCommentVariables>({
    mutationFn: ({ commentId, request }) => resolveReviewComment(commentId, request),
    onSuccess: (_comment, variables) => {
      void queryClient.invalidateQueries({ queryKey: reviewDetailQueryKey(variables.reviewId) });
    },
  });
}

export interface DeleteCommentVariables {
  reviewId: string;
  commentId: string;
}

export function useDeleteReviewComment(): UseMutationResult<
  undefined,
  ReviewsApiError,
  DeleteCommentVariables
> {
  const queryClient = useQueryClient();
  return useMutation<undefined, ReviewsApiError, DeleteCommentVariables>({
    mutationFn: ({ commentId }) => deleteReviewComment(commentId),
    onSuccess: (_data, variables) => {
      void queryClient.invalidateQueries({ queryKey: reviewDetailQueryKey(variables.reviewId) });
    },
  });
}

export interface GroupsListParams {
  limit: number;
  offset: number;
}

export function useApproverGroupsList(
  params: GroupsListParams,
): UseQueryResult<ApproverGroupListResponse, ReviewsApiError> {
  const { limit, offset } = params;
  return useQuery<ApproverGroupListResponse, ReviewsApiError>({
    queryKey: groupsListQueryKey(limit, offset),
    queryFn: () => listApproverGroups(limit, offset),
    placeholderData: (previous) => previous,
  });
}

export function useApproverGroup(
  id: string | null,
): UseQueryResult<ApproverGroup, ReviewsApiError> {
  return useQuery<ApproverGroup, ReviewsApiError>({
    queryKey: id === null ? ['approver-groups', 'detail', 'none'] : groupDetailQueryKey(id),
    queryFn: () => getApproverGroup(id ?? ''),
    enabled: id !== null,
  });
}

function onGroupChanged(
  queryClient: ReturnType<typeof useQueryClient>,
  group: ApproverGroup,
): void {
  queryClient.setQueryData(groupDetailQueryKey(group.id), group);
  void queryClient.invalidateQueries({ queryKey: groupDetailQueryKey(group.id) });
  void queryClient.invalidateQueries({ queryKey: [GROUPS_LIST_KEY_PREFIX] });
}

export function useCreateApproverGroup(): UseMutationResult<
  ApproverGroup,
  ReviewsApiError,
  CreateApproverGroupRequest
> {
  const queryClient = useQueryClient();
  return useMutation<ApproverGroup, ReviewsApiError, CreateApproverGroupRequest>({
    mutationFn: createApproverGroup,
    onSuccess: (group) => {
      onGroupChanged(queryClient, group);
    },
  });
}

export interface UpdateGroupVariables {
  id: string;
  request: UpdateApproverGroupRequest;
}

export function useUpdateApproverGroup(): UseMutationResult<
  ApproverGroup,
  ReviewsApiError,
  UpdateGroupVariables
> {
  const queryClient = useQueryClient();
  return useMutation<ApproverGroup, ReviewsApiError, UpdateGroupVariables>({
    mutationFn: ({ id, request }) => updateApproverGroup(id, request),
    onSuccess: (group) => {
      onGroupChanged(queryClient, group);
    },
  });
}

export interface AddMemberVariables {
  id: string;
  request: AddGroupMemberRequest;
}

export function useAddGroupMember(): UseMutationResult<
  ApproverGroup,
  ReviewsApiError,
  AddMemberVariables
> {
  const queryClient = useQueryClient();
  return useMutation<ApproverGroup, ReviewsApiError, AddMemberVariables>({
    mutationFn: ({ id, request }) => addGroupMember(id, request),
    onSuccess: (group) => {
      onGroupChanged(queryClient, group);
    },
  });
}

export interface RemoveMemberVariables {
  id: string;
  userId: string;
}

export function useRemoveGroupMember(): UseMutationResult<
  ApproverGroup,
  ReviewsApiError,
  RemoveMemberVariables
> {
  const queryClient = useQueryClient();
  return useMutation<ApproverGroup, ReviewsApiError, RemoveMemberVariables>({
    mutationFn: ({ id, userId }) => removeGroupMember(id, userId),
    onSuccess: (group) => {
      onGroupChanged(queryClient, group);
    },
  });
}

export interface SetExperimenterGroupVariables {
  experimenterId: string;
  request: SetExperimenterGroupRequest;
}

export function useSetExperimenterGroup(): UseMutationResult<
  undefined,
  ReviewsApiError,
  SetExperimenterGroupVariables
> {
  return useMutation<undefined, ReviewsApiError, SetExperimenterGroupVariables>({
    mutationFn: ({ experimenterId, request }) => setExperimenterGroup(experimenterId, request),
  });
}
