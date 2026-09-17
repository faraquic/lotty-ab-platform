import {
  Accordion,
  Box,
  Button,
  Container,
  Group,
  Skeleton,
  Stack,
  Text,
  Title,
} from '@mantine/core';
import { IconAlertTriangle, IconArrowLeft, IconCheck, IconCircleFilled, IconRefresh } from '@tabler/icons-react';
import type { ComponentType } from 'react';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Link } from 'react-router';
import { BottomBar } from '@/components/ui/BottomBar';
import { SegmentedSpinner } from '@/components/ui/SegmentedSpinner';
import { ServiceStatusSection } from '@/features/status/components/ServiceStatusSection';
import { useServiceStatus } from '@/features/status/api/useServiceStatus';
import { aggregateOverallStatus } from '@/features/status/lib/aggregate';
import { formatTime } from '@/features/status/lib/format';
import { ANALYTICS_API_BASE_URL, PANEL_API_BASE_URL, RUNTIME_API_BASE_URL } from '@/shared/config/api';
import type { OverallStatus, ServiceProbe } from '@/features/status/types';

type IconProps = {
  size?: number | string;
};

function overallDisplay(overall: OverallStatus): {
  color: string;
  Icon: ComponentType<IconProps>;
  titleKey: 'status.operational' | 'status.partial' | 'status.outage';
} {
  switch (overall) {
    case 'operational':
      return { color: 'green', Icon: IconCheck, titleKey: 'status.operational' };
    case 'partial':
      return { color: 'yellow', Icon: IconAlertTriangle, titleKey: 'status.partial' };
    case 'outage':
      return { color: 'red', Icon: IconCircleFilled, titleKey: 'status.outage' };
  }
}

export function StatusPage() {
  const { t, i18n } = useTranslation();
  const language = i18n.resolvedLanguage ?? 'en';
  const panel = useServiceStatus('panel', PANEL_API_BASE_URL);
  const runtime = useServiceStatus('runtime', RUNTIME_API_BASE_URL);
  const analytics = useServiceStatus('analytics', ANALYTICS_API_BASE_URL);
  const [expanded, setExpanded] = useState<string[]>([]);

  const probes: ServiceProbe[] = [panel.data, runtime.data, analytics.data].filter(
    (probe): probe is ServiceProbe => probe !== undefined,
  );
  const isInitialLoading = probes.length === 0;
  const isRefreshing = panel.isFetching || runtime.isFetching || analytics.isFetching;

  const overall = aggregateOverallStatus(probes.map((probe) => probe.status));
  const display = overallDisplay(overall);
  const StatusIcon = display.Icon;

  const lastChecked =
    probes.length > 0 ? Math.max(...probes.map((probe) => probe.checkedAt)) : null;

  const handleRefresh = (): void => {
    void panel.refetch();
    void runtime.refetch();
    void analytics.refetch();
  };

  return (
    <>
      <Box className="login-bg" mih="100dvh" py="xl">
        <Container size="md" pb={100}>
          <Stack gap="lg">
            {isInitialLoading ? (
              <>
                <Group gap="md" wrap="nowrap">
                  <Skeleton height={52} circle />
                  <Stack gap="xs" flex={1}>
                    <Skeleton height={14} width="30%" radius="sm" />
                    <Skeleton height={28} width="60%" radius="sm" />
                  </Stack>
                </Group>
                <Skeleton height={76} radius="md" />
                <Skeleton height={76} radius="md" />
                <Skeleton height={76} radius="md" />
              </>
            ) : (
              <>
                <Group justify="flex-start">
                  <Button
                    variant="subtle"
                    size="sm"
                    leftSection={<IconArrowLeft size={16} />}
                    component={Link}
                    to="/"
                  >
                    {t('status.back')}
                  </Button>
                </Group>
                <Group justify="space-between" align="flex-start">
                  <Group gap="md" wrap="nowrap">
                    <Box className="status-indicator" c={display.color} aria-hidden="true">
                      <SegmentedSpinner size={56} />
                      <StatusIcon size={22} />
                    </Box>
                    <Stack gap={2}>
                      <Text size="sm" c="dimmed">
                        {t('status.title')}
                      </Text>
                      <Title order={1} size="1.65rem" fw={600}>
                        {t(display.titleKey)}
                      </Title>
                    </Stack>
                  </Group>
                  <Stack gap={2} align="flex-end">
                    <Text size="xs" c="dimmed">
                      {t('status.lastChecked')}
                    </Text>
                    <Text size="sm" fw={600}>
                      {lastChecked === null ? '—' : formatTime(lastChecked, language)}
                    </Text>
                    <Button
                      variant="default"
                      size="xs"
                      className="panel-illuminate"
                      leftSection={<IconRefresh size={14} />}
                      loading={isRefreshing}
                      onClick={handleRefresh}
                      mt={4}
                    >
                      {t('status.refresh')}
                    </Button>
                  </Stack>
                </Group>
                <Accordion
                  variant="separated"
                  radius="md"
                  multiple
                  value={expanded}
                  onChange={setExpanded}
                  classNames={{
                    root: 'status-accordion',
                    item: 'status-service-item',
                    control: 'status-service-control',
                  }}
                >
                  {probes.map((probe) => (
                    <ServiceStatusSection key={probe.service} probe={probe} />
                  ))}
                </Accordion>
              </>
            )}
          </Stack>
        </Container>
      </Box>
      <BottomBar />
    </>
  );
}
