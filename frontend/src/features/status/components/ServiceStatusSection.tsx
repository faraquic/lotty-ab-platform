import { Accordion, Badge, Group, Stack, Text } from '@mantine/core';
import { IconAlertTriangle, IconCheck, IconCircleFilled, IconX } from '@tabler/icons-react';
import type { ComponentType } from 'react';
import { useTranslation } from 'react-i18next';
import { formatLatency, formatTime } from '../lib/format';
import type { ServiceName, ServiceProbe, ServiceStatus } from '../types';

const SERVICE_NAMES: Record<ServiceName, 'status.services.panel' | 'status.services.runtime' | 'status.services.analytics'> = {
  panel: 'status.services.panel',
  runtime: 'status.services.runtime',
  analytics: 'status.services.analytics',
};

const SERVICE_LABELS: Record<ServiceStatus, 'status.healthy' | 'status.degraded' | 'status.down'> = {
  healthy: 'status.healthy',
  degraded: 'status.degraded',
  down: 'status.down',
};

const COMPONENT_LABELS = {
  healthy: 'status.healthy',
  warning: 'status.warning',
  unhealthy: 'status.unhealthy',
} as const;

type IconProps = {
  size?: number | string;
};

function serviceIndicator(status: ServiceStatus): { color: string; Icon: ComponentType<IconProps> } {
  switch (status) {
    case 'healthy':
      return { color: 'green', Icon: IconCheck };
    case 'degraded':
      return { color: 'yellow', Icon: IconAlertTriangle };
    case 'down':
      return { color: 'red', Icon: IconCircleFilled };
  }
}

function componentIndicator(status: string): {
  color: string;
  Icon: ComponentType<IconProps>;
  label: 'healthy' | 'warning' | 'unhealthy';
} {
  if (status === 'ok') {
    return { color: 'green', Icon: IconCheck, label: 'healthy' };
  }
  if (status === 'unavailable') {
    return { color: 'red', Icon: IconX, label: 'unhealthy' };
  }
  return { color: 'yellow', Icon: IconAlertTriangle, label: 'warning' };
}

interface ServiceStatusSectionProps {
  probe: ServiceProbe;
}

export function ServiceStatusSection({ probe }: ServiceStatusSectionProps) {
  const { t, i18n } = useTranslation();
  const language = i18n.resolvedLanguage ?? 'en';
  const indicator = serviceIndicator(probe.status);
  const RowIcon = indicator.Icon;

  const componentEntries = probe.ready !== null ? Object.entries(probe.ready.components) : [];

  return (
    <Accordion.Item value={probe.service}>
      <Accordion.Control py="md">
        <Group justify="space-between" pr="md" wrap="nowrap">
          <Group gap="sm" wrap="nowrap">
            <Text c={indicator.color} span lh={1}>
              <RowIcon size={22} />
            </Text>
            <Stack gap={0}>
              <Text fw={600} size="md">
                {t(SERVICE_NAMES[probe.service])}
              </Text>
              {probe.ready !== null && (
                <Text size="xs" c="dimmed">
                  {probe.ready.service} · {probe.ready.version} · {probe.ready.environment}
                </Text>
              )}
            </Stack>
          </Group>
          <Group gap="sm" wrap="nowrap">
            <Text size="sm" c="dimmed">
              {formatLatency(probe.latencyMs, t)}
            </Text>
            <Badge color={indicator.color}>{t(SERVICE_LABELS[probe.status])}</Badge>
          </Group>
        </Group>
      </Accordion.Control>
      <Accordion.Panel>
        {probe.ready !== null ? (
          <Stack gap="md" pb="xs">
            <Group gap="xl">
              <Stack gap={2}>
                <Text size="xs" c="dimmed">
                  {t('status.service')}
                </Text>
                <Text size="sm">{probe.ready.service}</Text>
              </Stack>
              <Stack gap={2}>
                <Text size="xs" c="dimmed">
                  {t('status.version')}
                </Text>
                <Text size="sm">{probe.ready.version}</Text>
              </Stack>
              <Stack gap={2}>
                <Text size="xs" c="dimmed">
                  {t('status.environment')}
                </Text>
                <Text size="sm">{probe.ready.environment}</Text>
              </Stack>
            </Group>
            <Text size="sm" fw={600}>
              {t('status.components')}
            </Text>
            {componentEntries.map(([name, component]) => {
              const badge = componentIndicator(component.status);
              const ComponentIcon = badge.Icon;
              const isOk = component.status === 'ok';
              return (
                <Stack key={name} gap={0}>
                  <Group justify="space-between" wrap="nowrap">
                    <Text size="sm" tt="capitalize">
                      {name}
                    </Text>
                    <Badge
                      color={badge.color}
                      size="sm"
                      variant="light"
                      leftSection={<ComponentIcon size={12} />}
                    >
                      {t(COMPONENT_LABELS[badge.label])}
                    </Badge>
                  </Group>
                  {!isOk && component.message !== null && component.message.length > 0 && (
                    <Text size="xs" c="dimmed">
                      {component.message}
                    </Text>
                  )}
                </Stack>
              );
            })}
            <Group justify="space-between" pt="xs">
              <Text size="xs" c="dimmed">
                {t('status.lastChecked')}: {formatTime(probe.checkedAt, language)}
              </Text>
              <Text size="xs" c="dimmed">
                {t('status.responseTime')}: {formatLatency(probe.latencyMs, t)}
              </Text>
            </Group>
          </Stack>
        ) : (
          <FailureMessage probe={probe} />
        )}
      </Accordion.Panel>
    </Accordion.Item>
  );
}

function FailureMessage({ probe }: ServiceStatusSectionProps) {
  const { t } = useTranslation();
  const failure = probe.failure;
  if (failure === null) {
    return (
      <Text size="sm" c="dimmed">
        {t('status.serviceUnavailable')}
      </Text>
    );
  }
  switch (failure.reason) {
    case 'timeout':
      return (
        <Text size="sm" c="dimmed">
          {t('status.timeoutAfter', { seconds: 5 })}
        </Text>
      );
    case 'network':
      return (
        <Text size="sm" c="dimmed">
          {t('status.unreachable')}
        </Text>
      );
    case 'invalid':
      return (
        <Text size="sm" c="dimmed">
          {t('status.invalidResponse')}
        </Text>
      );
    case 'http':
      return (
        <Text size="sm" c="dimmed">
          {failure.httpStatus === null
            ? t('status.serviceUnavailable')
            : t('status.httpError', { status: failure.httpStatus })}
        </Text>
      );
  }
}
