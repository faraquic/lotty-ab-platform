import type { ReviewComment, ReviewStatus } from '../types';

export function reviewStatusBadgeColor(status: ReviewStatus): string {
  switch (status) {
    case 'open':
      return 'yellow';
    case 'approved':
      return 'green';
    case 'changes_requested':
      return 'orange';
    case 'rejected':
      return 'red';
  }
}

export function approvalDecisionBadgeColor(decision: string): string {
  switch (decision) {
    case 'approve':
      return 'green';
    case 'request_changes':
      return 'orange';
    case 'reject':
      return 'red';
    default:
      return 'gray';
  }
}

export function countUnresolvedComments(comments: ReviewComment[]): number {
  let count = 0;
  for (const comment of comments) {
    if (!comment.resolved) {
      count += 1;
    }
    count += countUnresolvedComments(comment.replies);
  }
  return count;
}

export function countComments(comments: ReviewComment[]): number {
  let count = 0;
  for (const comment of comments) {
    count += 1;
    count += countComments(comment.replies);
  }
  return count;
}

export function canManageComment(
  commentAuthorId: string,
  currentUserId: string | null,
  currentUserRole: string | null,
): boolean {
  if (currentUserId === null) {
    return false;
  }
  if (commentAuthorId === currentUserId) {
    return true;
  }
  return currentUserRole === 'admin';
}

export function formatReviewDateTime(iso: string, language: string): string {
  const time = Date.parse(iso);
  if (Number.isNaN(time)) {
    return iso;
  }
  return new Intl.DateTimeFormat(language, {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  }).format(new Date(time));
}
