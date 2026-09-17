export const USER_ROLES = ['admin', 'experimenter', 'approver', 'viewer'] as const;

export type UserRole = (typeof USER_ROLES)[number];

export function isUserRole(value: unknown): value is UserRole {
  return (
    typeof value === 'string' && (USER_ROLES as readonly string[]).includes(value)
  );
}

export interface User {
  id: string;
  full_name: string;
  email: string;
  role: UserRole;
  avatar_url: string | null;
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

export interface UserListResponse {
  success: true;
  data: User[];
  meta: PaginationMeta;
}

export interface UserResponse {
  success: true;
  data: User;
}

export interface CreateUserRequest {
  full_name: string;
  email: string;
  password: string;
  role: UserRole;
}

export interface UpdateUserRequest {
  email?: string;
  role?: UserRole;
}

export type UsersFailureKind = 'http' | 'network' | 'unexpected';

export class UsersApiError extends Error {
  readonly kind: UsersFailureKind;
  readonly status: number | null;
  readonly code: string | null;

  constructor(
    kind: UsersFailureKind,
    message: string,
    status: number | null,
    code: string | null,
  ) {
    super(message);
    this.name = 'UsersApiError';
    this.kind = kind;
    this.status = status;
    this.code = code;
  }
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

export function parseUser(value: unknown): User | null {
  if (!isRecord(value)) {
    return null;
  }
  const { id, full_name, email, role, created_at, updated_at } = value;
  const avatarUrl = value['avatar_url'];
  if (
    typeof id !== 'string' ||
    id.length === 0 ||
    typeof full_name !== 'string' ||
    typeof email !== 'string' ||
    !isUserRole(role) ||
    typeof created_at !== 'string' ||
    typeof updated_at !== 'string'
  ) {
    return null;
  }
  if (avatarUrl !== null && avatarUrl !== undefined && typeof avatarUrl !== 'string') {
    return null;
  }
  return {
    id,
    full_name,
    email,
    role,
    avatar_url: typeof avatarUrl === 'string' ? avatarUrl : null,
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

export function parseUserResponse(payload: unknown): UserResponse | null {
  if (!isRecord(payload) || payload['success'] !== true) {
    return null;
  }
  const user = parseUser(payload['data']);
  if (user === null) {
    return null;
  }
  return { success: true, data: user };
}

export function parseUserListResponse(payload: unknown): UserListResponse | null {
  if (!isRecord(payload) || payload['success'] !== true) {
    return null;
  }
  const data = payload['data'];
  if (!Array.isArray(data)) {
    return null;
  }
  const users: User[] = [];
  for (const item of data) {
    const user = parseUser(item);
    if (user === null) {
      return null;
    }
    users.push(user);
  }
  const meta = parsePaginationMeta(payload['meta']);
  if (meta === null) {
    return null;
  }
  return { success: true, data: users, meta };
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
