export const REVIEW_STATUSES = ['open', 'approved', 'changes_requested', 'rejected'] as const;

export type ReviewStatus = (typeof REVIEW_STATUSES)[number];

export function isReviewStatus(value: unknown): value is ReviewStatus {
  return typeof value === 'string' && (REVIEW_STATUSES as readonly string[]).includes(value);
}

export const APPROVAL_DECISIONS = ['approve', 'request_changes', 'reject'] as const;

export type ApprovalDecision = (typeof APPROVAL_DECISIONS)[number];

export function isApprovalDecision(value: unknown): value is ApprovalDecision {
  return (
    typeof value === 'string' && (APPROVAL_DECISIONS as readonly string[]).includes(value)
  );
}

export const GROUP_STATUSES = ['active', 'archived'] as const;

export type ApproverGroupStatus = (typeof GROUP_STATUSES)[number];

export function isApproverGroupStatus(value: unknown): value is ApproverGroupStatus {
  return typeof value === 'string' && (GROUP_STATUSES as readonly string[]).includes(value);
}

export interface ApproverGroupMember {
  id: string;
  full_name: string;
  email: string;
  role: string;
  avatar_url: string | null;
}

export interface ApproverGroup {
  id: string;
  name: string;
  description: string | null;
  min_approvals: number;
  status: ApproverGroupStatus;
  members: ApproverGroupMember[];
  created_at: string;
  updated_at: string;
}

export interface ReviewApproval {
  id: string;
  reviewer_id: string;
  decision: ApprovalDecision;
  comment: string | null;
  created_at: string;
}

export interface ReviewComment {
  id: string;
  author_id: string;
  parent_id: string | null;
  body: string;
  resolved: boolean;
  created_at: string;
  updated_at: string;
  replies: ReviewComment[];
}

export interface Review {
  id: string;
  experiment_id: string;
  version_id: string;
  version_num: number;
  status: ReviewStatus;
  approvals: ReviewApproval[];
  comments: ReviewComment[];
  created_by: string;
  created_at: string;
  updated_at: string;
}

export interface PaginationMeta {
  limit: number;
  offset: number;
  count: number;
  total: number;
  has_next: boolean;
}

export interface ReviewListResponse {
  success: true;
  data: Review[];
  meta: PaginationMeta;
}

export interface ReviewResponse {
  success: true;
  data: Review;
}

export interface ApproverGroupListResponse {
  success: true;
  data: ApproverGroup[];
  meta: PaginationMeta;
}

export interface ApproverGroupResponse {
  success: true;
  data: ApproverGroup;
}

export interface ReviewCommentResponse {
  success: true;
  data: ReviewComment;
}

export interface CreateApproverGroupRequest {
  name: string;
  description?: string;
  min_approvals: number;
}

export interface UpdateApproverGroupRequest {
  name?: string;
  description?: string | null;
  min_approvals?: number;
  status?: ApproverGroupStatus;
}

export interface AddGroupMemberRequest {
  user_id: string;
}

export interface SetExperimenterGroupRequest {
  group_id: string | null;
}

export interface ActOnReviewRequest {
  decision: ApprovalDecision;
  version: number;
  comment?: string;
}

export interface AddReviewCommentRequest {
  body: string;
  parent_id?: string;
}

export interface ResolveReviewCommentRequest {
  resolved: boolean;
}

export type ReviewsFailureKind = 'http' | 'network' | 'unexpected';

export class ReviewsApiError extends Error {
  readonly kind: ReviewsFailureKind;
  readonly status: number | null;
  readonly code: string | null;

  constructor(
    kind: ReviewsFailureKind,
    message: string,
    status: number | null,
    code: string | null,
  ) {
    super(message);
    this.name = 'ReviewsApiError';
    this.kind = kind;
    this.status = status;
    this.code = code;
  }
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

function parseNullableString(value: unknown): string | null | undefined {
  if (value === null || value === undefined) {
    return null;
  }
  if (typeof value !== 'string') {
    return undefined;
  }
  return value;
}

export function parseGroupMember(value: unknown): ApproverGroupMember | null {
  if (!isRecord(value)) {
    return null;
  }
  const { id, full_name, email, role } = value;
  if (
    typeof id !== 'string' ||
    id.length === 0 ||
    typeof full_name !== 'string' ||
    typeof email !== 'string' ||
    typeof role !== 'string'
  ) {
    return null;
  }
  const avatarUrl = parseNullableString(value['avatar_url']);
  if (avatarUrl === undefined) {
    return null;
  }
  return { id, full_name, email, role, avatar_url: avatarUrl };
}

export function parseApproverGroup(value: unknown): ApproverGroup | null {
  if (!isRecord(value)) {
    return null;
  }
  const { id, name, min_approvals, status, created_at, updated_at } = value;
  if (
    typeof id !== 'string' ||
    id.length === 0 ||
    typeof name !== 'string' ||
    typeof min_approvals !== 'number' ||
    !isApproverGroupStatus(status) ||
    typeof created_at !== 'string' ||
    typeof updated_at !== 'string'
  ) {
    return null;
  }
  const description = parseNullableString(value['description']);
  if (description === undefined) {
    return null;
  }
  const rawMembers = value['members'];
  if (!Array.isArray(rawMembers)) {
    return null;
  }
  const members: ApproverGroupMember[] = [];
  for (const item of rawMembers) {
    const member = parseGroupMember(item);
    if (member === null) {
      return null;
    }
    members.push(member);
  }
  return { id, name, description, min_approvals, status, members, created_at, updated_at };
}

export function parseApproval(value: unknown): ReviewApproval | null {
  if (!isRecord(value)) {
    return null;
  }
  const { id, reviewer_id, decision, created_at } = value;
  if (
    typeof id !== 'string' ||
    id.length === 0 ||
    typeof reviewer_id !== 'string' ||
    !isApprovalDecision(decision) ||
    typeof created_at !== 'string'
  ) {
    return null;
  }
  const comment = parseNullableString(value['comment']);
  if (comment === undefined) {
    return null;
  }
  return { id, reviewer_id, decision, comment, created_at };
}

export function parseComment(value: unknown): ReviewComment | null {
  if (!isRecord(value)) {
    return null;
  }
  const { id, author_id, body, resolved, created_at, updated_at } = value;
  if (
    typeof id !== 'string' ||
    id.length === 0 ||
    typeof author_id !== 'string' ||
    typeof body !== 'string' ||
    typeof resolved !== 'boolean' ||
    typeof created_at !== 'string' ||
    typeof updated_at !== 'string'
  ) {
    return null;
  }
  const parentId = parseNullableString(value['parent_id']);
  if (parentId === undefined) {
    return null;
  }
  const rawReplies = value['replies'];
  const replies: ReviewComment[] = [];
  if (rawReplies !== undefined && rawReplies !== null) {
    if (!Array.isArray(rawReplies)) {
      return null;
    }
    for (const item of rawReplies) {
      const reply = parseComment(item);
      if (reply === null) {
        return null;
      }
      replies.push(reply);
    }
  }
  return { id, author_id, parent_id: parentId, body, resolved, created_at, updated_at, replies };
}

export function parseReview(value: unknown): Review | null {
  if (!isRecord(value)) {
    return null;
  }
  const {
    id,
    experiment_id,
    version_id,
    version_num,
    status,
    created_by,
    created_at,
    updated_at,
  } = value;
  if (
    typeof id !== 'string' ||
    id.length === 0 ||
    typeof experiment_id !== 'string' ||
    typeof version_id !== 'string' ||
    typeof version_num !== 'number' ||
    !isReviewStatus(status) ||
    typeof created_by !== 'string' ||
    typeof created_at !== 'string' ||
    typeof updated_at !== 'string'
  ) {
    return null;
  }
  const rawApprovals = value['approvals'];
  const rawComments = value['comments'];
  if (!Array.isArray(rawApprovals) || !Array.isArray(rawComments)) {
    return null;
  }
  const approvals: ReviewApproval[] = [];
  for (const item of rawApprovals) {
    const approval = parseApproval(item);
    if (approval === null) {
      return null;
    }
    approvals.push(approval);
  }
  const comments: ReviewComment[] = [];
  for (const item of rawComments) {
    const comment = parseComment(item);
    if (comment === null) {
      return null;
    }
    comments.push(comment);
  }
  return {
    id,
    experiment_id,
    version_id,
    version_num,
    status,
    approvals,
    comments,
    created_by,
    created_at,
    updated_at,
  };
}

function parsePaginationMeta(value: unknown): PaginationMeta | null {
  if (!isRecord(value)) {
    return null;
  }
  const { limit, offset, count, total, has_next } = value;
  if (
    typeof limit !== 'number' ||
    typeof offset !== 'number' ||
    typeof count !== 'number' ||
    typeof total !== 'number' ||
    typeof has_next !== 'boolean'
  ) {
    return null;
  }
  return { limit, offset, count, total, has_next };
}

export function parseReviewResponse(payload: unknown): ReviewResponse | null {
  if (!isRecord(payload) || payload['success'] !== true) {
    return null;
  }
  const review = parseReview(payload['data']);
  if (review === null) {
    return null;
  }
  return { success: true, data: review };
}

export function parseReviewListResponse(payload: unknown): ReviewListResponse | null {
  if (!isRecord(payload) || payload['success'] !== true) {
    return null;
  }
  const data = payload['data'];
  if (!Array.isArray(data)) {
    return null;
  }
  const reviews: Review[] = [];
  for (const item of data) {
    const review = parseReview(item);
    if (review === null) {
      return null;
    }
    reviews.push(review);
  }
  const meta = parsePaginationMeta(payload['meta']);
  if (meta === null) {
    return null;
  }
  return { success: true, data: reviews, meta };
}

export function parseApproverGroupResponse(payload: unknown): ApproverGroupResponse | null {
  if (!isRecord(payload) || payload['success'] !== true) {
    return null;
  }
  const group = parseApproverGroup(payload['data']);
  if (group === null) {
    return null;
  }
  return { success: true, data: group };
}

export function parseApproverGroupListResponse(
  payload: unknown,
): ApproverGroupListResponse | null {
  if (!isRecord(payload) || payload['success'] !== true) {
    return null;
  }
  const data = payload['data'];
  if (!Array.isArray(data)) {
    return null;
  }
  const groups: ApproverGroup[] = [];
  for (const item of data) {
    const group = parseApproverGroup(item);
    if (group === null) {
      return null;
    }
    groups.push(group);
  }
  const meta = parsePaginationMeta(payload['meta']);
  if (meta === null) {
    return null;
  }
  return { success: true, data: groups, meta };
}

export function parseReviewCommentResponse(payload: unknown): ReviewCommentResponse | null {
  if (!isRecord(payload) || payload['success'] !== true) {
    return null;
  }
  const comment = parseComment(payload['data']);
  if (comment === null) {
    return null;
  }
  return { success: true, data: comment };
}

export function parseSuccessPayload(payload: unknown): boolean {
  return isRecord(payload) && payload['success'] === true;
}

export function parseErrorPayload(payload: unknown): {
  code: string | null;
  message: string | null;
} {
  if (!isRecord(payload)) {
    return { code: null, message: null };
  }
  const error = payload['error'];
  if (!isRecord(error)) {
    return { code: null, message: null };
  }
  const code = error['code'];
  const message = error['message'];
  return {
    code: typeof code === 'string' ? code : null,
    message: typeof message === 'string' ? message : null,
  };
}
