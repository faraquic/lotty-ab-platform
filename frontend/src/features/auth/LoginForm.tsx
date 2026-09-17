import { Button, PasswordInput, Stack, TextInput } from '@mantine/core';
import { useForm } from '@mantine/form';
import { IconLogin2 } from '@tabler/icons-react';
import { useTranslation } from 'react-i18next';
import { getDevLoginDefaults } from './lib/devLoginDefaults';

const MIN_PASSWORD_LENGTH = 8;
const EMAIL_REGEX = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;

export interface LoginFormValues {
  email: string;
  password: string;
}

interface LoginFormProps {
  onSubmit: (values: LoginFormValues) => void;
  isPending: boolean;
}

export function LoginForm({ onSubmit, isPending }: LoginFormProps) {
  const { t, i18n } = useTranslation();

  const form = useForm<LoginFormValues>({
    initialValues: getDevLoginDefaults() ?? {
      email: '',
      password: '',
    },
    validate: {
      email: (value) => {
        if (value.trim().length === 0) {
          return i18n.t('login.emailRequired');
        }
        if (!EMAIL_REGEX.test(value)) {
          return i18n.t('login.emailInvalid');
        }
        return null;
      },
      password: (value) => {
        if (value.length === 0) {
          return i18n.t('login.passwordRequired');
        }
        if (value.length < MIN_PASSWORD_LENGTH) {
          return i18n.t('login.passwordMinLength', { count: MIN_PASSWORD_LENGTH });
        }
        return null;
      },
    },
  });

  const handleSubmit = (values: LoginFormValues): void => {
    if (!isPending) {
      onSubmit(values);
    }
  };

  return (
    <form onSubmit={form.onSubmit(handleSubmit)} noValidate>
      <Stack gap={22}>
        <TextInput
          label={t('login.emailLabel')}
          placeholder={t('login.emailPlaceholder')}
          type="email"
          autoComplete="email"
          withAsterisk
          size="sm"
          disabled={isPending}
          {...form.getInputProps('email')}
        />
        <PasswordInput
          label={t('login.passwordLabel')}
          placeholder={t('login.passwordPlaceholder')}
          autoComplete="current-password"
          withAsterisk
          size="sm"
          disabled={isPending}
          {...form.getInputProps('password')}
        />
        <Button
          type="submit"
          fullWidth
          className="btn-glow-green"
          leftSection={<IconLogin2 size={16} />}
          loading={isPending}
          disabled={isPending || !form.isValid()}
          fw={500}
        >
          {t('login.submit')}
        </Button>
      </Stack>
    </form>
  );
}
