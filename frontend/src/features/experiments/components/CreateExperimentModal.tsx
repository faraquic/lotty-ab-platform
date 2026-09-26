import { Button, Group, Modal, NumberInput, Select, Stack, Textarea, TextInput } from '@mantine/core';
import { useForm } from '@mantine/form';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import type { FlagType } from '@/features/flags/types';
import { VariantsEditor } from './VariantsEditor';
import {
  DEFAULT_WEIGHTS_TOTAL,
  MAX_DESCRIPTION_LENGTH,
  MAX_NAME_LENGTH,
  normalizePercentInput,
  parseTargetingInput,
  validateVariantDrafts,
} from '../lib/transitions';
import type { VariantDraft } from '../lib/transitions';
import type { CreateExperimentRequest } from '../types';

export interface CreateExperimentFormValues {
  flag_id: string;
  name: string;
  description: string;
  weightsTotalRaw: string;
  targetingRaw: string;
}

export interface FlagOption {
  value: string;
  label: string;
  key: string;
  type: FlagType;
}

interface CreateExperimentModalProps {
  opened: boolean;
  isPending: boolean;
  flagOptions: FlagOption[];
  flagsLoading: boolean;
  onClose: () => void;
  onSubmit: (request: CreateExperimentRequest) => void;
}

function initialDrafts(): VariantDraft[] {
  return [
    { name: '', valueRaw: '', weightRaw: '', isControl: true },
    { name: '', valueRaw: '', weightRaw: '', isControl: false },
  ];
}

function draftsTouched(drafts: VariantDraft[]): boolean {
  return drafts.some(
    (draft) =>
      draft.name.trim().length > 0 ||
      draft.valueRaw.trim().length > 0 ||
      draft.weightRaw.trim().length > 0,
  );
}

export function CreateExperimentModal({
  opened,
  isPending,
  flagOptions,
  flagsLoading,
  onClose,
  onSubmit,
}: CreateExperimentModalProps) {
  const { t, i18n } = useTranslation();
  const [drafts, setDrafts] = useState<VariantDraft[]>(initialDrafts);

  const form = useForm<CreateExperimentFormValues>({
    initialValues: {
      flag_id: '',
      name: '',
      description: '',
      weightsTotalRaw: String(DEFAULT_WEIGHTS_TOTAL / 100),
      targetingRaw: '',
    },
    validate: {
      flag_id: (value) => (value.length === 0 ? i18n.t('experiments.flagRequired') : null),
      name: (value) => {
        if (value.trim().length === 0) {
          return i18n.t('experiments.nameRequired');
        }
        if (value.length > MAX_NAME_LENGTH) {
          return i18n.t('experiments.nameMaxLength', { count: MAX_NAME_LENGTH });
        }
        return null;
      },
      description: (value) =>
        value.length > MAX_DESCRIPTION_LENGTH
          ? i18n.t('experiments.descriptionMaxLength', { count: MAX_DESCRIPTION_LENGTH })
          : null,
      weightsTotalRaw: (value) =>
        normalizePercentInput(value) === null ? i18n.t('experiments.weightsTotalInvalid') : null,
      targetingRaw: (value) =>
        parseTargetingInput(value).ok ? null : i18n.t('experiments.targetingInvalid'),
    },
  });

  const weightsTotal = normalizePercentInput(form.values.weightsTotalRaw) ?? DEFAULT_WEIGHTS_TOTAL;
  const selectedFlagType = flagOptions.find((option) => option.value === form.values.flag_id)?.type ?? 'string';
  const touched = draftsTouched(drafts);
  const validation = validateVariantDrafts(drafts, weightsTotal, selectedFlagType);

  const handleSubmit = (values: CreateExperimentFormValues): void => {
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
    if (touched && validation.variants === null) {
      return;
    }
    const description = values.description.trim();
    onSubmit({
      flag_id: values.flag_id,
      name: values.name.trim(),
      ...(description.length > 0 ? { description } : {}),
      weights_total: total,
      ...(targetingParsed.targeting !== null ? { targeting: targetingParsed.targeting } : {}),
      ...(validation.variants !== null ? { variants: validation.variants } : {}),
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
    <Modal opened={opened} onClose={handleClose} title={t('experiments.createExperiment')} centered size="lg">
      <form onSubmit={form.onSubmit(handleSubmit)} noValidate>
        <Stack gap="md">
          <Select
            label={t('experiments.flag')}
            placeholder={t('experiments.flagPlaceholder')}
            withAsterisk
            disabled={isPending || flagsLoading}
            data={flagOptions.map(({ value, label }) => ({ value, label }))}
            searchable
            filter={({ options, search }) => {
              const needle = search.trim().toLowerCase();
              if (needle.length === 0) {
                return options;
              }
              return options.filter((option) => {
                if (!('value' in option)) {
                  return false;
                }
                const source = flagOptions.find(
                  (flagOption) => flagOption.value === option.value,
                );
                if (source === undefined) {
                  return false;
                }
                return (
                  source.label.toLowerCase().includes(needle) ||
                  source.key.toLowerCase().includes(needle)
                );
              });
            }}
            {...form.getInputProps('flag_id')}
          />
          <TextInput
            label={t('experiments.name')}
            placeholder={t('experiments.namePlaceholder')}
            withAsterisk
            disabled={isPending}
            {...form.getInputProps('name')}
          />
          <Textarea
            label={t('experiments.description')}
            placeholder={t('experiments.descriptionPlaceholder')}
            disabled={isPending}
            autosize
            minRows={2}
            {...form.getInputProps('description')}
          />
          <NumberInput
            label={t('experiments.weightsTotal')}
            description={t('experiments.weightsTotalHint')}
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
            flagType={selectedFlagType}
            validationError={touched ? validation.error : null}
            weightsSum={validation.weightsSum}
            disabled={isPending}
            onChange={setDrafts}
          />
          <Group justify="flex-end" gap="sm">
            <Button variant="subtle" onClick={handleClose} disabled={isPending}>
              {t('experiments.cancel')}
            </Button>
            <Button type="submit" loading={isPending} disabled={isPending || !form.isValid()}>
              {t('experiments.create')}
            </Button>
          </Group>
        </Stack>
      </form>
    </Modal>
  );
}
