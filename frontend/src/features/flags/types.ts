import { parseUser } from '@/features/users/types';
import type { User } from '@/features/users/types';

export const FLAG_TYPES = ['string', 'number', 'bool'] as const;

export type FlagType = (typeof FLAG_TYPES)[number];

export function isFlagType(value: unknown): value is FlagType {
  return typeof value === 'string' && (FLAG_TYPES as readonly string[]).includes(value);
}

export type FlagDefaultValue = string | number | boolean;

export function isFlagDefaultValue(
  flagType: FlagType,
  value: unknown,
): value is FlagDefaultValue {
  switch (flagType) {
    case 'string':
      return typeof value === 'string';
    case 'number':
      return typeof value === 'number' && Number.isFinite(value);
    case 'bool':
      return typeof value === 'boolean';
  }
}

export interface Flag {
  id: string;
  key: string;
  name: string;
  type: FlagType;
  default_value: FlagDefaultValue;
  description: string | null;
  created_by: User | null;
  updated_by: User | null;
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

export interface FlagListResponse {
  success: true;
  data: Flag[];
  meta: PaginationMeta;
}

export interface FlagResponse {
  success: true;
  data: Flag;
}

export interface CreateFlagRequest {
  key: string;
  name: string;
  type: FlagType;
  default_value: FlagDefaultValue;
  description?: string;
}

export interface UpdateFlagRequest {
  key?: string;
  name?: string;
  default_value?: FlagDefaultValue;
  description?: string;
}

export type FlagsFailureKind = 'http' | 'network' | 'unexpected';

export class FlagsApiError extends Error {
  readonly kind: FlagsFailureKind;
  readonly status: number | null;
  readonly code: string | null;

  constructor(
    kind: FlagsFailureKind,
    message: string,
    status: number | null,
    code: string | null,
  ) {
    super(message);
    this.name = 'FlagsApiError';
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

export function parseFlag(value: unknown): Flag | null {
  if (!isRecord(value)) {
    return null;
  }
  const { id, key, name, type, default_value, created_at, updated_at } = value;
  const description = value['description'];
  if (
    typeof id !== 'string' ||
    id.length === 0 ||
    typeof key !== 'string' ||
    typeof name !== 'string' ||
    !isFlagType(type) ||
    typeof created_at !== 'string' ||
    typeof updated_at !== 'string'
  ) {
    return null;
  }
  if (!isFlagDefaultValue(type, default_value)) {
    return null;
  }
  if (description !== null && description !== undefined && typeof description !== 'string') {
    return null;
  }
  const createdBy = parseNullableUser(value['created_by']);
  const updatedBy = parseNullableUser(value['updated_by']);
  if (createdBy === undefined || updatedBy === undefined) {
    return null;
  }
  return {
    id,
    key,
    name,
    type,
    default_value,
    description: typeof description === 'string' ? description : null,
    created_by: createdBy,
    updated_by: updatedBy,
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

export function parseFlagResponse(payload: unknown): FlagResponse | null {
  if (!isRecord(payload) || payload['success'] !== true) {
    return null;
  }
  const flag = parseFlag(payload['data']);
  if (flag === null) {
    return null;
  }
  return { success: true, data: flag };
}

export function parseFlagListResponse(payload: unknown): FlagListResponse | null {
  if (!isRecord(payload) || payload['success'] !== true) {
    return null;
  }
  const data = payload['data'];
  if (!Array.isArray(data)) {
    return null;
  }
  const flags: Flag[] = [];
  for (const item of data) {
    const flag = parseFlag(item);
    if (flag === null) {
      return null;
    }
    flags.push(flag);
  }
  const meta = parsePaginationMeta(payload['meta']);
  if (meta === null) {
    return null;
  }
  return { success: true, data: flags, meta };
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
