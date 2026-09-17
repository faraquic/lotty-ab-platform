import {
  DEFAULT_THEME,
  MantineProvider,
  createTheme,
  localStorageColorSchemeManager,
} from '@mantine/core';
import { ModalsProvider } from '@mantine/modals';
import { Notifications } from '@mantine/notifications';
import { QueryClientProvider } from '@tanstack/react-query';
import type { ReactNode } from 'react';
import { queryClient } from '@/shared/lib/queryClient';
import '@fontsource/inter/400.css';
import '@fontsource/inter/500.css';
import '@fontsource/inter/600.css';
import '@fontsource/inter/700.css';
import '@mantine/core/styles.css';
import '@mantine/notifications/styles.css';

export const COLOR_SCHEME_STORAGE_KEY = 'labp-color-scheme';

const colorSchemeManager = localStorageColorSchemeManager({
  key: COLOR_SCHEME_STORAGE_KEY,
});

const FONT_STACK =
  'Inter, -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Oxygen, Ubuntu, sans-serif';

const inputFieldStyles = {
  label: {
    fontWeight: 600,
    marginBottom: 4,
  },
  input: {
    '--input-bg': 'var(--mantine-color-default)',
    '--input-bd': 'var(--mantine-color-default-border)',
    '--input-bd-focus': 'var(--input-focus)',
    '&:hover': {
      '--input-bd': 'var(--input-bd-hover)',
    },
  },
} as const;

const theme = createTheme({
  primaryColor: 'green',
  colors: {
    // Blue is banned from the UI: remap the stock scale to green so that
    // no component default can render a blue pixel.
    blue: DEFAULT_THEME.colors.green,
  },
  components: {
    TextInput: {
      styles: inputFieldStyles,
    },
    PasswordInput: {
      styles: inputFieldStyles,
    },
  },
  fontFamily: FONT_STACK,
  headings: {
    fontFamily: FONT_STACK,
  },
  defaultRadius: 'md',
});

interface AppProvidersProps {
  children: ReactNode;
}

export function AppProviders({ children }: AppProvidersProps) {
  return (
    <QueryClientProvider client={queryClient}>
      <MantineProvider
        theme={theme}
        colorSchemeManager={colorSchemeManager}
        defaultColorScheme="auto"
      >
        <Notifications position="bottom-right" limit={5} />
        <ModalsProvider>{children}</ModalsProvider>
      </MantineProvider>
    </QueryClientProvider>
  );
}
