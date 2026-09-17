import { Box } from '@mantine/core';
import type { MantineColor } from '@mantine/core';
import type { ReactNode } from 'react';

interface MetricIconProps {
  children: ReactNode;
  color: MantineColor;
  radius?: 'xs' | 'sm' | 'md' | 'lg' | 'xl';
  size?: number;
}

export function MetricIcon({ children, color, radius = 'md', size = 36 }: MetricIconProps) {
  return (
    <Box
      w={size}
      h={size}
      style={{
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        flexShrink: 0,
        borderRadius: `var(--mantine-radius-${radius})`,
        color: `var(--mantine-color-${color}-filled)`,
        backgroundColor: `color-mix(in srgb, var(--mantine-color-${color}-filled) 12%, transparent)`,
      }}
    >
      {children}
    </Box>
  );
}
