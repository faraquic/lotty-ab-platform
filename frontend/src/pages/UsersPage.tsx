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
import { CreateUserModal } from '@/features/users/components/CreateUserModal';
import { UserDetailsModal } from '@/features/users/components/UserDetailsModal';
import { UsersTable } from '@/features/users/components/UsersTable';
import {
  USERS_PAGE_SIZE,
  useCreateUser,
  useDeleteUser,
  useMe,
  useUpdateUser,
  useUploadAvatar,
  useUsersList,
} from '@/features/users/api/useUsers';
import { resolveUsersErrorMessage } from '@/features/users/lib/usersErrorMessage';
import { lastPageIndex, offsetForPage, totalPages } from '@/features/users/lib/rules';
import type { CreateUserRequest, UpdateUserRequest, User } from '@/features/users/types';

const LIMIT = USERS_PAGE_SIZE;

export function UsersPage() {
  const { t } = useTranslation();
  const [page, setPage] = useState(0);
  const [createOpened, setCreateOpened] = useState(false);
  const [selectedUser, setSelectedUser] = useState<User | null>(null);

  const offset = offsetForPage(page, LIMIT);
  const list = useUsersList({ limit: LIMIT, offset });
  const meQuery = useMe();

  const me = meQuery.data ?? null;
  const meta = list.data?.meta;
  const users = list.data?.data ?? [];
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

  const createMutation = useCreateUser();
  const updateMutation = useUpdateUser();
  const deleteMutation = useDeleteUser();
  const avatarMutation = useUploadAvatar();

  const handleCreate = (request: CreateUserRequest): void => {
    createMutation.mutate(request, {
      onSuccess: () => {
        notifications.show({
          id: 'users-created',
          title: t('users.successTitle'),
          message: t('users.userCreated'),
          color: 'green',
        });
        setCreateOpened(false);
        const currentTotal = list.data?.meta.total ?? 0;
        setPage(lastPageIndex(currentTotal + 1, LIMIT));
      },
      onError: (error) => {
        notifications.show({
          id: 'users-create-error',
          title: t('users.errorTitle'),
          message: resolveUsersErrorMessage(error, t),
          color: 'red',
        });
      },
    });
  };

  const handleSave = (id: string, request: UpdateUserRequest): void => {
    updateMutation.mutate(
      { id, request },
      {
        onSuccess: (user) => {
          notifications.show({
            id: 'users-updated',
            title: t('users.successTitle'),
            message: t('users.userUpdated'),
            color: 'green',
          });
          setSelectedUser(user);
        },
        onError: (error) => {
          notifications.show({
            id: 'users-update-error',
            title: t('users.errorTitle'),
            message: resolveUsersErrorMessage(error, t),
            color: 'red',
          });
        },
      },
    );
  };

  const handleDelete = (user: User): void => {
    modals.openConfirmModal({
      title: t('users.deleteUserTitle'),
      centered: true,
      children: (
        <Stack gap="xs">
          <Text size="sm">{t('users.deleteConfirm', { name: user.full_name, email: user.email })}</Text>
          <Text size="sm" c="dimmed">
            {t('users.deleteConfirmUndone')}
          </Text>
        </Stack>
      ),
      labels: { confirm: t('users.delete'), cancel: t('users.cancel') },
      confirmProps: { color: 'red' },
      onConfirm: () => {
        deleteMutation.mutate(user.id, {
          onSuccess: () => {
            notifications.show({
              id: 'users-deleted',
              title: t('users.successTitle'),
              message: t('users.userDeleted'),
              color: 'green',
            });
            if (selectedUser?.id === user.id) {
              setSelectedUser(null);
            }
            const count = list.data?.meta.count ?? 0;
            const total = list.data?.meta.total ?? 0;
            if (count <= 1 && total > 1 && page > 0) {
              setPage(page - 1);
            }
          },
          onError: (error) => {
            notifications.show({
              id: 'users-delete-error',
              title: t('users.errorTitle'),
              message: resolveUsersErrorMessage(error, t),
              color: 'red',
            });
          },
        });
      },
    });
  };

  const handleAvatar = (id: string, file: File | null): void => {
    avatarMutation.mutate(
      { id, file },
      {
        onSuccess: (user) => {
          notifications.show({
            id: 'users-avatar-updated',
            title: t('users.successTitle'),
            message: file === null ? t('users.avatarRemoved') : t('users.avatarUpdated'),
            color: 'green',
          });
          setSelectedUser(user);
        },
        onError: (error) => {
          notifications.show({
            id: 'users-avatar-error',
            title: t('users.errorTitle'),
            message: resolveUsersErrorMessage(error, t),
            color: 'red',
          });
        },
      },
    );
  };

  const isInitialLoading = list.isPending;

  return (
    <Container size="lg" py="md">
      <Stack gap="md">
        <Group justify="space-between" align="center">
          <Title order={2} size="1.25rem" fw={600}>
            {t('users.title')}
          </Title>
          <Button
            size="sm"
            leftSection={<IconPlus size={16} />}
            onClick={() => {
              setCreateOpened(true);
            }}
            disabled={list.isError}
          >
            {t('users.createUser')}
          </Button>
        </Group>

        {isInitialLoading ? (
          <Stack gap="xs" aria-label={t('users.title')}>
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
            title={t('users.errorTitle')}
            icon={<IconAlertCircle size={16} />}
          >
            <Stack gap="sm" align="flex-start">
              <Text size="sm">{resolveUsersErrorMessage(list.error, t)}</Text>
              {list.error.status === 403 ? (
                <Text size="sm" c="dimmed">
                  {t('users.listForbiddenHint')}
                </Text>
              ) : null}
              <Button
                size="xs"
                variant="default"
                leftSection={<IconRefresh size={14} />}
                loading={list.isFetching}
                onClick={() => {
                  void list.refetch();
                }}
              >
                {t('users.retry')}
              </Button>
            </Stack>
          </Alert>
        ) : null}

        {!isInitialLoading && !list.isError && meta !== undefined && meta.total === 0 ? (
          <Stack gap="sm" align="center" py="xl">
            <Text size="sm" fw={600}>
              {t('users.noUsers')}
            </Text>
            <Text size="sm" c="dimmed">
              {t('users.noUsersHint')}
            </Text>
            <Button
              size="sm"
              leftSection={<IconPlus size={16} />}
              onClick={() => {
                setCreateOpened(true);
              }}
            >
              {t('users.createUser')}
            </Button>
          </Stack>
        ) : null}

        {!isInitialLoading && !list.isError && users.length > 0 ? (
          <>
            <UsersTable
              users={users}
              me={me}
              onView={setSelectedUser}
              onDelete={handleDelete}
              isMutating={deleteMutation.isPending}
            />
            <Group justify="space-between" align="center">
              <Text size="xs" c="dimmed">
                {t('users.paginationSummary', {
                  from: offset + 1,
                  to: offset + users.length,
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
                  aria-label={t('users.title')}
                />
              ) : null}
            </Group>
          </>
        ) : null}
      </Stack>

      <CreateUserModal
        opened={createOpened}
        isPending={createMutation.isPending}
        onClose={() => {
          setCreateOpened(false);
        }}
        onSubmit={handleCreate}
      />
      <UserDetailsModal
        user={selectedUser}
        me={me}
        opened={selectedUser !== null}
        isSaving={updateMutation.isPending}
        isAvatarPending={avatarMutation.isPending}
        onClose={() => {
          setSelectedUser(null);
        }}
        onSave={handleSave}
        onUploadAvatar={handleAvatar}
      />
    </Container>
  );
}
