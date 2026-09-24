import type { TFunction } from 'i18next';
import type { ReviewsApiError } from '../types';

function backendDetail(error: ReviewsApiError): string | null {
  const message = error.message.trim();
  if (message.length === 0) {
    return null;
  }
  return error.code !== null ? `${message} (${error.code})` : message;
}

export function resolveReviewsErrorMessage(error: ReviewsApiError, t: TFunction): string {
  if (error.kind === 'network') {
    return t('reviews.errors.networkError');
  }
  if (error.kind === 'unexpected') {
    return t('reviews.errors.unexpectedError');
  }

  if (error.status === 400) {
    return backendDetail(error) ?? t('reviews.errors.badRequest');
  }
  if (error.status === 401) {
    return backendDetail(error) ?? t('reviews.errors.unauthorized');
  }
  if (error.status === 403) {
    return backendDetail(error) ?? t('reviews.errors.forbidden');
  }
  if (error.status === 404) {
    return backendDetail(error) ?? t('reviews.errors.notFound');
  }
  if (error.status === 409) {
    return backendDetail(error) ?? t('reviews.errors.conflict');
  }
  if (error.status === 422) {
    return backendDetail(error) ?? t('reviews.errors.validationFailed');
  }
  if (error.status === 429) {
    return backendDetail(error) ?? t('reviews.errors.tooManyRequests');
  }
  if (error.status !== null && error.status >= 500) {
    if (error.message.trim().length === 0) {
      return t('reviews.errors.serverError');
    }
    return t('reviews.errors.serverErrorDetail', {
      status: error.status,
      code: error.code ?? t('reviews.errors.unknownCode'),
      message: error.message,
    });
  }
  return backendDetail(error) ?? t('reviews.errors.unexpectedError');
}
