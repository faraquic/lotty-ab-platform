const DEFAULT_PANEL_API_BASE_URL = '/api/v1/panel';
const DEFAULT_RUNTIME_API_BASE_URL = '/api/v1/runtime';
const DEFAULT_ANALYTICS_API_BASE_URL = '/api/v1/analytics';

function resolveBaseUrl(envName: string, fallback: string): string {
  const fromEnv: unknown = import.meta.env[envName];
  if (typeof fromEnv === 'string' && fromEnv.trim().length > 0) {
    return fromEnv.replace(/\/+$/, '');
  }
  return fallback;
}

export const PANEL_API_BASE_URL = resolveBaseUrl(
  'VITE_PANEL_API_BASE_URL',
  DEFAULT_PANEL_API_BASE_URL,
);

export const RUNTIME_API_BASE_URL = resolveBaseUrl(
  'VITE_RUNTIME_API_BASE_URL',
  DEFAULT_RUNTIME_API_BASE_URL,
);

export const ANALYTICS_API_BASE_URL = resolveBaseUrl(
  'VITE_ANALYTICS_API_BASE_URL',
  DEFAULT_ANALYTICS_API_BASE_URL,
);
