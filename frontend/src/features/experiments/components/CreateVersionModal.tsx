import { Button, Group, Modal, NumberInput, Stack, Textarea } from '@mantine/core';
import { useForm } from '@mantine/form';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import type { FlagType } from '@/features/flags/types';
import { VariantsEditor } from './VariantsEditor';
import {
  DEFAULT_WEIGHTS_TOTAL,
  normalizePercentInput,
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
  flagType: FlagType;
  currentWeightsTotal: number;
  currentTargeting: string | null;
  onClose: () => void;
  onSubmit: (request: CreateExperimentVersionRequest) => void;
}

function initialDrafts(): VariantDraft[] {
  return [
    { name: '', valueRaw: '', weightRaw: '', isControl: true },
    { name: '', valueRaw: '', weightRaw: '', isControl: false },
  ];
}

function defaultWeightsRaw(bp: number): string {
  return String(bp / 100);
}

export function CreateVersionModal({
  opened,
  isPending,
  flagType,
  currentWeightsTotal,
  currentTargeting,
  onClose,
  onSubmit,
}: CreateVersionModalProps) {
  const { t, i18n } = useTranslation();
  const [drafts, setDrafts] = useState<VariantDraft[]>(initialDrafts);

  const form = useForm<VersionFormValues>({
    initialValues: {
      weightsTotalRaw: defaultWeightsRaw(currentWeightsTotal),
      targetingRaw: currentTargeting ?? '',
    },
    validate: {
      weightsTotalRaw: (value) =>
        normalizePercentInput(value) === null ? i18n.t('experiments.weightsTotalInvalid') : null,
      targetingRaw: (value) =>
        parseTargetingInput(value).ok ? null : i18n.t('experiments.targetingInvalid'),
    },
  });

  const weightsTotal = normalizePercentInput(form.values.weightsTotalRaw) ?? DEFAULT_WEIGHTS_TOTAL;
  const validation = validateVariantDrafts(drafts, weightsTotal, flagType);

  const handleSubmit = (values: VersionFormValues): void => {
    if (isPending) {
      return;
    }
    const total = normalizePercentInput(values.weightsTotalRaw);
    if (total === null) {
      return;
    }
    const targetingParsed = parseTargetingInput(values.targetingRaw);
    if (!targetingParsed.ok) {
      return;
    }
    if (validation.variants === null) {
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
      form.reset();
      setDrafts(initialDrafts());
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
            min={0.01}
            max={100}
            decimalScale={2}
            suffix="%"
            value={form.values.weightsTotalRaw === '' ? '' : Number(form.values.weightsTotalRaw)}
            onChange={(value) => {
              form.setFieldValue('weightsTotalRaw', typeof value === 'number' ? String(value) : '');
            }}
            error={form.errors.weightsTotalRaw}
          />
          <Textarea
            label={t('experiments.targeting')}
            placeholder={t('experiments.targetingPlaceholder')}
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
            flagType={flagType}
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
