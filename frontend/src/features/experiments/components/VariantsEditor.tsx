import { ActionIcon, Button, Group, NumberInput, Stack, Switch, Text, TextInput } from '@mantine/core';
import { IconPlus, IconTrash } from '@tabler/icons-react';
import { useTranslation } from 'react-i18next';
import type { VariantDraft, VariantsValidation } from '../lib/transitions';

interface VariantsEditorProps {
  drafts: VariantDraft[];
  weightsTotal: number;
  validationError: VariantsValidation['error'];
  weightsSum: number;
  disabled: boolean;
  onChange: (drafts: VariantDraft[]) => void;
}

export function VariantsEditor({
  drafts,
  weightsTotal,
  validationError,
  weightsSum,
  disabled,
  onChange,
}: VariantsEditorProps) {
  const { t } = useTranslation();

  const setDraft = (index: number, patch: Partial<VariantDraft>): void => {
    onChange(drafts.map((draft, i) => (i === index ? { ...draft, ...patch } : draft)));
  };

  const markControl = (index: number): void => {
    onChange(drafts.map((draft, i) => ({ ...draft, isControl: i === index })));
  };

  const removeDraft = (index: number): void => {
    onChange(drafts.filter((_, i) => i !== index));
  };

  return (
    <Stack gap="xs">
      {drafts.map((draft, index) => (
        <Group key={index} gap="xs" align="flex-end" wrap="nowrap">
          <TextInput
            label={index === 0 ? t('experiments.variantName') : undefined}
            placeholder={t('experiments.variantNamePlaceholder')}
            disabled={disabled}
            value={draft.name}
            onChange={(event) => {
              setDraft(index, { name: event.currentTarget.value });
            }}
            style={{ flex: '1 1 0' }}
            aria-label={t('experiments.variantName')}
          />
          <TextInput
            label={index === 0 ? t('experiments.variantValue') : undefined}
            placeholder={t('experiments.variantValuePlaceholder')}
            disabled={disabled}
            value={draft.valueRaw}
            onChange={(event) => {
              setDraft(index, { valueRaw: event.currentTarget.value });
            }}
            style={{ flex: '1 1 0' }}
            aria-label={t('experiments.variantValue')}
          />
          <NumberInput
            label={index === 0 ? t('experiments.variantWeight') : undefined}
            placeholder="5000"
            disabled={disabled}
            value={draft.weightRaw === '' ? '' : Number(draft.weightRaw)}
            onChange={(value) => {
              setDraft(index, { weightRaw: typeof value === 'number' ? String(value) : '' });
            }}
            min={1}
            style={{ flex: '0 0 110px' }}
            aria-label={t('experiments.variantWeight')}
          />
          <Switch
            label={t('experiments.variantControl')}
            disabled={disabled}
            checked={draft.isControl}
            onChange={() => {
              markControl(index);
            }}
            aria-label={t('experiments.variantControl')}
          />
          <ActionIcon
            variant="subtle"
            color="red"
            disabled={disabled || drafts.length <= 2}
            onClick={() => {
              removeDraft(index);
            }}
            aria-label={t('experiments.removeVariant')}
          >
            <IconTrash size={16} />
          </ActionIcon>
        </Group>
      ))}
      <Group justify="space-between" align="center">
        <Text size="xs" c={weightsSum === weightsTotal ? 'dimmed' : 'orange'}>
          {t('experiments.weightsSum', { sum: weightsSum, total: weightsTotal })}
        </Text>
        <Button
          size="xs"
          variant="subtle"
          leftSection={<IconPlus size={14} />}
          disabled={disabled}
          onClick={() => {
            onChange([
              ...drafts,
              { name: '', valueRaw: '', weightRaw: '', isControl: false },
            ]);
          }}
        >
          {t('experiments.addVariant')}
        </Button>
      </Group>
      {validationError !== null ? (
        <Text size="xs" c="red">
          {t(`experiments.variantErrors.${validationError}`)}
        </Text>
      ) : null}
    </Stack>
  );
}
