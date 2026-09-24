import { describe, expect, it } from 'vitest';
import {
  approvalDecisionBadgeColor,
  canManageComment,
  countComments,
  countUnresolvedComments,
  reviewStatusBadgeColor,
} from './threads';
import type { ReviewComment } from '../types';

function comment(overrides: Partial<ReviewComment> = {}): ReviewComment {
  return {
    id: 'c-1',
    author_id: 'u-1',
    parent_id: null,
    body: 'hello',
    resolved: false,
    created_at: '2026-09-17T07:55:24.211Z',
    updated_at: '2026-09-17T07:55:24.211Z',
    replies: [],
    ...overrides,
  };
}

describe('review status and decision colors', () => {
  it('maps every review status to a badge color', () => {
    expect(reviewStatusBadgeColor('open')).toBe('yellow');
    expect(reviewStatusBadgeColor('approved')).toBe('green');
    expect(reviewStatusBadgeColor('changes_requested')).toBe('orange');
    expect(reviewStatusBadgeColor('rejected')).toBe('red');
  });

  it('maps every approval decision to a badge color', () => {
    expect(approvalDecisionBadgeColor('approve')).toBe('green');
    expect(approvalDecisionBadgeColor('request_changes')).toBe('orange');
    expect(approvalDecisionBadgeColor('reject')).toBe('red');
    expect(approvalDecisionBadgeColor('unknown')).toBe('gray');
  });
});

describe('comment thread counts', () => {
  const tree = [
    comment({
      id: 'root-1',
      resolved: false,
      replies: [
        comment({ id: 'reply-1', parent_id: 'root-1', resolved: true }),
        comment({ id: 'reply-2', parent_id: 'root-1', resolved: false }),
      ],
    }),
    comment({ id: 'root-2', resolved: true }),
  ];

  it('counts all comments including nested replies', () => {
    expect(countComments(tree)).toBe(4);
    expect(countComments([])).toBe(0);
  });

  it('counts only unresolved comments across levels', () => {
    expect(countUnresolvedComments(tree)).toBe(2);
    expect(countUnresolvedComments([])).toBe(0);
  });
});

describe('comment management permissions', () => {
  it('allows authors to manage their own comments', () => {
    expect(canManageComment('u-1', 'u-1', 'viewer')).toBe(true);
  });

  it('allows admins to manage foreign comments', () => {
    expect(canManageComment('u-1', 'u-9', 'admin')).toBe(true);
  });

  it('denies other roles on foreign comments without a session', () => {
    expect(canManageComment('u-1', 'u-9', 'approver')).toBe(false);
    expect(canManageComment('u-1', 'u-9', 'viewer')).toBe(false);
    expect(canManageComment('u-1', null, 'admin')).toBe(false);
  });
});
