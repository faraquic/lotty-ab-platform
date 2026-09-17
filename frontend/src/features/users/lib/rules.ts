import type { UpdateUserRequest, User } from '../types';

export { lastPageIndex, offsetForPage, totalPages } from '@/shared/lib/pagination';

export function isSelfUser(me: User | null | undefined, user: User): boolean {
  if (me === null || me === undefined) {
    return false;
  }
  return me.id === user.id;
}

export function canDeleteUser(me: User | null | undefined, user: User): boolean {
  return !isSelfUser(me, user);
}

export function canChangeUserRole(me: User | null | undefined, user: User): boolean {
  return !isSelfUser(me, user);
}

export interface EditableUserFields {
  email: string;
  role: User['role'];
}

export function buildUpdatePayload(
  original: User,
  values: EditableUserFields,
): UpdateUserRequest | null {
  const payload: UpdateUserRequest = {};
  const nextEmail = values.email.trim();
  if (nextEmail.length > 0 && nextEmail !== original.email) {
    payload.email = nextEmail;
  }
  if (values.role !== original.role) {
    payload.role = values.role;
  }
  if (payload.email === undefined && payload.role === undefined) {
    return null;
  }
  return payload;
}
