import { parseUser } from '@/features/users/types';
import type { User } from '@/features/users/types';

export const EXPERIMENT_STATUSES = [
  'draft',
  'review',
  'approved',
  'running',
  'paused',
  'completed',
  'archived',
  'rejected',
] as const;

export type ExperimentStatus = (typeof EXPERIMENT_STATUSES)[number];

export function isExperimentStatus(value: unknown): value is ExperimentStatus {
  return typeof value === 'string' && (EXPERIMENT_STATUSES as readonly string[]).includes(value);
}

export const COMPLETION_DECISIONS = ['rollout_winner', 'rollback', 'no_effect'] as const;

export type CompletionDecision = (typeof COMPLETION_DECISIONS)[number];

export function isCompletionDecision(value: unknown): value is CompletionDecision {
  return (
    typeof value === 'string' && (COMPLETION_DECISIONS as readonly string[]).includes(value)
  );
}

export type Targeting = string;

export function parseTargeting(value: unknown): Targeting | null {
  if (value === null || value === undefined) {
    return null;
  }
  if (typeof value === 'string') {
    return value.length > 0 ? value : null;
  }
  return null;
}

export interface ExperimentVariantInput {
  name: string;
  value: unknown;
  weight_bp: number;
  is_control: boolean;
}

export interface ExperimentVariant extends ExperimentVariantInput {
  id: string;
}

export interface ExperimentVersion {
  id: string;
  experiment_id: string;
  version_num: number;
  review_id: string | null;
  weights_total: number;
  targeting: Targeting | null;
  distribution_salt: string;
  created_by: string;
  created_at: string;
  updated_at: string;
}

export interface Experiment {
  id: string;
  flag_id: string;
  name: string;
  description: string | null;
  status: ExperimentStatus;
  current_version_id: string | null;
  owner_id: string;
  version: number;
  guardrail_paused: boolean;
  completion_decision: CompletionDecision | null;
  completion_reason: string | null;
  created_by: User | null;
  updated_by: User | null;
  current_version: ExperimentVersion | null;
  variants: ExperimentVariant[];
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

export interface ExperimentListResponse {
  success: true;
  data: Experiment[];
  meta: PaginationMeta;
}

export interface ExperimentResponse {
  success: true;
  data: Experiment;
}

export interface CreateExperimentRequest {
  flag_id: string;
  name: string;
  description?: string;
  weights_total?: number;
  targeting?: Targeting;
  variants?: ExperimentVariantInput[];
}

export interface UpdateExperimentRequest {
  version: number;
  name?: string;
  description?: string | null;
}

export interface CreateExperimentVersionRequest {
  version: number;
  weights_total?: number;
  targeting?: Targeting | null;
  variants?: ExperimentVariantInput[];
}

export interface SetExperimentVariantsRequest {
  version: number;
  variants: ExperimentVariantInput[];
}

export interface ExperimentTransitionRequest {
  version: number;
}

export interface CompleteExperimentRequest {
  version: number;
  decision: CompletionDecision;
  reason: string;
  winner_variant_id?: string;
}

export interface RolloutExperimentRequest {
  version: number;
  reason: string;
  winner_variant_id: string;
}

export type ExperimentsFailureKind = 'http' | 'network' | 'unexpected';

export class ExperimentsApiError extends Error {
  readonly kind: ExperimentsFailureKind;
  readonly status: number | null;
  readonly code: string | null;

  constructor(
    kind: ExperimentsFailureKind,
    message: string,
    status: number | null,
    code: string | null,
  ) {
    super(message);
    this.name = 'ExperimentsApiError';
    this.kind = kind;
    this.status = status;
    this.code = code;
  }
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

function parseNullableUser(value: unknown): User | null | undefined {
  if (value === null || value === undefined) {
    return null;
  }
  return parseUser(value);
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

function parseCompletionDecision(value: unknown): CompletionDecision | null | undefined {
  if (value === null || value === undefined) {
    return null;
  }
  if (!isCompletionDecision(value)) {
    return undefined;
  }
  return value;
}

export function parseVariantInput(value: unknown): ExperimentVariantInput | null {
  if (!isRecord(value)) {
    return null;
  }
  const { name, weight_bp, is_control } = value;
  if (typeof name !== 'string' || typeof weight_bp !== 'number' || typeof is_control !== 'boolean') {
    return null;
  }
  if (!('value' in value)) {
    return null;
  }
  return { name, value: value['value'], weight_bp, is_control };
}

export function parseVariant(value: unknown): ExperimentVariant | null {
  const input = parseVariantInput(value);
  if (input === null || !isRecord(value)) {
    return null;
  }
  const { id } = value;
  if (typeof id !== 'string' || id.length === 0) {
    return null;
  }
  return { ...input, id };
}

export function parseVersion(value: unknown): ExperimentVersion | null {
  if (!isRecord(value)) {
    return null;
  }
  const {
    id,
    experiment_id,
    version_num,
    weights_total,
    distribution_salt,
    created_by,
    created_at,
    updated_at,
  } = value;
  if (
    typeof id !== 'string' ||
    id.length === 0 ||
    typeof experiment_id !== 'string' ||
    typeof version_num !== 'number' ||
    typeof weights_total !== 'number' ||
    typeof distribution_salt !== 'string' ||
    typeof created_by !== 'string' ||
    typeof created_at !== 'string' ||
    typeof updated_at !== 'string'
  ) {
    return null;
  }
  const reviewId = parseNullableString(value['review_id']);
  if (reviewId === undefined) {
    return null;
  }
  if (!('targeting' in value)) {
    return null;
  }
  const targeting = parseTargeting(value['targeting']);
  if (value['targeting'] !== null && value['targeting'] !== undefined && targeting === null) {
    return null;
  }
  return {
    id,
    experiment_id,
    version_num,
    review_id: reviewId,
    weights_total,
    targeting,
    distribution_salt,
    created_by,
    created_at,
    updated_at,
  };
}

export function parseExperiment(value: unknown): Experiment | null {
  if (!isRecord(value)) {
    return null;
  }
  const {
    id,
    flag_id,
    name,
    status,
    owner_id,
    version,
    guardrail_paused,
    created_at,
    updated_at,
  } = value;
  if (
    typeof id !== 'string' ||
    id.length === 0 ||
    typeof flag_id !== 'string' ||
    typeof name !== 'string' ||
    !isExperimentStatus(status) ||
    typeof owner_id !== 'string' ||
    typeof version !== 'number' ||
    typeof guardrail_paused !== 'boolean' ||
    typeof created_at !== 'string' ||
    typeof updated_at !== 'string'
  ) {
    return null;
  }
  const description = parseNullableString(value['description']);
  const currentVersionId = parseNullableString(value['current_version_id']);
  const completionDecision = parseCompletionDecision(value['completion_decision']);
  const completionReason = parseNullableString(value['completion_reason']);
  if (
    description === undefined ||
    currentVersionId === undefined ||
    completionDecision === undefined ||
    completionReason === undefined
  ) {
    return null;
  }
  const createdBy = parseNullableUser(value['created_by']);
  const updatedBy = parseNullableUser(value['updated_by']);
  if (createdBy === undefined || updatedBy === undefined) {
    return null;
  }
  let currentVersion: ExperimentVersion | null = null;
  if (value['current_version'] !== null && value['current_version'] !== undefined) {
    currentVersion = parseVersion(value['current_version']);
    if (currentVersion === null) {
      return null;
    }
  }
  const rawVariants = value['variants'];
  const variants: ExperimentVariant[] = [];
  if (rawVariants !== undefined && rawVariants !== null) {
    if (!Array.isArray(rawVariants)) {
      return null;
    }
    for (const item of rawVariants) {
      const variant = parseVariant(item);
      if (variant === null) {
        return null;
      }
      variants.push(variant);
    }
  }
  return {
    id,
    flag_id,
    name,
    description,
    status,
    current_version_id: currentVersionId,
    owner_id,
    version,
    guardrail_paused,
    completion_decision: completionDecision,
    completion_reason: completionReason,
    created_by: createdBy,
    updated_by: updatedBy,
    current_version: currentVersion,
    variants,
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

export function parseExperimentResponse(payload: unknown): ExperimentResponse | null {
  if (!isRecord(payload) || payload['success'] !== true) {
    return null;
  }
  const experiment = parseExperiment(payload['data']);
  if (experiment === null) {
    return null;
  }
  return { success: true, data: experiment };
}

export function parseExperimentListResponse(payload: unknown): ExperimentListResponse | null {
  if (!isRecord(payload) || payload['success'] !== true) {
    return null;
  }
  const data = payload['data'];
  if (!Array.isArray(data)) {
    return null;
  }
  const experiments: Experiment[] = [];
  for (const item of data) {
    const experiment = parseExperiment(item);
    if (experiment === null) {
      return null;
    }
    experiments.push(experiment);
  }
  const meta = parsePaginationMeta(payload['meta']);
  if (meta === null) {
    return null;
  }
  return { success: true, data: experiments, meta };
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
