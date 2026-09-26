import { Alert, Button, Card, Container, Group, Skeleton, Stack, Text, Title } from '@mantine/core';
import { notifications } from '@mantine/notifications';
import { IconAlertCircle, IconArrowLeft, IconRefresh } from '@tabler/icons-react';
import { useTranslation } from 'react-i18next';
import { Link, useParams } from 'react-router';
import { ReviewDetailsView } from '@/features/reviews/components/ReviewDetailsView';
import type { ActFormValues } from '@/features/reviews/components/ReviewDetailsView';
import {
  useActOnReview,
  useAddReviewComment,
  useDeleteReviewComment,
  useResolveReviewComment,
  useReview,
} from '@/features/reviews/api/useReviews';
import { resolveReviewsErrorMessage } from '@/features/reviews/lib/reviewsErrorMessage';
import { useMe } from '@/features/users/api/useUsers';
import type { Review } from '@/features/reviews/types';

// Re-export for callers that previously imported the modal's form type.
export type { ActFormValues };

export function ReviewDetailsPage() {
  const { t } = useTranslation();
  const { id } = useParams();
  const reviewId = typeof id === 'string' ? id : null;

  const query = useReview(reviewId);
  const meQuery = useMe();
  const review = query.data ?? null;

  const actMutation = useActOnReview();
  const addCommentMutation = useAddReviewComment();
  const resolveMutation = useResolveReviewComment();
  const deleteCommentMutation = useDeleteReviewComment();

  const commentPending =
    addCommentMutation.isPending || resolveMutation.isPending || deleteCommentMutation.isPending;

  const notifyOk = (id: string, message: string): void => {
    notifications.show({ id, title: t('reviews.successTitle'), message, color: 'green' });
  };

  const notifyError = (id: string, error: unknown): void => {
    notifications.show({
      id,
      title: t('reviews.errorTitle'),
      message: resolveReviewsErrorMessage(
        error as Parameters<typeof resolveReviewsErrorMessage>[0],
        t,
      ),
      color: 'red',
    });
  };

  if (query.isPending) {
    return (
      <Container size="lg" py="md">
        <Stack gap="xs" aria-label={t('reviews.reviewDetails')}>
          <Skeleton height={28} radius="sm" />
          <Skeleton height={160} radius="sm" />
          <Skeleton height={120} radius="sm" />
        </Stack>
      </Container>
    );
  }

  if (query.isError) {
    return (
      <Container size="lg" py="md">
        <Alert
          variant="light"
          color="red"
          title={t('reviews.errorTitle')}
          icon={<IconAlertCircle size={16} />}
        >
          <Stack gap="sm" align="flex-start">
            <Text size="sm">{resolveReviewsErrorMessage(query.error, t)}</Text>
            <Button
              size="xs"
              variant="default"
              leftSection={<IconRefresh size={14} />}
              loading={query.isFetching}
              onClick={() => {
                void query.refetch();
              }}
            >
              {t('reviews.retry')}
            </Button>
          </Stack>
        </Alert>
      </Container>
    );
  }

  if (review === null) {
    return null;
  }

  const handleAct = (target: Review, values: ActFormValues, version: number): void => {
    if (values.decision === '') {
      return;
    }
    const comment = values.comment.trim();
    actMutation.mutate(
      {
        id: target.id,
        request: {
          decision: values.decision,
          version,
          ...(comment.length > 0 ? { comment } : {}),
        },
      },
      {
        onSuccess: () => {
          notifyOk('reviews-act', t('reviews.decisionRecorded'));
        },
        onError: (error) => {
          notifyError('reviews-act-error', error);
        },
      },
    );
  };

  const handleAddComment = (target: Review, body: string, parentId?: string): void => {
    addCommentMutation.mutate(
      { id: target.id, request: { body, ...(parentId ? { parent_id: parentId } : {}) } },
      {
        onSuccess: () => {
          notifyOk('reviews-comment', t('reviews.commentAdded'));
        },
        onError: (error) => {
          notifyError('reviews-comment-error', error);
        },
      },
    );
  };

  const handleResolve = (target: Review, commentId: string, resolved: boolean): void => {
    resolveMutation.mutate(
      { reviewId: target.id, commentId, request: { resolved } },
      {
        onSuccess: () => {
          notifyOk(
            'reviews-resolve',
            t(resolved ? 'reviews.threadResolved' : 'reviews.threadReopened'),
          );
        },
        onError: (error) => {
          notifyError('reviews-resolve-error', error);
        },
      },
    );
  };

  const handleDeleteComment = (target: Review, commentId: string): void => {
    deleteCommentMutation.mutate(
      { reviewId: target.id, commentId },
      {
        onSuccess: () => {
          notifyOk('reviews-comment-deleted', t('reviews.commentDeleted'));
        },
        onError: (error) => {
          notifyError('reviews-comment-delete-error', error);
        },
      },
    );
  };

  return (
    <Container size="md" py="md">
      <Stack gap="md">
        <Group justify="space-between" align="center">
          <Group gap="sm">
            <Button
              variant="subtle"
              size="sm"
              leftSection={<IconArrowLeft size={16} />}
              component={Link}
              to="/reviews"
            >
              {t('reviews.backToQueue')}
            </Button>
            <Title order={2} size="1.25rem" fw={600}>
              {t('reviews.reviewDetails')}
            </Title>
          </Group>
          <Button
            variant="default"
            size="xs"
            leftSection={<IconRefresh size={14} />}
            loading={query.isFetching}
            onClick={() => {
              void query.refetch();
            }}
          >
            {t('reviews.refresh')}
          </Button>
        </Group>

        <Card withBorder padding="md" radius="sm">
          <ReviewDetailsView
            review={review}
            currentUserId={meQuery.data?.id ?? null}
            currentUserRole={meQuery.data?.role ?? null}
            isActPending={actMutation.isPending}
            isCommentPending={commentPending}
            onAct={handleAct}
            onAddComment={handleAddComment}
            onResolve={handleResolve}
            onDeleteComment={handleDeleteComment}
          />
        </Card>
      </Stack>
    </Container>
  );
}
