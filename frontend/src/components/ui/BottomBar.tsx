import { Group, Paper, Select } from '@mantine/core';
import { useMantineColorScheme } from '@mantine/core';
import type { MantineColorScheme } from '@mantine/core';
import { useTranslation } from 'react-i18next';
import type { Locale, ThemePreference } from '@/shared/types/common';

function isThemePreference(value: string): value is ThemePreference {
  return value === 'system' || value === 'light' || value === 'dark';
}

function isLocale(value: string): value is Locale {
  return value === 'ru' || value === 'en';
}

function toMantineScheme(preference: ThemePreference): MantineColorScheme {
  return preference === 'system' ? 'auto' : preference;
}

function fromMantineScheme(scheme: MantineColorScheme): ThemePreference {
  return scheme === 'auto' ? 'system' : scheme;
}

export function BottomBar() {
  const { t, i18n } = useTranslation();
  const { colorScheme, setColorScheme } = useMantineColorScheme();

  const handleThemeChange = (value: string | null) => {
    if (value !== null && isThemePreference(value)) {
      setColorScheme(toMantineScheme(value));
    }
  };

  const handleLocaleChange = (value: string | null) => {
    if (value !== null && isLocale(value)) {
      void i18n.changeLanguage(value);
    }
  };

  const blurActiveControl = () => {
    if (document.activeElement instanceof HTMLElement) {
      document.activeElement.blur();
    }
  };

  return (
    <Paper
      withBorder
      radius="md"
      px="md"
      py="xs"
      className="panel-illuminate"
      style={{
        position: 'fixed',
        bottom: 16,
        left: '50%',
        transform: 'translateX(-50%)',
        zIndex: 100,
      }}
    >
      <Group gap="md" justify="center" grow={false}>
        <Group gap="xs">
          <span>{t('theme.label')}</span>
          <Select
            size="xs"
            w={120}
            aria-label={t('theme.label')}
            value={fromMantineScheme(colorScheme)}
            onChange={handleThemeChange}
            onDropdownClose={blurActiveControl}
            data={[
              { label: t('theme.system'), value: 'system' },
              { label: t('theme.light'), value: 'light' },
              { label: t('theme.dark'), value: 'dark' },
            ]}
          />
        </Group>
        <Group gap="xs">
          <span>{t('language.label')}</span>
          <Select
            size="xs"
            w={90}
            aria-label={t('language.label')}
            value={i18n.resolvedLanguage === 'ru' ? 'ru' : 'en'}
            onChange={handleLocaleChange}
            onDropdownClose={blurActiveControl}
            data={[
              { label: 'RU', value: 'ru' },
              { label: 'EN', value: 'en' },
            ]}
          />
        </Group>
      </Group>
    </Paper>
  );
}
