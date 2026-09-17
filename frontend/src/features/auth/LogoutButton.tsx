import { ActionIcon } from '@mantine/core';
import { notifications } from '@mantine/notifications';
import { IconLogout } from '@tabler/icons-react';
import { useTranslation } from 'react-i18next';
import { useNavigate } from 'react-router';
import { clearAuthSession } from './lib/authSession';
import { queryClient } from '@/shared/lib/queryClient';

export function LogoutButton() {
  const { t } = useTranslation();
  const navigate = useNavigate();

  const handleLogout = (): void => {
    clearAuthSession();
    queryClient.clear();
    notifications.show({
      id: 'signed-out',
      title: t('auth.signedOutTitle'),
      message: t('auth.signedOutMessage'),
      color: 'green',
    });
    void navigate('/login', { replace: true });
  };

  return (
    <ActionIcon variant="subtle" size={36} onClick={handleLogout} aria-label="Logout">
      <IconLogout size={16} />
    </ActionIcon>
  );
}
