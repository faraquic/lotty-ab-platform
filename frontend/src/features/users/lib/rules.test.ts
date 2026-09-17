import { describe, expect, it } from 'vitest';
import {
  buildUpdatePayload,
  canChangeUserRole,
  canDeleteUser,
  isSelfUser,
  lastPageIndex,
  offsetForPage,
  totalPages,
} from './rules';
import type { User } from '../types';

function makeUser(overrides: Partial<User> = {}): User {
  return {
    id: 'uid-1',
    full_name: 'John Doe',
    email: 'john@example.com',
    role: 'experimenter',
    avatar_url: null,
    created_at: '2026-09-17T07:55:24.211Z',
    updated_at: '2026-09-17T07:55:24.211Z',
    ...overrides,
  };
}

describe('users rules', () => {
  it('detects the current user for destructive-action gating', () => {
    const me = makeUser({ id: 'me' });
    const other = makeUser({ id: 'other' });

    expect(isSelfUser(me, me)).toBe(true);
    expect(isSelfUser(me, other)).toBe(false);
    expect(isSelfUser(null, other)).toBe(false);
    expect(canDeleteUser(me, me)).toBe(false);
    expect(canDeleteUser(me, other)).toBe(true);
    expect(canChangeUserRole(me, me)).toBe(false);
    expect(canChangeUserRole(me, other)).toBe(true);
  });

  it('builds an update payload with only changed fields', () => {
    const original = makeUser();

    expect(buildUpdatePayload(original, { email: original.email, role: original.role })).toBeNull();
    expect(buildUpdatePayload(original, { email: 'new@example.com', role: original.role })).toEqual({
      email: 'new@example.com',
    });
    expect(buildUpdatePayload(original, { email: original.email, role: 'approver' })).toEqual({
      role: 'approver',
    });
  });

  it('computes pagination offsets and page counts', () => {
    expect(offsetForPage(0, 20)).toBe(0);
    expect(offsetForPage(2, 20)).toBe(40);
    expect(totalPages(0, 20)).toBe(1);
    expect(totalPages(137, 20)).toBe(7);
    expect(totalPages(40, 20)).toBe(2);
    expect(lastPageIndex(137, 20)).toBe(6);
    expect(lastPageIndex(0, 20)).toBe(0);
  });
});
