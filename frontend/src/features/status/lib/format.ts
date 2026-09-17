import type { TFunction } from 'i18next';

export function formatTime(checkedAt: number, language: string): string {
  return new Intl.DateTimeFormat(language, {
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
  }).format(new Date(checkedAt));
}

export function formatLatency(latencyMs: number | null, t: TFunction): string {
  if (latencyMs === null) {
    return '—';
  }
  return t('status.msValue', { ms: latencyMs });
}
