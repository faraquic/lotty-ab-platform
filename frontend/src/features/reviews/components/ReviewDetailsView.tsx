import { ActionIcon, Badge, Button, Group, Select, Stack, Text, Textarea, Tooltip } from '@mantine/core';
import { useForm } from '@mantine/form';
import { modals } from '@mantine/modals';
import { IconCheck, IconTrash } from '@tabler/icons-react';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useExperiment } from '@/features/experiments/api/useExperiments';
import {
  approvalDecisionBadgeColor,
  canManageComment,
  formatReviewDateTime,
} from '../lib/threads';
import { APPROVAL_DECISIONS } from '../types';
import type { ApprovalDecision, Review, ReviewComment } from '../types';

export interface ActFormValues {
  decision: ApprovalDecision | '';
  comment: string;
}

interface CommentNodeProps {
  reviewId: string;
  comment: ReviewComment;
  depth: number;
  currentUserId: string | null;
  currentUserRole: string | null;
  isPending: boolean;
  onReply: (parentId: string, body: string) => void;
  onResolve: (commentId: string, resolved: boolean) => void;
  onDelete: (commentId: string) => void;
}

function CommentNode({
  reviewId,
  comment,
  depth,
  currentUserId,
  currentUserRole,
  isPending,
  onReply,
  onResolve,
  onDelete,
}: CommentNodeProps) {
  const { t, i18n } = useTranslation();
  const [replying, setReplying] = useState(false);
  const [replyBody, setReplyBody] = useState('');

  const manageable = canManageComment(comment.author_id, currentUserId, currentUserRole);
  const resolvable = manageable || currentUserRole === 'approver' || currentUserRole === 'admin';

  const submitReply = (): void => {
    if (replyBody.trim().length === 0 || isPending) {
      return;
    }
    onReply(comment.id, replyBody.trim());
    setReplyBody('');
    setReplying(false);
  };

  return (
    <Stack
      gap="xs"
      pl={depth > 0 ? 'md' : 0}
      style={depth > 0 ? { borderLeft: '2px solid var(--mantine-color-default-border)' } : undefined}
      aria-label={t('reviews.comment')}
    >
      <Group gap="xs" justify="space-between" wrap="nowrap">
        <Text size="xs" c="dimmed">
          {formatReviewDateTime(comment.created_at, i18n.language)}
        </Text>
        <Group gap={4}>
          {comment.resolved ? (
            <Badge size="xs" color="green" variant="light">
              {t('reviews.resolved')}
            </Badge>
          ) : null}
          {resolvable ? (
            <Tooltip label={t(comment.resolved ? 'reviews.reopen' : 'reviews.resolve')}>
              <ActionIcon
                size="sm"
                variant="subtle"
                color="green"
                disabled={isPending}
                onClick={() => {
                  onResolve(comment.id, !comment.resolved);
                }}
                aria-label={t(comment.resolved ? 'reviews.reopen' : 'reviews.resolve')}
              >
                <IconCheck size={14} />
              </ActionIcon>
            </Tooltip>
          ) : null}
          {manageable ? (
            <Tooltip label={t('reviews.deleteComment')}>
              <ActionIcon
                size="sm"
                variant="subtle"
                color="red"
                disabled={isPending}
                onClick={() => {
                  onDelete(comment.id);
                }}
                aria-label={t('reviews.deleteComment')}
              >
                <IconTrash size={14} />
              </ActionIcon>
            </Tooltip>
          ) : null}
        </Group>
      </Group>
      <Text size="sm">{comment.body}</Text>
      {depth === 0 && !replying ? (
        <Button
          size="xs"
          variant="subtle"
          disabled={isPending}
          onClick={() => {
            setReplying(true);
          }}
        >
          {t('reviews.reply')}
        </Button>
      ) : null}
      {replying ? (
        <Stack gap="xs">
          <Textarea
            placeholder={t('reviews.replyPlaceholder')}
            autosize
            minRows={2}
            disabled={isPending}
            value={replyBody}
            onChange={(event) => {
              setReplyBody(event.currentTarget.value);
            }}
            aria-label={t('reviews.replyPlaceholder')}
          />
          <Group gap="xs">
            <Button size="xs" disabled={isPending || replyBody.trim().length === 0} onClick={submitReply}>
              {t('reviews.sendReply')}
            </Button>
            <Button
              size="xs"
              variant="subtle"
              disabled={isPending}
              onClick={() => {
                setReplying(false);
                setReplyBody('');
              }}
            >
              {t('reviews.cancel')}
            </Button>
          </Group>
        </Stack>
      ) : null}
      {comment.replies.map((reply) => (
        <CommentNode
          key={reply.id}
          reviewId={reviewId}
          comment={reply}
          depth={depth + 1}
          currentUserId={currentUserId}
          currentUserRole={currentUserRole}
          isPending={isPending}
          onReply={onReply}
          onResolve={onResolve}
          onDelete={onDelete}
        />
      ))}
    </Stack>
  );
}

export interface ReviewDetailsViewProps {
  review: Review;
  currentUserId: string | null;
  currentUserRole: string | null;
  isActPending: boolean;
  isCommentPending: boolean;
  onAct: (review: Review, values: ActFormValues, version: number) => void;
  onAddComment: (review: Review, body: string, parentId?: string) => void;
  onResolve: (review: Review, commentId: string, resolved: boolean) => void;
  onDeleteComment: (review: Review, commentId: string) => void;
}

const MAX_COMMENT_LENGTH = 4096;

export function ReviewDetailsView({
  review,
  currentUserId,
  currentUserRole,
  isActPending,
  isCommentPending,
  onAct,
  onAddComment,
  onResolve,
  onDeleteComment,
}: ReviewDetailsViewProps) {
  const { t, i18n } = useTranslation();
  const [commentBody, setCommentBody] = useState('');
  const experimentQuery = useExperiment(review.experiment_id);

  const actForm = useForm<ActFormValues>({
    initialValues: { decision: '', comment: '' },
    validate: {
      decision: (value) => (value === '' ? i18n.t('reviews.decisionRequired') : null),
      comment: (value) =>
        value.length > MAX_COMMENT_LENGTH
          ? i18n.t('reviews.commentMaxLength', { count: MAX_COMMENT_LENGTH })
          : null,
    },
  });

  const confirmDelete = (target: Review, commentId: string): void => {
    modals.openConfirmModal({
      title: t('reviews.deleteCommentTitle'),
      centered: true,
      children: <Text size="sm">{t('reviews.deleteCommentConfirm')}</Text>,
      labels: { confirm: t('reviews.delete'), cancel: t('reviews.cancel') },
      confirmProps: { color: 'red' },
      onConfirm: () => {
        onDeleteComment(target, commentId);
      },
    });
  };

  return (
    <Stack gap="md">
      <Group gap="xs">
        <Text size="xs" c="dimmed">
          {t('reviews.fieldExperiment', { id: review.experiment_id })}
        </Text>
        <Text size="xs" c="dimmed">
          {t('reviews.fieldVersionNum', { num: review.version_num })}
        </Text>
      </Group>

      <Stack gap="xs">
        <Text size="sm" fw={600}>
          {t('reviews.approvalsTitle', { count: review.approvals.length })}
        </Text>
        {review.approvals.length === 0 ? (
          <Text size="sm" c="dimmed">
            {t('reviews.noApprovals')}
          </Text>
        ) : (
          review.approvals.map((approval) => (
            <Group key={approval.id} gap="xs" justify="space-between">
              <Group gap="xs">
                <Badge
                  size="sm"
                  color={approvalDecisionBadgeColor(approval.decision)}
                  variant="light"
                >
                  {t(`reviews.decisions.${approval.decision}`)}
                </Badge>
                {approval.comment ? (
                  <Text size="sm" c="dimmed">
                    {approval.comment}
                  </Text>
                ) : null}
              </Group>
              <Text size="xs" c="dimmed">
                {formatReviewDateTime(approval.created_at, i18n.language)}
              </Text>
            </Group>
          ))
        )}
      </Stack>

      {review.status === 'open' ? (
        <form
          onSubmit={actForm.onSubmit((values) => {
            const version = experimentQuery.data?.version;
            if (values.decision !== '' && version !== undefined && !isActPending) {
              onAct(review, values, version);
              actForm.reset();
            }
          })}
          noValidate
        >
          <Stack gap="sm">
            <Text size="sm" fw={600}>
              {t('reviews.actTitle')}
            </Text>
            <Select
              label={t('reviews.decision')}
              placeholder={t('reviews.decisionPlaceholder')}
              withAsterisk
              disabled={isActPending}
              data={APPROVAL_DECISIONS.map((decision) => ({
                value: decision,
                label: t(`reviews.decisions.${decision}`),
              }))}
              {...actForm.getInputProps('decision')}
              onChange={(next) => {
                actForm.setFieldValue('decision', (next ?? '') as ApprovalDecision | '');
              }}
            />
            <Textarea
              label={t('reviews.actComment')}
              placeholder={t('reviews.actCommentPlaceholder')}
              disabled={isActPending}
              autosize
              minRows={2}
              {...actForm.getInputProps('comment')}
            />
            {experimentQuery.isPending ? (
              <Text size="xs" c="dimmed">
                {t('reviews.loadingExperimentVersion')}
              </Text>
            ) : null}
            <Group justify="flex-end">
              <Button
                type="submit"
                size="xs"
                loading={isActPending}
                disabled={isActPending || !actForm.isValid() || experimentQuery.data === undefined}
              >
                {t('reviews.submitDecision')}
              </Button>
            </Group>
          </Stack>
        </form>
      ) : (
        <Text size="sm" c="dimmed">
          {t('reviews.reviewClosedHint')}
        </Text>
      )}

      <Stack gap="sm">
        <Text size="sm" fw={600}>
          {t('reviews.commentsTitle')}
        </Text>
        {review.comments.length === 0 ? (
          <Text size="sm" c="dimmed">
            {t('reviews.noComments')}
          </Text>
        ) : (
          review.comments.map((comment) => (
            <CommentNode
              key={comment.id}
              reviewId={review.id}
              comment={comment}
              depth={0}
              currentUserId={currentUserId}
              currentUserRole={currentUserRole}
              isPending={isCommentPending}
              onReply={(parentId, body) => {
                onAddComment(review, body, parentId);
              }}
              onResolve={(commentId, resolved) => {
                onResolve(review, commentId, resolved);
              }}
              onDelete={(commentId) => {
                confirmDelete(review, commentId);
              }}
            />
          ))
        )}
        <Textarea
          placeholder={t('reviews.commentPlaceholder')}
          autosize
          minRows={2}
          disabled={isCommentPending}
          value={commentBody}
          onChange={(event) => {
            setCommentBody(event.currentTarget.value);
          }}
          aria-label={t('reviews.commentPlaceholder')}
        />
        <Group justify="flex-end">
          <Button
            size="xs"
            loading={isCommentPending}
            disabled={
              isCommentPending ||
              commentBody.trim().length === 0 ||
              commentBody.length > MAX_COMMENT_LENGTH
            }
            onClick={() => {
              onAddComment(review, commentBody.trim());
              setCommentBody('');
            }}
          >
            {t('reviews.addComment')}
          </Button>
        </Group>
      </Stack>
    </Stack>
  );
}
