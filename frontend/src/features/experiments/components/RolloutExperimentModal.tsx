import { Button, Group, Modal, Select, Stack, Textarea } from '@mantine/core';
import { useForm } from '@mantine/form';
import { useTranslation } from 'react-i18next';

export interface RolloutFormValues {
  reason: string;
  winner_variant_id: string;
}

interface RolloutExperimentModalProps {
  opened: boolean;
  isPending: boolean;
  variantOptions: { value: string; label: string }[];
  onClose: () => void;
  onSubmit: (values: RolloutFormValues) => void;
}

const MAX_REASON_LENGTH = 4096;

export function RolloutExperimentModal({
  opened,
  isPending,
  variantOptions,
  onClose,
  onSubmit,
}: RolloutExperimentModalProps) {
  const { t, i18n } = useTranslation();

  const form = useForm<RolloutFormValues>({
    initialValues: { reason: '', winner_variant_id: '' },
    validate: {
      reason: (value) => {
        if (value.trim().length === 0) {
          return i18n.t('experiments.reasonRequired');
        }
        if (value.length > MAX_REASON_LENGTH) {
          return i18n.t('experiments.reasonMaxLength', { count: MAX_REASON_LENGTH });
        }
        return null;
      },
      winner_variant_id: (value) =>
        value === '' ? i18n.t('experiments.winnerRequired') : null,
    },
  });

  const handleClose = (): void => {
    if (!isPending) {
      form.reset();
      onClose();
    }
  };

  return (
    <Modal opened={opened} onClose={handleClose} title={t('experiments.rolloutTitle')} centered>
      <form
        onSubmit={form.onSubmit((values) => {
          if (!isPending) {
            onSubmit(values);
          }
        })}
        noValidate
      >
        <Stack gap="md">
          <Select
            label={t('experiments.winnerVariant')}
            placeholder={t('experiments.winnerPlaceholder')}
            withAsterisk
            disabled={isPending}
            data={variantOptions}
            {...form.getInputProps('winner_variant_id')}
          />
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
              {t('experiments.rolloutAction')}
            </Button>
          </Group>
        </Stack>
      </form>
    </Modal>
  );
}
