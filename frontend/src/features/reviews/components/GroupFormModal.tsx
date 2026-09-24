import { Button, Group, Modal, NumberInput, Select, Stack, Textarea, TextInput } from '@mantine/core';
import { useForm } from '@mantine/form';
import { useTranslation } from 'react-i18next';
import { GROUP_STATUSES } from '../types';
import type { ApproverGroup, ApproverGroupStatus, CreateApproverGroupRequest, UpdateApproverGroupRequest } from '../types';

export interface GroupFormValues {
  name: string;
  description: string;
  minApprovalsRaw: string;
  status: ApproverGroupStatus | '';
}

interface GroupFormModalProps {
  group: ApproverGroup | null;
  opened: boolean;
  isPending: boolean;
  onClose: () => void;
  onCreate: (request: CreateApproverGroupRequest) => void;
  onUpdate: (id: string, request: UpdateApproverGroupRequest) => void;
}

const MAX_NAME_LENGTH = 256;
const MAX_DESCRIPTION_LENGTH = 4096;

export function GroupFormModal({ group, opened, isPending, onClose, onCreate, onUpdate }: GroupFormModalProps) {
  const { t, i18n } = useTranslation();
  const editing = group !== null;

  const form = useForm<GroupFormValues>({
    initialValues: {
      name: group?.name ?? '',
      description: group?.description ?? '',
      minApprovalsRaw: String(group?.min_approvals ?? 1),
      status: group?.status ?? '',
    },
    validate: {
      name: (value) => {
        if (value.trim().length === 0) {
          return i18n.t('reviews.groupNameRequired');
        }
        if (value.length > MAX_NAME_LENGTH) {
          return i18n.t('reviews.groupNameMaxLength', { count: MAX_NAME_LENGTH });
        }
        return null;
      },
      description: (value) =>
        value.length > MAX_DESCRIPTION_LENGTH
          ? i18n.t('reviews.descriptionMaxLength', { count: MAX_DESCRIPTION_LENGTH })
          : null,
      minApprovalsRaw: (value) => {
        const parsed = Number(value);
        if (!Number.isInteger(parsed) || parsed < 1) {
          return i18n.t('reviews.minApprovalsInvalid');
        }
        return null;
      },
    },
  });

  const handleSubmit = (values: GroupFormValues): void => {
    if (isPending) {
      return;
    }
    const minApprovals = Number(values.minApprovalsRaw);
    if (!Number.isInteger(minApprovals) || minApprovals < 1) {
      return;
    }
    const description = values.description.trim();
    if (!editing) {
      onCreate({
        name: values.name.trim(),
        min_approvals: minApprovals,
        ...(description.length > 0 ? { description } : {}),
      });
      return;
    }
    const request: UpdateApproverGroupRequest = {};
    if (values.name.trim() !== group.name) {
      request.name = values.name.trim();
    }
    if (description !== (group.description ?? '')) {
      request.description = description.length > 0 ? description : null;
    }
    if (minApprovals !== group.min_approvals) {
      request.min_approvals = minApprovals;
    }
    if (values.status !== '' && values.status !== group.status) {
      request.status = values.status;
    }
    if (Object.keys(request).length === 0) {
      return;
    }
    onUpdate(group.id, request);
  };

  const handleClose = (): void => {
    if (!isPending) {
      form.reset();
      onClose();
    }
  };

  return (
    <Modal
      key={group?.id ?? 'new'}
      opened={opened}
      onClose={handleClose}
      title={t(editing ? 'reviews.editGroup' : 'reviews.createGroup')}
      centered
    >
      <form onSubmit={form.onSubmit(handleSubmit)} noValidate>
        <Stack gap="md">
          <TextInput
            label={t('reviews.groupName')}
            placeholder={t('reviews.groupNamePlaceholder')}
            withAsterisk
            disabled={isPending}
            {...form.getInputProps('name')}
          />
          <Textarea
            label={t('reviews.groupDescription')}
            placeholder={t('reviews.groupDescriptionPlaceholder')}
            disabled={isPending}
            autosize
            minRows={2}
            {...form.getInputProps('description')}
          />
          <NumberInput
            label={t('reviews.minApprovals')}
            disabled={isPending}
            min={1}
            value={form.values.minApprovalsRaw === '' ? '' : Number(form.values.minApprovalsRaw)}
            onChange={(value) => {
              form.setFieldValue('minApprovalsRaw', typeof value === 'number' ? String(value) : '');
            }}
            error={form.errors.minApprovalsRaw}
          />
          {editing ? (
            <Select
              label={t('reviews.groupStatus')}
              disabled={isPending}
              data={GROUP_STATUSES.map((status) => ({
                value: status,
                label: t(`reviews.groupStatuses.${status}`),
              }))}
              {...form.getInputProps('status')}
              onChange={(next) => {
                form.setFieldValue('status', (next ?? '') as ApproverGroupStatus | '');
              }}
            />
          ) : null}
          <Group justify="flex-end" gap="sm">
            <Button variant="subtle" onClick={handleClose} disabled={isPending}>
              {t('reviews.cancel')}
            </Button>
            <Button type="submit" loading={isPending} disabled={isPending || !form.isValid()}>
              {editing ? t('reviews.save') : t('reviews.create')}
            </Button>
          </Group>
        </Stack>
      </form>
    </Modal>
  );
}
