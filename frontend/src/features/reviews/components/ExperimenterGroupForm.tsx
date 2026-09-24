import { Button, Group, Select, Stack, Text } from '@mantine/core';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';

interface ExperimenterGroupFormProps {
  experimenterOptions: { value: string; label: string }[];
  groupOptions: { value: string; label: string }[];
  selectorsLoading: boolean;
  isPending: boolean;
  onAssign: (experimenterId: string, groupId: string | null) => void;
}

export function ExperimenterGroupForm({
  experimenterOptions,
  groupOptions,
  selectorsLoading,
  isPending,
  onAssign,
}: ExperimenterGroupFormProps) {
  const { t } = useTranslation();
  const [experimenterId, setExperimenterId] = useState('');
  const [groupId, setGroupId] = useState('');

  return (
    <Stack gap="xs">
      <Text size="sm" fw={600}>
        {t('reviews.experimenterGroupTitle')}
      </Text>
      <Text size="xs" c="dimmed">
        {t('reviews.experimenterGroupHint')}
      </Text>
      <Group gap="xs" align="flex-end">
        <Select
          label={t('reviews.experimenter')}
          placeholder={t('reviews.experimenterPlaceholder')}
          disabled={isPending || selectorsLoading}
          data={experimenterOptions}
          searchable
          value={experimenterId}
          onChange={(next) => {
            setExperimenterId(next ?? '');
          }}
          style={{ flex: '1 1 0' }}
          aria-label={t('reviews.experimenter')}
        />
        <Select
          label={t('reviews.approverGroup')}
          placeholder={t('reviews.approverGroupPlaceholder')}
          disabled={isPending || selectorsLoading}
          data={groupOptions}
          searchable
          clearable
          value={groupId}
          onChange={(next) => {
            setGroupId(next ?? '');
          }}
          style={{ flex: '1 1 0' }}
          aria-label={t('reviews.approverGroup')}
        />
        <Button
          size="sm"
          disabled={isPending || experimenterId === ''}
          onClick={() => {
            onAssign(experimenterId, groupId === '' ? null : groupId);
          }}
        >
          {t('reviews.assign')}
        </Button>
      </Group>
    </Stack>
  );
}
