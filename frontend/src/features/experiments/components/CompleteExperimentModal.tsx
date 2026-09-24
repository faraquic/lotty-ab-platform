import { Button, Group, Modal, Select, Stack, Textarea } from '@mantine/core';
import { useForm } from '@mantine/form';
import { useTranslation } from 'react-i18next';
import { COMPLETION_DECISIONS } from '../types';
import type { CompletionDecision } from '../types';

export interface CompleteFormValues {
  decision: CompletionDecision | '';
  reason: string;
  winner_variant_id: string;
}

interface CompleteExperimentModalProps {
  opened: boolean;
  isPending: boolean;
  variantOptions: { value: string; label: string }[];
  onClose: () => void;
  onSubmit: (values: CompleteFormValues) => void;
}

const MAX_REASON_LENGTH = 4096;

export function CompleteExperimentModal({
  opened,
  isPending,
  variantOptions,
  onClose,
  onSubmit,
}: CompleteExperimentModalProps) {
  const { t, i18n } = useTranslation();

  const form = useForm<CompleteFormValues>({
    initialValues: { decision: '', reason: '', winner_variant_id: '' },
    validate: {
      decision: (value) => (value === '' ? i18n.t('experiments.decisionRequired') : null),
      reason: (value) => {
        if (value.trim().length === 0) {
          return i18n.t('experiments.reasonRequired');
        }
        if (value.length > MAX_REASON_LENGTH) {
          return i18n.t('experiments.reasonMaxLength', { count: MAX_REASON_LENGTH });
        }
        return null;
      },
      winner_variant_id: (value, values) =>
        values.decision === 'rollout_winner' && value === ''
          ? i18n.t('experiments.winnerRequired')
          : null,
    },
  });

  const handleSubmit = (values: CompleteFormValues): void => {
    if (!isPending && values.decision !== '') {
      onSubmit(values);
    }
  };

  const handleClose = (): void => {
    if (!isPending) {
      form.reset();
      onClose();
    }
  };

  return (
    <Modal opened={opened} onClose={handleClose} title={t('experiments.completeTitle')} centered>
      <form onSubmit={form.onSubmit(handleSubmit)} noValidate>
        <Stack gap="md">
          <Select
            label={t('experiments.decision')}
            placeholder={t('experiments.decisionPlaceholder')}
            withAsterisk
            disabled={isPending}
            data={COMPLETION_DECISIONS.map((decision) => ({
              value: decision,
              label: t(`experiments.decisions.${decision}`),
            }))}
            {...form.getInputProps('decision')}
            onChange={(next) => {
              form.setFieldValue('decision', (next ?? '') as CompletionDecision | '');
            }}
          />
          {form.values.decision === 'rollout_winner' ? (
            <Select
              label={t('experiments.winnerVariant')}
              placeholder={t('experiments.winnerPlaceholder')}
              withAsterisk
              disabled={isPending}
              data={variantOptions}
              {...form.getInputProps('winner_variant_id')}
            />
          ) : null}
          <Textarea
            label={t('experiments.reason')}
            placeholder={t('experiments.reasonPlaceholder')}
            withAsterisk
            disabled={isPending}
            autosize
            minRows={3}
            {...form.getInputProps('reason')}
          />
          <Group justify="flex-end" gap="sm">
            <Button variant="subtle" onClick={handleClose} disabled={isPending}>
              {t('experiments.cancel')}
            </Button>
            <Button type="submit" loading={isPending} disabled={isPending || !form.isValid()}>
              {t('experiments.completeAction')}
            </Button>
          </Group>
        </Stack>
      </form>
    </Modal>
  );
}
