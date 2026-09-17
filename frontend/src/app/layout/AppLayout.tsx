import {
  AppShell,
  Burger,
  Button,
  Container,
  Group,
  NavLink,
  Stack,
  Text,
  Title,
} from '@mantine/core';
import { useDisclosure } from '@mantine/hooks';
import { IconFlag, IconHome, IconUsers } from '@tabler/icons-react';
import { useTranslation } from 'react-i18next';
import { Link, Outlet, useLocation } from 'react-router';
import { LogoutButton } from '@/features/auth/LogoutButton';
import { OverallStatusPill } from '@/features/status/components/OverallStatusPill';
import { CurrentUserChip } from '@/features/users/components/CurrentUserChip';
import { SidebarFooter } from './SidebarFooter';

export function AppLayout() {
  const { t } = useTranslation();
  const location = useLocation();
  const [navOpened, { toggle: toggleNav, close: closeNav }] = useDisclosure(false);

  return (
    <AppShell
      header={{ height: 56 }}
      navbar={{ width: 250, breakpoint: 'sm', collapsed: { mobile: !navOpened } }}
      padding="md"
    >
      <AppShell.Header px="md" className="app-header">
        <Group h="100%" gap="sm" justify="space-between" wrap="nowrap">
          <Group gap="sm" wrap="nowrap">
            <Burger
              opened={navOpened}
              onClick={toggleNav}
              hiddenFrom="sm"
              size="sm"
              aria-label={t('layout.menu')}
            />
            <Title order={1} size="1.3rem" fw={600} style={{ letterSpacing: '-0.02em' }}>
              Lotty{' '}
              <Text span inherit className="accent-word">
                A/B
              </Text>{' '}
              Platform
            </Title>
          </Group>
          <Group gap="sm" wrap="nowrap">
            <Button
              className="status-nav-button"
              variant="subtle"
              size="sm"
              leftSection={<OverallStatusPill />}
              component={Link}
              to="/status"
            >
              {t('status.openStatus')}
            </Button>
            <CurrentUserChip />
            <LogoutButton />
          </Group>
        </Group>
      </AppShell.Header>
      <AppShell.Navbar p="xs">
        <Stack gap={4} h="100%" justify="space-between">
          <Stack gap={4}>
            <Text size="xs" c="dimmed" px="sm" pt="xs" tt="uppercase" fw={600}>
              {t('layout.navigation')}
            </Text>
          <NavLink
            label={t('layout.home')}
            leftSection={<IconHome size={16} />}
            component={Link}
            to="/"
            active={location.pathname === '/'}
            onClick={closeNav}
            aria-label={t('layout.home')}
            className="app-nav-link"
          />
          <NavLink
            label={t('users.title')}
            leftSection={<IconUsers size={16} />}
            component={Link}
            to="/users"
            active={location.pathname === '/users' || location.pathname.startsWith('/users/')}
            onClick={closeNav}
            aria-label={t('users.title')}
            className="app-nav-link"
          />
          <NavLink
            label={t('flags.title')}
            leftSection={<IconFlag size={16} />}
            component={Link}
            to="/flags"
            active={location.pathname === '/flags' || location.pathname.startsWith('/flags/')}
            onClick={closeNav}
            aria-label={t('flags.title')}
            className="app-nav-link"
          />
          </Stack>
          <SidebarFooter />
        </Stack>
      </AppShell.Navbar>
      <AppShell.Main className="app-bg">
        <Outlet />
      </AppShell.Main>
    </AppShell>
  );
}

export function RootIndexStub() {
  const { t } = useTranslation();

  return (
    <Container size={560} py="xl">
      <Text c="dimmed" ta="center">
        {t('layout.indexStub')}
      </Text>
    </Container>
  );
}
