import { Divider, Stack, Text } from '@mantine/core';
import { useTranslation } from 'react-i18next';
import packageJson from '../../../package.json';

export function SidebarFooter() {
  const { t } = useTranslation();

  return (
    <Stack gap={4} aria-label={t('layout.footer')}>
      <Divider />
      <Text size="xs" c="dimmed" px="sm" pt={4}>
        {t('layout.build')} · {t('layout.frontend')} v{packageJson.version}
      </Text>
    </Stack>
  );
}
