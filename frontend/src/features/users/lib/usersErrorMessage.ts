import type { TFunction } from 'i18next';
import type { UsersApiError } from '../types';

function backendDetail(error: UsersApiError): string | null {
  const message = error.message.trim();
  if (message.length === 0) {
    return null;
  }
  return error.code !== null ? `${message} (${error.code})` : message;
}

export function resolveUsersErrorMessage(error: UsersApiError, t: TFunction): string {
  if (error.kind === 'network') {
    return t('users.errors.networkError');
  }
  if (error.kind === 'unexpected') {
    return t('users.errors.unexpectedError');
  }

  if (error.status === 400) {
    return backendDetail(error) ?? t('users.errors.badRequest');
  }
  if (error.status === 401) {
    return backendDetail(error) ?? t('users.errors.unauthorized');
  }
  if (error.status === 403) {
    return backendDetail(error) ?? t('users.errors.forbidden');
  }
  if (error.status === 404) {
    return backendDetail(error) ?? t('users.errors.notFound');
  }
  if (error.status === 409) {
    return backendDetail(error) ?? t('users.errors.conflict');
  }
  if (error.status === 422) {
    return backendDetail(error) ?? t('users.errors.validationFailed');
  }
  if (error.status === 429) {
    return backendDetail(error) ?? t('users.errors.tooManyRequests');
  }
  if (error.status !== null && error.status >= 500) {
    if (error.message.trim().length === 0) {
      return t('users.errors.serverError');
    }
    return t('users.errors.serverErrorDetail', {
      status: error.status,
      code: error.code ?? t('users.errors.unknownCode'),
      message: error.message,
    });
  }
  return backendDetail(error) ?? t('users.errors.unexpectedError');
}
