import {
  Avatar,
  Button,
  Divider,
  FileInput,
  Group,
  Modal,
  Select,
  Stack,
  Text,
  TextInput,
} from '@mantine/core';
import { useForm } from '@mantine/form';
import { IconTrash, IconUpload } from '@tabler/icons-react';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { USER_ROLES } from '../types';
import type { UpdateUserRequest, User, UserRole } from '../types';
import { formatUserDateTime } from '../lib/format';
import { buildUpdatePayload, isSelfUser } from '../lib/rules';

const EMAIL_REGEX = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;

interface EditUserFormValues {
  email: string;
  role: UserRole;
}

interface DetailsBodyProps {
  user: User;
  me: User | null;
  isSaving: boolean;
  isAvatarPending: boolean;
  onClose: () => void;
  onSave: (id: string, request: UpdateUserRequest) => void;
  onUploadAvatar: (id: string, file: File | null) => void;
}

function DetailsBody({
  user,
  me,
  isSaving,
  isAvatarPending,
  onClose,
  onSave,
  onUploadAvatar,
}: DetailsBodyProps) {
  const { t, i18n } = useTranslation();
  const language = i18n.resolvedLanguage ?? 'en';
  const [avatarFile, setAvatarFile] = useState<File | null>(null);

  const form = useForm<EditUserFormValues>({
    initialValues: {
      email: user.email,
      role: user.role,
    },
    validate: {
      email: (value) => {
        if (value.trim().length === 0) {
          return i18n.t('users.emailRequired');
        }
        if (!EMAIL_REGEX.test(value.trim())) {
          return i18n.t('users.emailInvalid');
        }
        return null;
      },
    },
  });

  const isSelf = isSelfUser(me, user);
  const canManageAvatar = me?.role === 'admin';
  const pending = isSaving || isAvatarPending;
  const payload = buildUpdatePayload(user, {
    email: form.values.email,
    role: form.values.role,
  });

  const handleSubmit = (values: EditUserFormValues): void => {
    if (isSaving) {
      return;
    }
    const next = buildUpdatePayload(user, values);
    if (next !== null) {
      onSave(user.id, next);
    }
  };

  const handleUpload = (): void => {
    if (avatarFile !== null && !isAvatarPending) {
      onUploadAvatar(user.id, avatarFile);
      setAvatarFile(null);
    }
  };

  const handleRemoveAvatar = (): void => {
    if (!isAvatarPending) {
      onUploadAvatar(user.id, null);
    }
  };

  return (
    <Stack gap="md">
      <form onSubmit={form.onSubmit(handleSubmit)} noValidate>
        <Stack gap="md">
          <TextInput label={t('users.fullName')} value={user.full_name} readOnly />
          <TextInput
            label={t('users.email')}
            type="email"
            autoComplete="email"
            disabled={isSaving}
            {...form.getInputProps('email')}
          />
          <Select
            label={t('users.role')}
            disabled={isSaving || isSelf}
            description={isSelf ? t('users.selfRoleHint') : undefined}
            data={USER_ROLES.map((role) => ({
              value: role,
              label: t(`users.roles.${role}`),
            }))}
            {...form.getInputProps('role')}
          />
          <Group justify="flex-end" gap="sm">
            <Button variant="subtle" onClick={onClose} disabled={pending}>
              {t('users.cancel')}
            </Button>
            <Button
              type="submit"
              loading={isSaving}
              disabled={isSaving || payload === null || !form.isValid()}
            >
              {t('users.save')}
            </Button>
          </Group>
        </Stack>
      </form>

      {canManageAvatar ? (
        <>
          <Divider />
          <Stack gap="sm">
            <Text size="sm" fw={600}>
              {t('users.avatar')}
            </Text>
            <Group gap="md" wrap="nowrap" align="flex-start">
              <Avatar
                src={user.avatar_url}
                name={user.full_name}
                alt={user.full_name}
                size="lg"
                radius="sm"
              />
              <Stack gap="xs" flex={1}>
                <FileInput
                  value={avatarFile}
                  onChange={setAvatarFile}
                  accept="image/jpeg,image/png,image/webp"
                  placeholder={t('users.avatarPlaceholder')}
                  aria-label={t('users.uploadAvatar')}
                  disabled={isAvatarPending}
                  clearable
                />
                <Group gap="xs">
                  <Button
                    size="xs"
                    variant="default"
                    leftSection={<IconUpload size={14} />}
                    disabled={avatarFile === null || isAvatarPending}
                    loading={isAvatarPending && avatarFile !== null}
                    onClick={handleUpload}
                  >
                    {t('users.uploadAvatar')}
                  </Button>
                  {user.avatar_url !== null ? (
                    <Button
                      size="xs"
                      variant="subtle"
                      color="red"
                      leftSection={<IconTrash size={14} />}
                      disabled={isAvatarPending}
                      loading={isAvatarPending && avatarFile === null}
                      onClick={handleRemoveAvatar}
                    >
                      {t('users.removeAvatar')}
                    </Button>
                  ) : null}
                </Group>
              </Stack>
            </Group>
          </Stack>
        </>
      ) : null}

      <Divider />
      <Stack gap={2}>
        <Text size="xs" c="dimmed">
          {t('users.userId', { id: user.id })}
        </Text>
        <Text size="xs" c="dimmed">
          {t('users.createdAtValue', { value: formatUserDateTime(user.created_at, language) })}
        </Text>
        <Text size="xs" c="dimmed">
          {t('users.updatedAtValue', { value: formatUserDateTime(user.updated_at, language) })}
        </Text>
      </Stack>
    </Stack>
  );
}

interface UserDetailsModalProps {
  user: User | null;
  me: User | null;
  opened: boolean;
  isSaving: boolean;
  isAvatarPending: boolean;
  onClose: () => void;
  onSave: (id: string, request: UpdateUserRequest) => void;
  onUploadAvatar: (id: string, file: File | null) => void;
}

export function UserDetailsModal({
  user,
  me,
  opened,
  isSaving,
  isAvatarPending,
  onClose,
  onSave,
  onUploadAvatar,
}: UserDetailsModalProps) {
  const { t } = useTranslation();

  const handleClose = (): void => {
    if (!isSaving && !isAvatarPending) {
      onClose();
    }
  };

  return (
    <Modal opened={opened} onClose={handleClose} title={t('users.userDetails')} centered>
      {user === null ? null : (
        <DetailsBody
          key={user.id}
          user={user}
          me={me}
          isSaving={isSaving}
          isAvatarPending={isAvatarPending}
          onClose={onClose}
          onSave={onSave}
          onUploadAvatar={onUploadAvatar}
        />
      )}
    </Modal>
  );
}
