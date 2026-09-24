import { ActionIcon, Badge, Group, Table, Text, Tooltip } from '@mantine/core';
import { IconEye, IconUserMinus } from '@tabler/icons-react';
import { useTranslation } from 'react-i18next';
import type { ApproverGroup } from '../types';

interface GroupsTableProps {
  groups: ApproverGroup[];
  onView: (group: ApproverGroup) => void;
  onRemoveMember: (group: ApproverGroup, userId: string) => void;
  isMutating: boolean;
}

export function GroupsTable({ groups, onView, onRemoveMember, isMutating }: GroupsTableProps) {
  const { t } = useTranslation();

  return (
    <Table striped highlightOnHover withTableBorder withColumnBorders={false}>
      <Table.Thead>
        <Table.Tr>
          <Table.Th>{t('reviews.groupName')}</Table.Th>
          <Table.Th>{t('reviews.groupStatus')}</Table.Th>
          <Table.Th>{t('reviews.minApprovals')}</Table.Th>
          <Table.Th>{t('reviews.members')}</Table.Th>
          <Table.Th>{t('reviews.actions')}</Table.Th>
        </Table.Tr>
      </Table.Thead>
      <Table.Tbody>
        {groups.map((group) => (
          <Table.Tr key={group.id}>
            <Table.Td>
              <Text size="sm" fw={500}>
                {group.name}
              </Text>
              {group.description ? (
                <Text size="xs" c="dimmed">
                  {group.description}
                </Text>
              ) : null}
            </Table.Td>
            <Table.Td>
              <Badge
                size="sm"
                color={group.status === 'active' ? 'green' : 'gray'}
                variant="light"
              >
                {t(`reviews.groupStatuses.${group.status}`)}
              </Badge>
            </Table.Td>
            <Table.Td>
              <Text size="sm">{group.min_approvals}</Text>
            </Table.Td>
            <Table.Td>
              <Group gap={4}>
                {group.members.length === 0 ? (
                  <Text size="sm" c="dimmed">
                    —
                  </Text>
                ) : (
                  group.members.map((member) => (
                    <Group key={member.id} gap={4}>
                      <Badge size="sm" variant="light">
                        {member.full_name}
                      </Badge>
                      <Tooltip label={t('reviews.removeMemberNamed', { name: member.full_name })}>
                        <ActionIcon
                          size="xs"
                          variant="subtle"
                          color="red"
                          disabled={isMutating}
                          onClick={() => {
                            onRemoveMember(group, member.id);
                          }}
                          aria-label={t('reviews.removeMemberNamed', { name: member.full_name })}
                        >
                          <IconUserMinus size={12} />
                        </ActionIcon>
                      </Tooltip>
                    </Group>
                  ))
                )}
              </Group>
            </Table.Td>
            <Table.Td>
              <Tooltip label={t('reviews.viewGroup')}>
                <ActionIcon
                  variant="subtle"
                  onClick={() => {
                    onView(group);
                  }}
                  aria-label={t('reviews.viewGroupNamed', { name: group.name })}
                >
                  <IconEye size={16} />
                </ActionIcon>
              </Tooltip>
            </Table.Td>
          </Table.Tr>
        ))}
      </Table.Tbody>
    </Table>
  );
}
