import { notifications } from '@mantine/notifications';
import { useEffect } from 'react';
import type { ReactNode } from 'react';
import { useTranslation } from 'react-i18next';
import { useLocation, useNavigate } from 'react-router';
import { useAuthSession } from './lib/authSession';

interface RequireAuthProps {
  children: ReactNode;
}

export function RequireAuth({ children }: RequireAuthProps) {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const location = useLocation();
  const session = useAuthSession();

  useEffect(() => {
    if (session === null) {
      notifications.show({
        id: 'auth-required',
        title: t('login.authRequiredTitle'),
        message: t('login.authRequiredMessage'),
        color: 'yellow',
      });
      void navigate('/login', { replace: true, state: { from: location.pathname } });
    }
  }, [session, navigate, location.pathname, t]);

  if (session === null) {
    return null;
  }
  return <>{children}</>;
}
