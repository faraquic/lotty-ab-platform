import type { TFunction } from 'i18next';
import type { FlagsApiError } from '../types';

function backendDetail(error: FlagsApiError): string | null {
  const message = error.message.trim();
  if (message.length === 0) {
    return null;
  }
  return error.code !== null ? `${message} (${error.code})` : message;
}

export function resolveFlagsErrorMessage(error: FlagsApiError, t: TFunction): string {
  if (error.kind === 'network') {
    return t('flags.errors.networkError');
  }
  if (error.kind === 'unexpected') {
    return t('flags.errors.unexpectedError');
  }

  if (error.status === 400) {
    return backendDetail(error) ?? t('flags.errors.badRequest');
  }
  if (error.status === 401) {
    return backendDetail(error) ?? t('flags.errors.unauthorized');
  }
  if (error.status === 403) {
    return backendDetail(error) ?? t('flags.errors.forbidden');
  }
  if (error.status === 404) {
    return backendDetail(error) ?? t('flags.errors.notFound');
  }
  if (error.status === 409) {
    return backendDetail(error) ?? t('flags.errors.conflict');
  }
  if (error.status === 422) {
    return backendDetail(error) ?? t('flags.errors.validationFailed');
  }
  if (error.status === 429) {
    return backendDetail(error) ?? t('flags.errors.tooManyRequests');
  }
  if (error.status !== null && error.status >= 500) {
    if (error.message.trim().length === 0) {
      return t('flags.errors.serverError');
    }
    return t('flags.errors.serverErrorDetail', {
      status: error.status,
      code: error.code ?? t('flags.errors.unknownCode'),
      message: error.message,
    });
  }
  return backendDetail(error) ?? t('flags.errors.unexpectedError');
}
