import type { TFunction } from 'i18next';
import type { ExperimentsApiError } from '../types';

function backendDetail(error: ExperimentsApiError): string | null {
  const message = error.message.trim();
  if (message.length === 0) {
    return null;
  }
  return error.code !== null ? `${message} (${error.code})` : message;
}

export function resolveExperimentsErrorMessage(error: ExperimentsApiError, t: TFunction): string {
  if (error.kind === 'network') {
    return t('experiments.errors.networkError');
  }
  if (error.kind === 'unexpected') {
    return t('experiments.errors.unexpectedError');
  }

  if (error.status === 400) {
    return backendDetail(error) ?? t('experiments.errors.badRequest');
  }
  if (error.status === 401) {
    return backendDetail(error) ?? t('experiments.errors.unauthorized');
  }
  if (error.status === 403) {
    return backendDetail(error) ?? t('experiments.errors.forbidden');
  }
  if (error.status === 404) {
    return backendDetail(error) ?? t('experiments.errors.notFound');
  }
  if (error.status === 409) {
    return backendDetail(error) ?? t('experiments.errors.conflict');
  }
  if (error.status === 422) {
    return backendDetail(error) ?? t('experiments.errors.validationFailed');
  }
  if (error.status === 429) {
    return backendDetail(error) ?? t('experiments.errors.tooManyRequests');
  }
  if (error.status !== null && error.status >= 500) {
    if (error.message.trim().length === 0) {
      return t('experiments.errors.serverError');
    }
    return t('experiments.errors.serverErrorDetail', {
      status: error.status,
      code: error.code ?? t('experiments.errors.unknownCode'),
      message: error.message,
    });
  }
  return backendDetail(error) ?? t('experiments.errors.unexpectedError');
}
