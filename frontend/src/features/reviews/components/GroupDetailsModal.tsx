import { ActionIcon, Badge, Button, Group, Modal, Select, Stack, Text, Tooltip } from '@mantine/core';
import { IconUserMinus } from '@tabler/icons-react';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import type { ApproverGroup } from '../types';

interface GroupDetailsModalProps {
  group: ApproverGroup | null;
  isPending: boolean;
  userOptions: { value: string; label: string }[];
  usersLoading: boolean;
  onClose: () => void;
  onEdit: (group: ApproverGroup) => void;
  onAddMember: (group: ApproverGroup, userId: string) => void;
  onRemoveMember: (group: ApproverGroup, userId: string) => void;
}

export function GroupDetailsModal({
  group,
  isPending,
  userOptions,
  usersLoading,
  onClose,
  onEdit,
  onAddMember,
  onRemoveMember,
}: GroupDetailsModalProps) {
  const { t } = useTranslation();
  const [selectedUser, setSelectedUser] = useState('');

  return (
    <Modal
      opened={group !== null}
      onClose={onClose}
      title={group === null ? '' : t('reviews.groupDetailsNamed', { name: group.name })}
      centered
    >
      {group === null ? null : (
        <Stack gap="md">
          <Group justify="flex-end">
            <Button
              size="xs"
              variant="default"
              disabled={isPending}
              onClick={() => {
                onEdit(group);
              }}
            >
              {t('reviews.edit')}
            </Button>
          </Group>
          <Group gap="xl">
            <Text size="xs" c="dimmed">
              {t('reviews.minApprovalsValue', { count: group.min_approvals })}
            </Text>
            <Text size="xs" c="dimmed">
              {t('reviews.groupStatusValue', {
                status: t(`reviews.groupStatuses.${group.status}`),
              })}
            </Text>
          </Group>
          {group.description ? <Text size="sm">{group.description}</Text> : null}
          <Stack gap="xs">
            <Text size="sm" fw={600}>
              {t('reviews.membersTitle', { count: group.members.length })}
            </Text>
            {group.members.length === 0 ? (
              <Text size="sm" c="dimmed">
                {t('reviews.noMembers')}
              </Text>
            ) : (
              group.members.map((member) => (
                <Group key={member.id} gap="xs" justify="space-between">
                  <Group gap="xs">
                    <Badge size="sm" variant="light">
                      {member.full_name}
                    </Badge>
                    <Text size="xs" c="dimmed">
                      {member.email} · {member.role}
                    </Text>
                  </Group>
                  <Tooltip label={t('reviews.removeMemberNamed', { name: member.full_name })}>
                    <ActionIcon
                      size="sm"
                      variant="subtle"
                      color="red"
                      disabled={isPending}
                      onClick={() => {
                        onRemoveMember(group, member.id);
                      }}
                      aria-label={t('reviews.removeMemberNamed', { name: member.full_name })}
                    >
                      <IconUserMinus size={14} />
                    </ActionIcon>
                  </Tooltip>
                </Group>
              ))
            )}
            <Group gap="xs" align="flex-end">
              <Select
                label={t('reviews.addMember')}
                placeholder={t('reviews.addMemberPlaceholder')}
                disabled={isPending || usersLoading}
                data={userOptions}
                searchable
                value={selectedUser}
                onChange={(next) => {
                  setSelectedUser(next ?? '');
                }}
                style={{ flex: '1 1 0' }}
                aria-label={t('reviews.addMember')}
              />
              <Button
                size="sm"
                disabled={isPending || selectedUser === ''}
                onClick={() => {
                  onAddMember(group, selectedUser);
                  setSelectedUser('');
                }}
              >
                {t('reviews.add')}
              </Button>
            </Group>
            <Text size="xs" c="dimmed">
              {t('reviews.removeMemberGuardHint')}
            </Text>
          </Stack>
        </Stack>
      )}
    </Modal>
  );
}
