import {
  Button,
  Divider,
  Group,
  Modal,
  Stack,
  Text,
  Textarea,
  TextInput,
} from '@mantine/core';
import { useForm } from '@mantine/form';
import { useTranslation } from 'react-i18next';
import { FlagDefaultInput } from './FlagDefaultInput';
import { formatFlagDateTime } from '../lib/format';
import {
  buildFlagUpdatePayload,
  formatDefaultValue,
  parseDefaultValueInput,
} from '../lib/flagValue';
import type { Flag, UpdateFlagRequest } from '../types';

const MIN_KEY_LENGTH = 3;
const MAX_KEY_LENGTH = 128;
const MAX_NAME_LENGTH = 256;
const MAX_DESCRIPTION_LENGTH = 4096;

interface EditFlagFormValues {
  key: string;
  name: string;
  defaultRaw: string;
  description: string;
}

interface DetailsBodyProps {
  flag: Flag;
  canWrite: boolean;
  isSaving: boolean;
  onClose: () => void;
  onSave: (id: string, request: UpdateFlagRequest) => void;
}

function DetailsBody({ flag, canWrite, isSaving, onClose, onSave }: DetailsBodyProps) {
  const { t, i18n } = useTranslation();
  const language = i18n.resolvedLanguage ?? 'en';

  const form = useForm<EditFlagFormValues>({
    initialValues: {
      key: flag.key,
      name: flag.name,
      defaultRaw: formatDefaultValue(flag.default_value),
      description: flag.description ?? '',
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
      defaultRaw: (value) => {
        if (parseDefaultValueInput(flag.type, value) === null) {
          if (flag.type === 'number') {
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

  const payload = buildFlagUpdatePayload(flag, {
    key: form.values.key,
    name: form.values.name,
    defaultRaw: form.values.defaultRaw,
    description: form.values.description,
  });

  const handleSubmit = (values: EditFlagFormValues): void => {
    if (isSaving || !canWrite) {
      return;
    }
    const next = buildFlagUpdatePayload(flag, values);
    if (next !== null) {
      onSave(flag.id, next);
    }
  };

  return (
    <Stack gap="md">
      {!canWrite ? (
        <Text size="sm" c="dimmed">
          {t('flags.readOnlyHint')}
        </Text>
      ) : null}
      <form onSubmit={form.onSubmit(handleSubmit)} noValidate>
        <Stack gap="md">
          <TextInput
            label={t('flags.type')}
            value={t(`flags.types.${flag.type}`)}
            description={t('flags.typeImmutableHint')}
            readOnly
          />
          <TextInput
            label={t('flags.key')}
            disabled={isSaving || !canWrite}
            {...form.getInputProps('key')}
          />
          <TextInput
            label={t('flags.name')}
            disabled={isSaving || !canWrite}
            {...form.getInputProps('name')}
          />
          <FlagDefaultInput
            flagType={flag.type}
            value={form.values.defaultRaw}
            onChange={(next) => {
              form.setFieldValue('defaultRaw', next);
            }}
            onBlur={() => {
              form.validateField('defaultRaw');
            }}
            error={form.errors.defaultRaw}
            disabled={isSaving || !canWrite}
          />
          <Textarea
            label={t('flags.description')}
            placeholder={t('flags.descriptionPlaceholder')}
            description={canWrite ? t('flags.descriptionClearHint') : undefined}
            disabled={isSaving || !canWrite}
            autosize
            minRows={2}
            {...form.getInputProps('description')}
          />
          {canWrite ? (
            <Group justify="flex-end" gap="sm">
              <Button variant="subtle" onClick={onClose} disabled={isSaving}>
                {t('flags.cancel')}
              </Button>
              <Button
                type="submit"
                loading={isSaving}
                disabled={isSaving || payload === null || !form.isValid()}
              >
                {t('flags.save')}
              </Button>
            </Group>
          ) : null}
        </Stack>
      </form>

      <Divider />
      <Stack gap={2}>
        <Text size="xs" c="dimmed">
          {t('flags.flagId', { id: flag.id })}
        </Text>
        {flag.created_by !== null ? (
          <Text size="xs" c="dimmed">
            {t('flags.createdByValue', { name: flag.created_by.full_name })}
          </Text>
        ) : null}
        {flag.updated_by !== null ? (
          <Text size="xs" c="dimmed">
            {t('flags.updatedByValue', { name: flag.updated_by.full_name })}
          </Text>
        ) : null}
        <Text size="xs" c="dimmed">
          {t('flags.createdAtValue', { value: formatFlagDateTime(flag.created_at, language) })}
        </Text>
        <Text size="xs" c="dimmed">
          {t('flags.updatedAtValue', { value: formatFlagDateTime(flag.updated_at, language) })}
        </Text>
      </Stack>
    </Stack>
  );
}

interface FlagDetailsModalProps {
  flag: Flag | null;
  canWrite: boolean;
  opened: boolean;
  isSaving: boolean;
  onClose: () => void;
  onSave: (id: string, request: UpdateFlagRequest) => void;
}

export function FlagDetailsModal({
  flag,
  canWrite,
  opened,
  isSaving,
  onClose,
  onSave,
}: FlagDetailsModalProps) {
  const { t } = useTranslation();

  const handleClose = (): void => {
    if (!isSaving) {
      onClose();
    }
  };

  return (
    <Modal opened={opened} onClose={handleClose} title={t('flags.flagDetails')} centered>
      {flag === null ? null : (
        <DetailsBody
          key={flag.id}
          flag={flag}
          canWrite={canWrite}
          isSaving={isSaving}
          onClose={onClose}
          onSave={onSave}
        />
      )}
    </Modal>
  );
}
