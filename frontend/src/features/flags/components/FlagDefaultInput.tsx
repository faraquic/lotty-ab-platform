import { Select, TextInput } from '@mantine/core';
import { useTranslation } from 'react-i18next';
import type { ReactNode } from 'react';
import type { FlagType } from '../types';

interface FlagDefaultInputProps {
  flagType: FlagType;
  value: string;
  onChange: (value: string) => void;
  onBlur: () => void;
  error: ReactNode;
  disabled: boolean;
}

export function FlagDefaultInput({
  flagType,
  value,
  onChange,
  onBlur,
  error,
  disabled,
}: FlagDefaultInputProps) {
  const { t } = useTranslation();

  if (flagType === 'bool') {
    return (
      <Select
        label={t('flags.defaultValue')}
        withAsterisk
        disabled={disabled}
        placeholder={t('flags.defaultBoolPlaceholder')}
        data={[
          { value: 'true', label: 'true' },
          { value: 'false', label: 'false' },
        ]}
        value={value === '' ? null : value}
        onChange={(next) => {
          onChange(next ?? '');
        }}
        onBlur={onBlur}
        error={error}
      />
    );
  }

  return (
    <TextInput
      label={t('flags.defaultValue')}
      withAsterisk
      disabled={disabled}
      placeholder={
        flagType === 'number' ? t('flags.defaultNumberPlaceholder') : t('flags.defaultStringPlaceholder')
      }
      description={flagType === 'number' ? t('flags.defaultNumberHint') : undefined}
      inputMode={flagType === 'number' ? 'decimal' : undefined}
      value={value}
      onChange={(event) => {
        onChange(event.currentTarget.value);
      }}
      onBlur={onBlur}
      error={error}
    />
  );
}
