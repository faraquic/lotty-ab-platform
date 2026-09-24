import {
  Alert,
  Button,
  Card,
  Container,
  Group,
  Pagination,
  Select,
  Skeleton,
  Stack,
  Tabs,
  Text,
  Title,
} from '@mantine/core';
import { modals } from '@mantine/modals';
import { notifications } from '@mantine/notifications';
import { IconAlertCircle, IconPlus, IconRefresh } from '@tabler/icons-react';
import { useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useExperimentsList } from '@/features/experiments/api/useExperiments';
import { ExperimenterGroupForm } from '@/features/reviews/components/ExperimenterGroupForm';
import { GroupDetailsModal } from '@/features/reviews/components/GroupDetailsModal';
import { GroupFormModal } from '@/features/reviews/components/GroupFormModal';
import { GroupsTable } from '@/features/reviews/components/GroupsTable';
import { ReviewDetailsModal } from '@/features/reviews/components/ReviewDetailsModal';
import type { ActFormValues } from '@/features/reviews/components/ReviewDetailsModal';
import { ReviewsTable } from '@/features/reviews/components/ReviewsTable';
import {
  GROUPS_PAGE_SIZE,
  REVIEWS_PAGE_SIZE,
  useActOnReview,
  useAddGroupMember,
  useAddReviewComment,
  useApproverGroupsList,
  useCreateApproverGroup,
  useDeleteReviewComment,
  useRemoveGroupMember,
  useResolveReviewComment,
  useReviewsList,
  useSetExperimenterGroup,
  useUpdateApproverGroup,
} from '@/features/reviews/api/useReviews';
import { resolveReviewsErrorMessage } from '@/features/reviews/lib/reviewsErrorMessage';
import { useMe, useUsersList } from '@/features/users/api/useUsers';
import { lastPageIndex, offsetForPage, totalPages } from '@/shared/lib/pagination';
import type {
  ApproverGroup,
  CreateApproverGroupRequest,
  Review,
  ReviewStatus,
  UpdateApproverGroupRequest,
} from '@/features/reviews/types';
import { REVIEW_STATUSES } from '@/features/reviews/types';

const LIMIT = REVIEWS_PAGE_SIZE;
const GROUP_LIMIT = GROUPS_PAGE_SIZE;

export function ReviewsPage() {
  const { t } = useTranslation();
  const [tab, setTab] = useState('queue');
  const [page, setPage] = useState(0);
  const [status, setStatus] = useState<ReviewStatus | null>(null);
  const [selectedReview, setSelectedReview] = useState<Review | null>(null);
  const [groupPage, setGroupPage] = useState(0);
  const [groupFormOpened, setGroupFormOpened] = useState(false);
  const [editingGroup, setEditingGroup] = useState<ApproverGroup | null>(null);
  const [selectedGroup, setSelectedGroup] = useState<ApproverGroup | null>(null);

  const meQuery = useMe();
  const isAdmin = meQuery.data?.role === 'admin';
  const currentUserId = meQuery.data?.id ?? null;
  const currentUserRole = meQuery.data?.role ?? null;

  const offset = offsetForPage(page, LIMIT);
  const list = useReviewsList({ limit: LIMIT, offset, status });
  const meta = list.data?.meta;
  const reviews = list.data?.data ?? [];
  const pages = totalPages(meta?.total ?? 0, LIMIT);

  const experimentsQuery = useExperimentsList({ limit: 100, offset: 0, status: null });
  const experimentNames = useMemo(() => {
    const map = new Map<string, string>();
    for (const experiment of experimentsQuery.data?.data ?? []) {
      map.set(experiment.id, experiment.name);
    }
    return map;
  }, [experimentsQuery.data]);

  const groupOffset = offsetForPage(groupPage, GROUP_LIMIT);
  const groupsQuery = useApproverGroupsList({ limit: GROUP_LIMIT, offset: groupOffset });
  const groupMeta = groupsQuery.data?.meta;
  const groups = groupsQuery.data?.data ?? [];
  const groupPages = totalPages(groupMeta?.total ?? 0, GROUP_LIMIT);

  const usersQuery = useUsersList({ limit: 100, offset: 0 }, isAdmin);
  const userOptions = useMemo(
    () =>
      (usersQuery.data?.data ?? []).map((user) => ({
        value: user.id,
        label: `${user.full_name} — ${user.email}`,
      })),
    [usersQuery.data],
  );
  const experimenterOptions = useMemo(
    () =>
      (usersQuery.data?.data ?? [])
        .filter((user) => user.role === 'experimenter')
        .map((user) => ({ value: user.id, label: `${user.full_name} — ${user.email}` })),
    [usersQuery.data],
  );
  const groupOptions = useMemo(
    () => groups.map((group) => ({ value: group.id, label: group.name })),
    [groups],
  );

  useEffect(() => {
    if (meta !== undefined && meta.total > 0) {
      const last = lastPageIndex(meta.total, LIMIT);
      if (page > last) {
        setPage(last);
      }
    }
    if (meta !== undefined && meta.total === 0 && page !== 0) {
      setPage(0);
    }
  }, [meta, page]);

  const actMutation = useActOnReview();
  const addCommentMutation = useAddReviewComment();
  const resolveMutation = useResolveReviewComment();
  const deleteCommentMutation = useDeleteReviewComment();
  const createGroupMutation = useCreateApproverGroup();
  const updateGroupMutation = useUpdateApproverGroup();
  const addMemberMutation = useAddGroupMember();
  const removeMemberMutation = useRemoveGroupMember();
  const assignMutation = useSetExperimenterGroup();

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

  const handleAct = (review: Review, values: ActFormValues, version: number): void => {
    if (values.decision === '') {
      return;
    }
    const comment = values.comment.trim();
    actMutation.mutate(
      {
        id: review.id,
        request: {
          decision: values.decision,
          version,
          ...(comment.length > 0 ? { comment } : {}),
        },
      },
      {
        onSuccess: (next) => {
          notifyOk('reviews-act', t('reviews.decisionRecorded'));
          setSelectedReview(next);
        },
        onError: (error) => {
          notifyError('reviews-act-error', error);
        },
      },
    );
  };

  const handleAddComment = (review: Review, body: string, parentId?: string): void => {
    addCommentMutation.mutate(
      { id: review.id, request: { body, ...(parentId ? { parent_id: parentId } : {}) } },
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

  const handleResolve = (review: Review, commentId: string, resolved: boolean): void => {
    resolveMutation.mutate(
      { reviewId: review.id, commentId, request: { resolved } },
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

  const handleDeleteComment = (review: Review, commentId: string): void => {
    deleteCommentMutation.mutate(
      { reviewId: review.id, commentId },
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

  const handleCreateGroup = (request: CreateApproverGroupRequest): void => {
    createGroupMutation.mutate(request, {
      onSuccess: () => {
        notifyOk('reviews-group-created', t('reviews.groupCreated'));
        setGroupFormOpened(false);
      },
      onError: (error) => {
        notifyError('reviews-group-create-error', error);
      },
    });
  };

  const handleUpdateGroup = (id: string, request: UpdateApproverGroupRequest): void => {
    updateGroupMutation.mutate(
      { id, request },
      {
        onSuccess: (next) => {
          notifyOk('reviews-group-updated', t('reviews.groupUpdated'));
          setGroupFormOpened(false);
          setEditingGroup(null);
          if (selectedGroup?.id === next.id) {
            setSelectedGroup(next);
          }
        },
        onError: (error) => {
          notifyError('reviews-group-update-error', error);
        },
      },
    );
  };

  const handleAddMember = (group: ApproverGroup, userId: string): void => {
    addMemberMutation.mutate(
      { id: group.id, request: { user_id: userId } },
      {
        onSuccess: (next) => {
          notifyOk('reviews-member-added', t('reviews.memberAdded'));
          if (selectedGroup?.id === next.id) {
            setSelectedGroup(next);
          }
        },
        onError: (error) => {
          notifyError('reviews-member-add-error', error);
        },
      },
    );
  };

  const handleRemoveMember = (group: ApproverGroup, userId: string): void => {
    modals.openConfirmModal({
      title: t('reviews.removeMemberTitle'),
      centered: true,
      children: <Text size="sm">{t('reviews.removeMemberConfirm', { name: group.name })}</Text>,
      labels: { confirm: t('reviews.remove'), cancel: t('reviews.cancel') },
      confirmProps: { color: 'red' },
      onConfirm: () => {
        removeMemberMutation.mutate(
          { id: group.id, userId },
          {
            onSuccess: (next) => {
              notifyOk('reviews-member-removed', t('reviews.memberRemoved'));
              if (selectedGroup?.id === next.id) {
                setSelectedGroup(next);
              }
            },
            onError: (error) => {
              notifyError('reviews-member-remove-error', error);
            },
          },
        );
      },
    });
  };

  const handleAssign = (experimenterId: string, groupId: string | null): void => {
    assignMutation.mutate(
      { experimenterId, request: { group_id: groupId } },
      {
        onSuccess: () => {
          notifyOk('reviews-assign', t('reviews.assignmentSaved'));
        },
        onError: (error) => {
          notifyError('reviews-assign-error', error);
        },
      },
    );
  };

  const isInitialLoading = list.isPending;
  const groupPending =
    addMemberMutation.isPending || removeMemberMutation.isPending;

  return (
    <Container size="lg" py="md">
      <Stack gap="md">
        <Title order={2} size="1.25rem" fw={600}>
          {t('reviews.title')}
        </Title>

        <Tabs
          value={tab}
          onChange={(next) => {
            setTab(next ?? 'queue');
          }}
        >
          <Tabs.List>
            <Tabs.Tab value="queue">{t('reviews.queueTab')}</Tabs.Tab>
            <Tabs.Tab value="groups">{t('reviews.groupsTab')}</Tabs.Tab>
          </Tabs.List>

          <Tabs.Panel value="queue" pt="md">
            <Stack gap="md">
              <Select
                size="sm"
                placeholder={t('reviews.statusFilter')}
                clearable
                value={status}
                data={REVIEW_STATUSES.map((value) => ({
                  value,
                  label: t(`reviews.statuses.${value}`),
                }))}
                onChange={(next) => {
                  setStatus((next ?? '') === '' ? null : (next as ReviewStatus));
                  setPage(0);
                }}
                style={{ minWidth: 200, maxWidth: 280 }}
                aria-label={t('reviews.statusFilter')}
              />

              {isInitialLoading ? (
                <Stack gap="xs" aria-label={t('reviews.title')}>
                  <Skeleton height={38} radius="sm" />
                  <Skeleton height={52} radius="sm" />
                  <Skeleton height={52} radius="sm" />
                </Stack>
              ) : null}

              {list.isError ? (
                <Alert
                  variant="light"
                  color="red"
                  title={t('reviews.errorTitle')}
                  icon={<IconAlertCircle size={16} />}
                >
                  <Stack gap="sm" align="flex-start">
                    <Text size="sm">{resolveReviewsErrorMessage(list.error, t)}</Text>
                    <Button
                      size="xs"
                      variant="default"
                      leftSection={<IconRefresh size={14} />}
                      loading={list.isFetching}
                      onClick={() => {
                        void list.refetch();
                      }}
                    >
                      {t('reviews.retry')}
                    </Button>
                  </Stack>
                </Alert>
              ) : null}

              {!isInitialLoading && !list.isError && meta !== undefined && meta.total === 0 ? (
                <Stack gap="sm" align="center" py="xl">
                  <Text size="sm" fw={600}>
                    {t('reviews.noReviews')}
                  </Text>
                  <Text size="sm" c="dimmed">
                    {t('reviews.noReviewsHint')}
                  </Text>
                </Stack>
              ) : null}

              {!isInitialLoading && !list.isError && reviews.length > 0 ? (
                <>
                  <ReviewsTable
                    reviews={reviews}
                    experimentNames={experimentNames}
                    onView={setSelectedReview}
                  />
                  <Group justify="space-between" align="center">
                    <Text size="xs" c="dimmed">
                      {t('reviews.paginationSummary', {
                        from: offset + 1,
                        to: offset + reviews.length,
                        total: meta?.total ?? 0,
                      })}
                    </Text>
                    {pages > 1 ? (
                      <Pagination
                        size="sm"
                        value={page + 1}
                        total={pages}
                        onChange={(next) => {
                          setPage(next - 1);
                        }}
                        aria-label={t('reviews.title')}
                      />
                    ) : null}
                  </Group>
                </>
              ) : null}
            </Stack>
          </Tabs.Panel>

          <Tabs.Panel value="groups" pt="md">
            {!isAdmin ? (
              <Alert variant="light" color="yellow" title={t('reviews.errorTitle')}>
                <Text size="sm">{t('reviews.groupsAdminHint')}</Text>
              </Alert>
            ) : (
              <Stack gap="md">
                <Group justify="space-between" align="center">
                  <Text size="sm" fw={600}>
                    {t('reviews.groupsTitle')}
                  </Text>
                  <Button
                    size="sm"
                    leftSection={<IconPlus size={16} />}
                    onClick={() => {
                      setEditingGroup(null);
                      setGroupFormOpened(true);
                    }}
                    disabled={groupsQuery.isError}
                  >
                    {t('reviews.createGroup')}
                  </Button>
                </Group>

                {groupsQuery.isPending ? (
                  <Stack gap="xs">
                    <Skeleton height={38} radius="sm" />
                    <Skeleton height={52} radius="sm" />
                  </Stack>
                ) : null}

                {groupsQuery.isError ? (
                  <Alert
                    variant="light"
                    color="red"
                    title={t('reviews.errorTitle')}
                    icon={<IconAlertCircle size={16} />}
                  >
                    <Stack gap="sm" align="flex-start">
                      <Text size="sm">
                        {resolveReviewsErrorMessage(groupsQuery.error, t)}
                      </Text>
                      <Button
                        size="xs"
                        variant="default"
                        leftSection={<IconRefresh size={14} />}
                        loading={groupsQuery.isFetching}
                        onClick={() => {
                          void groupsQuery.refetch();
                        }}
                      >
                        {t('reviews.retry')}
                      </Button>
                    </Stack>
                  </Alert>
                ) : null}

                {!groupsQuery.isPending && !groupsQuery.isError && groups.length === 0 ? (
                  <Text size="sm" c="dimmed" ta="center" py="xl">
                    {t('reviews.noGroups')}
                  </Text>
                ) : null}

                {!groupsQuery.isPending && !groupsQuery.isError && groups.length > 0 ? (
                  <>
                    <GroupsTable
                      groups={groups}
                      onView={setSelectedGroup}
                      onRemoveMember={handleRemoveMember}
                      isMutating={groupPending}
                    />
                    <Group justify="space-between" align="center">
                      <Text size="xs" c="dimmed">
                        {t('reviews.paginationSummary', {
                          from: groupOffset + 1,
                          to: groupOffset + groups.length,
                          total: groupMeta?.total ?? 0,
                        })}
                      </Text>
                      {groupPages > 1 ? (
                        <Pagination
                          size="sm"
                          value={groupPage + 1}
                          total={groupPages}
                          onChange={(next) => {
                            setGroupPage(next - 1);
                          }}
                          aria-label={t('reviews.groupsTitle')}
                        />
                      ) : null}
                    </Group>
                  </>
                ) : null}

                <Card withBorder padding="md" radius="sm">
                  <ExperimenterGroupForm
                    experimenterOptions={experimenterOptions}
                    groupOptions={groupOptions}
                    selectorsLoading={usersQuery.isPending}
                    isPending={assignMutation.isPending}
                    onAssign={handleAssign}
                  />
                </Card>
              </Stack>
            )}
          </Tabs.Panel>
        </Tabs>
      </Stack>

      <ReviewDetailsModal
        review={selectedReview}
        currentUserId={currentUserId}
        currentUserRole={currentUserRole}
        isActPending={actMutation.isPending}
        isCommentPending={commentPending}
        onClose={() => {
          setSelectedReview(null);
        }}
        onAct={handleAct}
        onAddComment={handleAddComment}
        onResolve={handleResolve}
        onDeleteComment={handleDeleteComment}
      />
      <GroupFormModal
        group={editingGroup}
        opened={groupFormOpened}
        isPending={createGroupMutation.isPending || updateGroupMutation.isPending}
        onClose={() => {
          setGroupFormOpened(false);
          setEditingGroup(null);
        }}
        onCreate={handleCreateGroup}
        onUpdate={handleUpdateGroup}
      />
      <GroupDetailsModal
        group={selectedGroup}
        isPending={groupPending}
        userOptions={userOptions}
        usersLoading={usersQuery.isPending}
        onClose={() => {
          setSelectedGroup(null);
        }}
        onEdit={(group) => {
          setEditingGroup(group);
          setGroupFormOpened(true);
        }}
        onAddMember={handleAddMember}
        onRemoveMember={handleRemoveMember}
      />
    </Container>
  );
}
