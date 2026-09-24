import { NumberInput, Select, Stack, Switch, Text } from '@mantine/core';
import { useTranslation } from 'react-i18next';
import { MAX_WINDOW_DAYS, MIN_WINDOW_DAYS } from '../lib/aggregation';
import type { AttributionDraft } from '../lib/aggregation';

interface AttributionEditorProps {
  draft: AttributionDraft;
  windowError: boolean;
  fallbackError: boolean;
  disabled: boolean;
  onChange: (draft: AttributionDraft) => void;
}

export function AttributionEditor({ draft, windowError, fallbackError, disabled, onChange }: AttributionEditorProps) {
  const { t } = useTranslation();

  return (
    <Stack gap="xs">
      <Text size="sm" fw={500}>
        {t('metrics.attribution')}
      </Text>
      <Switch
        label={t('metrics.requireExposure')}
        disabled={disabled}
        checked={draft.requireExposure}
        onChange={(event) => {
          onChange({ ...draft, requireExposure: event.currentTarget.checked });
        }}
      />
      <NumberInput
        label={t('metrics.windowDays')}
        description={t('metrics.windowDaysHint', { min: MIN_WINDOW_DAYS, max: MAX_WINDOW_DAYS })}
        withAsterisk
        disabled={disabled}
        min={MIN_WINDOW_DAYS}
        max={MAX_WINDOW_DAYS}
        value={draft.windowDaysRaw === '' ? '' : Number(draft.windowDaysRaw)}
        onChange={(value) => {
          onChange({ ...draft, windowDaysRaw: typeof value === 'number' ? String(value) : '' });
        }}
        error={windowError ? t('metrics.attributionErrors.windowDays') : undefined}
      />
      <Select
        label={t('metrics.fallback')}
        withAsterisk
        disabled={disabled}
        data={[
          { value: 'none', label: t('metrics.fallbacks.none') },
          { value: 'subject', label: t('metrics.fallbacks.subject') },
        ]}
        value={draft.fallback}
        onChange={(next) => {
          onChange({ ...draft, fallback: (next ?? '') as AttributionDraft['fallback'] });
        }}
        error={fallbackError ? t('metrics.attributionErrors.fallback') : undefined}
      />
    </Stack>
  );
}
