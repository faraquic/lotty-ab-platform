import { NumberInput, Stack, Text, TextInput } from '@mantine/core';
import { useTranslation } from 'react-i18next';
import type { AggregationDraft, AggregationError } from '../lib/aggregation';
import type { MetricType } from '../types';

interface AggregationEditorProps {
  metricType: MetricType;
  draft: AggregationDraft;
  error: AggregationError;
  disabled: boolean;
  onChange: (draft: AggregationDraft) => void;
}

export function AggregationEditor({ metricType, draft, error, disabled, onChange }: AggregationEditorProps) {
  const { t } = useTranslation();

  const set = (patch: Partial<AggregationDraft>): void => {
    onChange({ ...draft, ...patch });
  };

  if (metricType === 'ratio') {
    return (
      <Stack gap="xs">
        <Text size="sm" fw={500}>
          {t('metrics.aggregation')}
        </Text>
        <TextInput
          label={t('metrics.numeratorEvent')}
          placeholder={t('metrics.eventPlaceholder')}
          disabled={disabled}
          value={draft.numeratorEventType}
          onChange={(event) => {
            set({ numeratorEventType: event.currentTarget.value });
          }}
          error={error === 'numerator' ? t('metrics.aggregationErrors.numerator') : undefined}
        />
        <TextInput
          label={t('metrics.numeratorField')}
          placeholder={t('metrics.fieldPlaceholderOptional')}
          disabled={disabled}
          value={draft.numeratorField}
          onChange={(event) => {
            set({ numeratorField: event.currentTarget.value });
          }}
        />
        <TextInput
          label={t('metrics.denominatorEvent')}
          placeholder={t('metrics.eventPlaceholder')}
          disabled={disabled}
          value={draft.denominatorEventType}
          onChange={(event) => {
            set({ denominatorEventType: event.currentTarget.value });
          }}
          error={error === 'denominator' ? t('metrics.aggregationErrors.denominator') : undefined}
        />
        <TextInput
          label={t('metrics.denominatorField')}
          placeholder={t('metrics.fieldPlaceholderOptional')}
          disabled={disabled}
          value={draft.denominatorField}
          onChange={(event) => {
            set({ denominatorField: event.currentTarget.value });
          }}
        />
      </Stack>
    );
  }

  const needsField = metricType === 'sum' || metricType === 'average' || metricType === 'percentile';
  const needsLevel = metricType === 'percentile';

  return (
    <Stack gap="xs">
      <Text size="sm" fw={500}>
        {t('metrics.aggregation')}
      </Text>
      <TextInput
        label={t('metrics.eventType')}
        placeholder={t('metrics.eventPlaceholder')}
        withAsterisk
        disabled={disabled}
        value={draft.eventType}
        onChange={(event) => {
          set({ eventType: event.currentTarget.value });
        }}
        error={error === 'eventType' ? t('metrics.aggregationErrors.eventType') : undefined}
      />
      {needsField ? (
        <TextInput
          label={t('metrics.field')}
          placeholder={t('metrics.fieldPlaceholder')}
          withAsterisk
          disabled={disabled}
          value={draft.field}
          onChange={(event) => {
            set({ field: event.currentTarget.value });
          }}
          error={error === 'field' ? t('metrics.aggregationErrors.field') : undefined}
        />
      ) : null}
      {needsLevel ? (
        <NumberInput
          label={t('metrics.level')}
          description={t('metrics.levelHint')}
          withAsterisk
          disabled={disabled}
          min={0}
          max={1}
          step={0.05}
          value={draft.levelRaw === '' ? '' : Number(draft.levelRaw)}
          onChange={(value) => {
            set({ levelRaw: typeof value === 'number' ? String(value) : '' });
          }}
          error={error === 'level' ? t('metrics.aggregationErrors.level') : undefined}
        />
      ) : null}
    </Stack>
  );
}
