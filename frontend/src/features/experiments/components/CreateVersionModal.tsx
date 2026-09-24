import { Button, Group, Modal, NumberInput, Stack, Textarea } from '@mantine/core';
import { useForm } from '@mantine/form';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { VariantsEditor } from './VariantsEditor';
import {
  MAX_WEIGHTS_TOTAL,
  normalizeWeightsTotal,
  parseTargetingInput,
  validateVariantDrafts,
} from '../lib/transitions';
import type { VariantDraft } from '../lib/transitions';
import type { CreateExperimentVersionRequest } from '../types';

export interface VersionFormValues {
  weightsTotalRaw: string;
  targetingRaw: string;
}

interface CreateVersionModalProps {
  opened: boolean;
  isPending: boolean;
  currentWeightsTotal: number;
  currentTargeting: Record<string, unknown> | null;
  onClose: () => void;
  onSubmit: (request: CreateExperimentVersionRequest) => void;
}

export function CreateVersionModal({
  opened,
  isPending,
  currentWeightsTotal,
  currentTargeting,
  onClose,
  onSubmit,
}: CreateVersionModalProps) {
  const { t, i18n } = useTranslation();
  const [drafts, setDrafts] = useState<VariantDraft[]>([
    { name: '', valueRaw: '', weightRaw: '', isControl: true },
    { name: '', valueRaw: '', weightRaw: '', isControl: false },
  ]);

  const form = useForm<VersionFormValues>({
    initialValues: {
      weightsTotalRaw: String(currentWeightsTotal),
      targetingRaw: currentTargeting === null ? '' : JSON.stringify(currentTargeting, null, 2),
    },
    validate: {
      weightsTotalRaw: (value) =>
        normalizeWeightsTotal(value) === null ? i18n.t('experiments.weightsTotalInvalid') : null,
      targetingRaw: (value) =>
        parseTargetingInput(value).ok ? null : i18n.t('experiments.targetingInvalid'),
    },
  });

  const weightsTotal = normalizeWeightsTotal(form.values.weightsTotalRaw) ?? currentWeightsTotal;
  const validation = validateVariantDrafts(drafts, weightsTotal);

  const handleSubmit = (values: VersionFormValues): void => {
    if (isPending || validation.variants === null) {
      return;
    }
    const total = normalizeWeightsTotal(values.weightsTotalRaw);
    if (total === null) {
      return;
    }
    const targetingParsed = parseTargetingInput(values.targetingRaw);
    if (!targetingParsed.ok) {
      return;
    }
    onSubmit({
      version: 0,
      weights_total: total,
      ...(targetingParsed.targeting !== null ? { targeting: targetingParsed.targeting } : {}),
      variants: validation.variants,
    });
  };

  const handleClose = (): void => {
    if (!isPending) {
      onClose();
    }
  };

  return (
    <Modal opened={opened} onClose={handleClose} title={t('experiments.newVersion')} centered size="lg">
      <form onSubmit={form.onSubmit(handleSubmit)} noValidate>
        <Stack gap="md">
          <NumberInput
            label={t('experiments.weightsTotal')}
            disabled={isPending}
            min={1}
            max={MAX_WEIGHTS_TOTAL}
            value={form.values.weightsTotalRaw === '' ? '' : Number(form.values.weightsTotalRaw)}
            onChange={(value) => {
              form.setFieldValue('weightsTotalRaw', typeof value === 'number' ? String(value) : '');
            }}
            error={form.errors.weightsTotalRaw}
          />
          <Textarea
            label={t('experiments.targeting')}
            description={t('experiments.targetingHint')}
            disabled={isPending}
            autosize
            minRows={2}
            spellCheck={false}
            {...form.getInputProps('targetingRaw')}
          />
          <VariantsEditor
            drafts={drafts}
            weightsTotal={weightsTotal}
            validationError={validation.error}
            weightsSum={validation.weightsSum}
            disabled={isPending}
            onChange={setDrafts}
          />
          <Group justify="flex-end" gap="sm">
            <Button variant="subtle" onClick={handleClose} disabled={isPending}>
              {t('experiments.cancel')}
            </Button>
            <Button
              type="submit"
              loading={isPending}
              disabled={isPending || !form.isValid() || validation.variants === null}
            >
              {t('experiments.createVersion')}
            </Button>
          </Group>
        </Stack>
      </form>
    </Modal>
  );
}
