import type { TFunction } from 'i18next';
import type { LoginApiError } from '../types';

function backendDetail(error: LoginApiError): string | null {
  const message = error.message.trim();
  if (message.length === 0) {
    return null;
  }
  return error.code !== null ? `${message} (${error.code})` : message;
}

export function resolveLoginErrorMessage(error: LoginApiError, t: TFunction): string {
  switch (error.kind) {
    case 'network':
      return t('login.networkError');
    case 'unexpected':
      return t('login.unexpectedError');
    case 'http':
      break;
  }

  if (error.status === 401) {
    return t('login.invalidCredentials');
  }
  if (error.status === 400) {
    return backendDetail(error) ?? t('login.badRequest');
  }
  if (error.status === 403) {
    return backendDetail(error) ?? t('login.forbidden');
  }
  if (error.status === 404) {
    return backendDetail(error) ?? t('login.notFound');
  }
  if (error.status === 422) {
    return backendDetail(error) ?? t('login.validationFailed');
  }
  if (error.status === 429) {
    return backendDetail(error) ?? t('login.tooManyRequests');
  }
  if (error.status !== null && error.status >= 500) {
    if (error.message.trim().length === 0) {
      return t('login.serverError');
    }
    return t('login.serverErrorDetail', {
      status: error.status,
      code: error.code ?? t('login.unknownCode'),
      message: error.message,
    });
  }
  return backendDetail(error) ?? t('login.unexpectedError');
}
