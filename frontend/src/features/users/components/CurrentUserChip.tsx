import { Avatar, Badge, Group, Skeleton, Stack, Text } from '@mantine/core';
import { useTranslation } from 'react-i18next';
import { useMe } from '../api/useUsers';
import { roleBadgeColor } from '../lib/roleBadge';

export function CurrentUserChip() {
  const { t } = useTranslation();
  const meQuery = useMe();

  if (meQuery.isPending) {
    return (
      <Group gap="xs" wrap="nowrap" aria-hidden="true">
        <Skeleton height={28} width={28} radius="sm" />
        <Skeleton height={12} width={72} radius="sm" />
      </Group>
    );
  }

  if (meQuery.data === undefined) {
    return null;
  }
  const me = meQuery.data;

  return (
    <Group gap="xs" wrap="nowrap" aria-label={t('users.currentUser', { name: me.full_name })}>
      <Avatar src={me.avatar_url} name={me.full_name} alt={me.full_name} size="sm" radius="sm" />
      <Stack gap={2} visibleFrom="sm">
        <Text size="xs" fw={600} lineClamp={1} maw={140}>
          {me.full_name}
        </Text>
        <Badge size="xs" variant="light" color={roleBadgeColor(me.role)}>
          {t(`users.roles.${me.role}`)}
        </Badge>
      </Stack>
    </Group>
  );
}
