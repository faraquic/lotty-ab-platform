import type { TFunction } from 'i18next';
import type { MetricsApiError } from '../types';

function backendDetail(error: MetricsApiError): string | null {
  const message = error.message.trim();
  if (message.length === 0) {
    return null;
  }
  return error.code !== null ? `${message} (${error.code})` : message;
}

export function resolveMetricsErrorMessage(error: MetricsApiError, t: TFunction): string {
  if (error.kind === 'network') {
    return t('metrics.errors.networkError');
  }
  if (error.kind === 'unexpected') {
    return t('metrics.errors.unexpectedError');
  }

  if (error.status === 400) {
    return backendDetail(error) ?? t('metrics.errors.badRequest');
  }
  if (error.status === 401) {
    return backendDetail(error) ?? t('metrics.errors.unauthorized');
  }
  if (error.status === 403) {
    return backendDetail(error) ?? t('metrics.errors.forbidden');
  }
  if (error.status === 404) {
    return backendDetail(error) ?? t('metrics.errors.notFound');
  }
  if (error.status === 409) {
    return backendDetail(error) ?? t('metrics.errors.conflict');
  }
  if (error.status === 422) {
    return backendDetail(error) ?? t('metrics.errors.validationFailed');
  }
  if (error.status === 429) {
    return backendDetail(error) ?? t('metrics.errors.tooManyRequests');
  }
  if (error.status !== null && error.status >= 500) {
    if (error.message.trim().length === 0) {
      return t('metrics.errors.serverError');
    }
    return t('metrics.errors.serverErrorDetail', {
      status: error.status,
      code: error.code ?? t('metrics.errors.unknownCode'),
      message: error.message,
    });
  }
  return backendDetail(error) ?? t('metrics.errors.unexpectedError');
}
