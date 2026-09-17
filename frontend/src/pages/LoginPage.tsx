import { Center, Container, Stack, Text, Title } from '@mantine/core';
import { notifications } from '@mantine/notifications';
import { useEffect } from 'react';
import { useTranslation } from 'react-i18next';
import { useNavigate } from 'react-router';
import { BottomBar } from '@/components/ui/BottomBar';
import { LoginForm } from '@/features/auth/LoginForm';
import type { LoginFormValues } from '@/features/auth/LoginForm';
import { useLoginMutation } from '@/features/auth/api/useLoginMutation';
import { setAuthSession, useAuthSession } from '@/features/auth/lib/authSession';
import { resolveLoginErrorMessage } from '@/features/auth/lib/loginErrorMessage';
import { queryClient } from '@/shared/lib/queryClient';

export function LoginPage() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const login = useLoginMutation();
  const session = useAuthSession();

  useEffect(() => {
    if (session !== null) {
      void navigate('/', { replace: true });
    }
  }, [session, navigate]);

  const handleSubmit = (values: LoginFormValues): void => {
    login.mutate(values, {
      onSuccess: (data) => {
        queryClient.clear();
        setAuthSession(data.token, data.expires_at);
        notifications.show({
          id: 'login-success',
          title: t('login.successTitle'),
          message: t('login.successMessage'),
          color: 'green',
        });
        void navigate('/', { replace: true });
      },
      onError: (error) => {
        notifications.show({
          id: 'login-error',
          title: t('login.errorTitle'),
          message: resolveLoginErrorMessage(error, t),
          color: 'red',
        });
      },
    });
  };

  return (
    <>
      <Center mih="100dvh" p="md" className="login-bg">
        <Container size={400} w="100%" pb={80}>
          <Stack gap="xl">
            <Title
              order={1}
              ta="center"
              size="2.5rem"
              fw={600}
              style={{ letterSpacing: '-0.02em' }}
            >
              Lotty{' '}
              <Text span inherit className="accent-word">
                A/B
              </Text>{' '}
              Platform
            </Title>
            <section aria-label={t('login.submit')}>
              <LoginForm onSubmit={handleSubmit} isPending={login.isPending} />
            </section>
          </Stack>
        </Container>
      </Center>
      <BottomBar />
    </>
  );
}
