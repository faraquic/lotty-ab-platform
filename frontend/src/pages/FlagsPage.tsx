import {
  Alert,
  Button,
  Container,
  Group,
  Pagination,
  Skeleton,
  Stack,
  Text,
  Title,
} from '@mantine/core';
import { modals } from '@mantine/modals';
import { notifications } from '@mantine/notifications';
import { IconAlertCircle, IconPlus, IconRefresh } from '@tabler/icons-react';
import { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { CreateFlagModal } from '@/features/flags/components/CreateFlagModal';
import { FlagDetailsModal } from '@/features/flags/components/FlagDetailsModal';
import { FlagsTable } from '@/features/flags/components/FlagsTable';
import {
  FLAGS_PAGE_SIZE,
  useCreateFlag,
  useDeleteFlag,
  useFlagsList,
  useUpdateFlag,
} from '@/features/flags/api/useFlags';
import { resolveFlagsErrorMessage } from '@/features/flags/lib/flagsErrorMessage';
import { useMe } from '@/features/users/api/useUsers';
import { lastPageIndex, offsetForPage, totalPages } from '@/shared/lib/pagination';
import type { CreateFlagRequest, Flag, UpdateFlagRequest } from '@/features/flags/types';

const LIMIT = FLAGS_PAGE_SIZE;

export function FlagsPage() {
  const { t } = useTranslation();
  const [page, setPage] = useState(0);
  const [createOpened, setCreateOpened] = useState(false);
  const [selectedFlag, setSelectedFlag] = useState<Flag | null>(null);

  const offset = offsetForPage(page, LIMIT);
  const list = useFlagsList({ limit: LIMIT, offset });
  const meQuery = useMe();

  const canWrite = meQuery.data?.role === 'admin';
  const meta = list.data?.meta;
  const flags = list.data?.data ?? [];
  const pages = totalPages(meta?.total ?? 0, LIMIT);

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

  const createMutation = useCreateFlag();
  const updateMutation = useUpdateFlag();
  const deleteMutation = useDeleteFlag();

  const handleCreate = (request: CreateFlagRequest): void => {
    createMutation.mutate(request, {
      onSuccess: () => {
        notifications.show({
          id: 'flags-created',
          title: t('flags.successTitle'),
          message: t('flags.flagCreated'),
          color: 'green',
        });
        setCreateOpened(false);
        const currentTotal = list.data?.meta.total ?? 0;
        setPage(lastPageIndex(currentTotal + 1, LIMIT));
      },
      onError: (error) => {
        notifications.show({
          id: 'flags-create-error',
          title: t('flags.errorTitle'),
          message: resolveFlagsErrorMessage(error, t),
          color: 'red',
        });
      },
    });
  };

  const handleSave = (id: string, request: UpdateFlagRequest): void => {
    updateMutation.mutate(
      { id, request },
      {
        onSuccess: (flag) => {
          notifications.show({
            id: 'flags-updated',
            title: t('flags.successTitle'),
            message: t('flags.flagUpdated'),
            color: 'green',
          });
          setSelectedFlag(flag);
        },
        onError: (error) => {
          notifications.show({
            id: 'flags-update-error',
            title: t('flags.errorTitle'),
            message: resolveFlagsErrorMessage(error, t),
            color: 'red',
          });
        },
      },
    );
  };

  const handleDelete = (flag: Flag): void => {
    modals.openConfirmModal({
      title: t('flags.deleteFlagTitle'),
      centered: true,
      children: (
        <Stack gap="xs">
          <Text size="sm">{t('flags.deleteConfirm', { name: flag.name, key: flag.key })}</Text>
          <Text size="sm" c="dimmed">
            {t('flags.deleteConfirmUndone')}
          </Text>
        </Stack>
      ),
      labels: { confirm: t('flags.delete'), cancel: t('flags.cancel') },
      confirmProps: { color: 'red' },
      onConfirm: () => {
        deleteMutation.mutate(flag.id, {
          onSuccess: () => {
            notifications.show({
              id: 'flags-deleted',
              title: t('flags.successTitle'),
              message: t('flags.flagDeleted'),
              color: 'green',
            });
            if (selectedFlag?.id === flag.id) {
              setSelectedFlag(null);
            }
            const count = list.data?.meta.count ?? 0;
            const total = list.data?.meta.total ?? 0;
            if (count <= 1 && total > 1 && page > 0) {
              setPage(page - 1);
            }
          },
          onError: (error) => {
            notifications.show({
              id: 'flags-delete-error',
              title: t('flags.errorTitle'),
              message: resolveFlagsErrorMessage(error, t),
              color: 'red',
            });
          },
        });
      },
    });
  };

  const isInitialLoading = list.isPending;

  return (
    <Container size="lg" py="md">
      <Stack gap="md">
        <Group justify="space-between" align="center">
          <Title order={2} size="1.25rem" fw={600}>
            {t('flags.title')}
          </Title>
          {canWrite ? (
            <Button
              size="sm"
              leftSection={<IconPlus size={16} />}
              onClick={() => {
                setCreateOpened(true);
              }}
              disabled={list.isError}
            >
              {t('flags.createFlag')}
            </Button>
          ) : (
            <Text size="sm" c="dimmed">
              {t('flags.readOnlyHint')}
            </Text>
          )}
        </Group>

        {isInitialLoading ? (
          <Stack gap="xs" aria-label={t('flags.title')}>
            <Skeleton height={38} radius="sm" />
            <Skeleton height={52} radius="sm" />
            <Skeleton height={52} radius="sm" />
            <Skeleton height={52} radius="sm" />
          </Stack>
        ) : null}

        {list.isError ? (
          <Alert
            variant="light"
            color="red"
            title={t('flags.errorTitle')}
            icon={<IconAlertCircle size={16} />}
          >
            <Stack gap="sm" align="flex-start">
              <Text size="sm">{resolveFlagsErrorMessage(list.error, t)}</Text>
              <Button
                size="xs"
                variant="default"
                leftSection={<IconRefresh size={14} />}
                loading={list.isFetching}
                onClick={() => {
                  void list.refetch();
                }}
              >
                {t('flags.retry')}
              </Button>
            </Stack>
          </Alert>
        ) : null}

        {!isInitialLoading && !list.isError && meta !== undefined && meta.total === 0 ? (
          <Stack gap="sm" align="center" py="xl">
            <Text size="sm" fw={600}>
              {t('flags.noFlags')}
            </Text>
            <Text size="sm" c="dimmed">
              {t('flags.noFlagsHint')}
            </Text>
            {canWrite ? (
              <Button
                size="sm"
                leftSection={<IconPlus size={16} />}
                onClick={() => {
                  setCreateOpened(true);
                }}
              >
                {t('flags.createFlag')}
              </Button>
            ) : null}
          </Stack>
        ) : null}

        {!isInitialLoading && !list.isError && flags.length > 0 ? (
          <>
            <FlagsTable
              flags={flags}
              canWrite={canWrite}
              onView={setSelectedFlag}
              onDelete={handleDelete}
              isMutating={deleteMutation.isPending}
            />
            <Group justify="space-between" align="center">
              <Text size="xs" c="dimmed">
                {t('flags.paginationSummary', {
                  from: offset + 1,
                  to: offset + flags.length,
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
                  aria-label={t('flags.title')}
                />
              ) : null}
            </Group>
          </>
        ) : null}
      </Stack>

      <CreateFlagModal
        opened={createOpened}
        isPending={createMutation.isPending}
        onClose={() => {
          setCreateOpened(false);
        }}
        onSubmit={handleCreate}
      />
      <FlagDetailsModal
        flag={selectedFlag}
        canWrite={canWrite}
        opened={selectedFlag !== null}
        isSaving={updateMutation.isPending}
        onClose={() => {
          setSelectedFlag(null);
        }}
        onSave={handleSave}
      />
    </Container>
  );
}
