import { Button, Group, Modal, Select, Stack, Textarea, TextInput } from '@mantine/core';
import { useForm } from '@mantine/form';
import { useTranslation } from 'react-i18next';
import { FlagDefaultInput } from './FlagDefaultInput';
import { parseDefaultValueInput } from '../lib/flagValue';
import { FLAG_TYPES, isFlagType } from '../types';
import type { CreateFlagRequest, FlagType } from '../types';

const MIN_KEY_LENGTH = 3;
const MAX_KEY_LENGTH = 128;
const MAX_NAME_LENGTH = 256;
const MAX_DESCRIPTION_LENGTH = 4096;

export interface CreateFlagFormValues {
  key: string;
  name: string;
  type: FlagType | '';
  defaultRaw: string;
  description: string;
}

interface CreateFlagModalProps {
  opened: boolean;
  isPending: boolean;
  onClose: () => void;
  onSubmit: (request: CreateFlagRequest) => void;
}

export function CreateFlagModal({ opened, isPending, onClose, onSubmit }: CreateFlagModalProps) {
  const { t, i18n } = useTranslation();

  const form = useForm<CreateFlagFormValues>({
    initialValues: {
      key: '',
      name: '',
      type: '',
      defaultRaw: '',
      description: '',
    },
    validate: {
      key: (value) => {
        if (value.trim().length === 0) {
          return i18n.t('flags.keyRequired');
        }
        if (value.trim().length < MIN_KEY_LENGTH || value.length > MAX_KEY_LENGTH) {
          return i18n.t('flags.keyLength', { min: MIN_KEY_LENGTH, max: MAX_KEY_LENGTH });
        }
        return null;
      },
      name: (value) => {
        if (value.trim().length === 0) {
          return i18n.t('flags.nameRequired');
        }
        if (value.length > MAX_NAME_LENGTH) {
          return i18n.t('flags.nameMaxLength', { count: MAX_NAME_LENGTH });
        }
        return null;
      },
      type: (value) => {
        if (value === '') {
          return i18n.t('flags.typeRequired');
        }
        return null;
      },
      defaultRaw: (value, values) => {
        if (values.type === '' || !isFlagType(values.type)) {
          return i18n.t('flags.typeRequired');
        }
        if (parseDefaultValueInput(values.type, value) === null) {
          if (values.type === 'number') {
            return i18n.t('flags.defaultInvalidNumber');
          }
          return i18n.t('flags.defaultRequired');
        }
        return null;
      },
      description: (value) => {
        if (value.length > MAX_DESCRIPTION_LENGTH) {
          return i18n.t('flags.descriptionMaxLength', { count: MAX_DESCRIPTION_LENGTH });
        }
        return null;
      },
    },
  });

  const selectedType = form.values.type;

  const handleSubmit = (values: CreateFlagFormValues): void => {
    if (isPending || !isFlagType(values.type)) {
      return;
    }
    const parsed = parseDefaultValueInput(values.type, values.defaultRaw);
    if (parsed === null) {
      return;
    }
    const description = values.description.trim();
    onSubmit({
      key: values.key.trim(),
      name: values.name.trim(),
      type: values.type,
      default_value: parsed,
      ...(description.length > 0 ? { description } : {}),
    });
  };

  const handleClose = (): void => {
    if (!isPending) {
      form.reset();
      onClose();
    }
  };

  return (
    <Modal opened={opened} onClose={handleClose} title={t('flags.createFlag')} centered>
      <form onSubmit={form.onSubmit(handleSubmit)} noValidate>
        <Stack gap="md">
          <TextInput
            label={t('flags.key')}
            placeholder={t('flags.keyPlaceholder')}
            withAsterisk
            disabled={isPending}
            {...form.getInputProps('key')}
          />
          <TextInput
            label={t('flags.name')}
            placeholder={t('flags.namePlaceholder')}
            withAsterisk
            disabled={isPending}
            {...form.getInputProps('name')}
          />
          <Select
            label={t('flags.type')}
            placeholder={t('flags.typePlaceholder')}
            withAsterisk
            disabled={isPending}
            data={FLAG_TYPES.map((flagType) => ({
              value: flagType,
              label: t(`flags.types.${flagType}`),
            }))}
            {...form.getInputProps('type')}
            onChange={(next) => {
              form.setFieldValue('type', (next ?? '') as FlagType | '');
              form.setFieldValue('defaultRaw', '');
            }}
          />
          {isFlagType(selectedType) ? (
            <FlagDefaultInput
              flagType={selectedType}
              value={form.values.defaultRaw}
              onChange={(next) => {
                form.setFieldValue('defaultRaw', next);
              }}
              onBlur={() => {
                form.validateField('defaultRaw');
              }}
              error={form.errors.defaultRaw}
              disabled={isPending}
            />
          ) : null}
          <Textarea
            label={t('flags.description')}
            placeholder={t('flags.descriptionPlaceholder')}
            disabled={isPending}
            autosize
            minRows={2}
            {...form.getInputProps('description')}
          />
          <Group justify="flex-end" gap="sm">
            <Button variant="subtle" onClick={handleClose} disabled={isPending}>
              {t('flags.cancel')}
            </Button>
            <Button type="submit" loading={isPending} disabled={isPending || !form.isValid()}>
              {t('flags.create')}
            </Button>
          </Group>
        </Stack>
      </form>
    </Modal>
  );
}
