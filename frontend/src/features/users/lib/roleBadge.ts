import type { UserRole } from '../types';

export function roleBadgeColor(role: UserRole): string {
  switch (role) {
    case 'admin':
      return 'red';
    case 'experimenter':
      return 'green';
    case 'approver':
      return 'violet';
    case 'viewer':
      return 'gray';
  }
}
