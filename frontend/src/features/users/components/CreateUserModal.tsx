import { Button, Group, Modal, PasswordInput, Select, Stack, TextInput } from '@mantine/core';
import { useForm } from '@mantine/form';
import { useTranslation } from 'react-i18next';
import { USER_ROLES } from '../types';
import type { CreateUserRequest, UserRole } from '../types';

const EMAIL_REGEX = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;
const MIN_PASSWORD_LENGTH = 8;
const MAX_NAME_LENGTH = 256;

export interface CreateUserFormValues {
  full_name: string;
  email: string;
  password: string;
  role: UserRole | '';
}

interface CreateUserModalProps {
  opened: boolean;
  isPending: boolean;
  onClose: () => void;
  onSubmit: (request: CreateUserRequest) => void;
}

export function CreateUserModal({ opened, isPending, onClose, onSubmit }: CreateUserModalProps) {
  const { t, i18n } = useTranslation();

  const form = useForm<CreateUserFormValues>({
    initialValues: {
      full_name: '',
      email: '',
      password: '',
      role: '',
    },
    validate: {
      full_name: (value) => {
        if (value.trim().length === 0) {
          return i18n.t('users.fullNameRequired');
        }
        if (value.length > MAX_NAME_LENGTH) {
          return i18n.t('users.fullNameMaxLength', { count: MAX_NAME_LENGTH });
        }
        return null;
      },
      email: (value) => {
        if (value.trim().length === 0) {
          return i18n.t('users.emailRequired');
        }
        if (!EMAIL_REGEX.test(value.trim())) {
          return i18n.t('users.emailInvalid');
        }
        return null;
      },
      password: (value) => {
        if (value.length === 0) {
          return i18n.t('users.passwordRequired');
        }
        if (value.length < MIN_PASSWORD_LENGTH) {
          return i18n.t('users.passwordMinLength', { count: MIN_PASSWORD_LENGTH });
        }
        return null;
      },
      role: (value) => {
        if (value === '') {
          return i18n.t('users.roleRequired');
        }
        return null;
      },
    },
  });

  const handleSubmit = (values: CreateUserFormValues): void => {
    if (isPending || values.role === '') {
      return;
    }
    onSubmit({
      full_name: values.full_name.trim(),
      email: values.email.trim(),
      password: values.password,
      role: values.role,
    });
  };

  const handleClose = (): void => {
    if (!isPending) {
      form.reset();
      onClose();
    }
  };

  return (
    <Modal opened={opened} onClose={handleClose} title={t('users.createUser')} centered>
      <form onSubmit={form.onSubmit(handleSubmit)} noValidate>
        <Stack gap="md">
          <TextInput
            label={t('users.fullName')}
            placeholder={t('users.fullNamePlaceholder')}
            withAsterisk
            disabled={isPending}
            {...form.getInputProps('full_name')}
          />
          <TextInput
            label={t('users.email')}
            placeholder={t('users.emailPlaceholder')}
            type="email"
            autoComplete="email"
            withAsterisk
            disabled={isPending}
            {...form.getInputProps('email')}
          />
          <PasswordInput
            label={t('users.password')}
            placeholder={t('users.passwordPlaceholder')}
            autoComplete="new-password"
            withAsterisk
            disabled={isPending}
            {...form.getInputProps('password')}
          />
          <Select
            label={t('users.role')}
            placeholder={t('users.rolePlaceholder')}
            withAsterisk
            disabled={isPending}
            data={USER_ROLES.map((role) => ({
              value: role,
              label: t(`users.roles.${role}`),
            }))}
            {...form.getInputProps('role')}
          />
          <Group justify="flex-end" gap="sm">
            <Button variant="subtle" onClick={handleClose} disabled={isPending}>
              {t('users.cancel')}
            </Button>
            <Button type="submit" loading={isPending} disabled={isPending || !form.isValid()}>
              {t('users.create')}
            </Button>
          </Group>
        </Stack>
      </form>
    </Modal>
  );
}
