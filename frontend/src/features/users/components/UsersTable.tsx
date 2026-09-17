import { ActionIcon, Avatar, Badge, Group, Table, Text, Tooltip } from '@mantine/core';
import { IconPencil, IconTrash } from '@tabler/icons-react';
import { useTranslation } from 'react-i18next';
import { formatUserDateTime } from '../lib/format';
import { isSelfUser } from '../lib/rules';
import { roleBadgeColor } from '../lib/roleBadge';
import type { User } from '../types';

interface UsersTableProps {
  users: User[];
  me: User | null;
  onView: (user: User) => void;
  onDelete: (user: User) => void;
  isMutating: boolean;
}

export function UsersTable({ users, me, onView, onDelete, isMutating }: UsersTableProps) {
  const { t, i18n } = useTranslation();
  const language = i18n.resolvedLanguage ?? 'en';

  return (
    <Table striped highlightOnHover withTableBorder verticalSpacing="sm">
      <Table.Thead>
        <Table.Tr>
          <Table.Th>{t('users.avatar')}</Table.Th>
          <Table.Th>{t('users.name')}</Table.Th>
          <Table.Th>{t('users.email')}</Table.Th>
          <Table.Th>{t('users.role')}</Table.Th>
          <Table.Th>{t('users.created')}</Table.Th>
          <Table.Th>{t('users.actions')}</Table.Th>
        </Table.Tr>
      </Table.Thead>
      <Table.Tbody>
        {users.map((user) => {
          const isSelf = isSelfUser(me, user);
          return (
            <Table.Tr key={user.id}>
              <Table.Td>
                <Avatar src={user.avatar_url} name={user.full_name} alt={user.full_name} size="sm" radius="sm" />
              </Table.Td>
              <Table.Td>
                <Text size="sm" fw={500}>
                  {user.full_name}
                </Text>
              </Table.Td>
              <Table.Td>
                <Text size="sm">{user.email}</Text>
              </Table.Td>
              <Table.Td>
                <Badge size="sm" variant="light" color={roleBadgeColor(user.role)}>
                  {t(`users.roles.${user.role}`)}
                </Badge>
              </Table.Td>
              <Table.Td>
                <Text size="sm" c="dimmed">
                  {formatUserDateTime(user.created_at, language)}
                </Text>
              </Table.Td>
              <Table.Td>
                <Group gap="xs" wrap="nowrap">
                  <Tooltip label={t('users.viewEdit')}>
                    <ActionIcon
                      variant="subtle"
                      size="sm"
                      onClick={() => {
                        onView(user);
                      }}
                      aria-label={t('users.viewEditUser', { name: user.full_name })}
                    >
                      <IconPencil size={16} />
                    </ActionIcon>
                  </Tooltip>
                  <Tooltip label={isSelf ? t('users.selfDeleteHint') : t('users.deleteUser')}>
                    <ActionIcon
                      variant="subtle"
                      size="sm"
                      color="red"
                      disabled={isSelf || isMutating}
                      onClick={() => {
                        onDelete(user);
                      }}
                      aria-label={t('users.deleteUserNamed', { name: user.full_name })}
                    >
                      <IconTrash size={16} />
                    </ActionIcon>
                  </Tooltip>
                </Group>
              </Table.Td>
            </Table.Tr>
          );
        })}
      </Table.Tbody>
    </Table>
  );
}
