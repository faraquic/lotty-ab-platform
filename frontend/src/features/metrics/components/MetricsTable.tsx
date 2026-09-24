import { ActionIcon, Badge, Group, Table, Text, Tooltip } from '@mantine/core';
import { IconEye, IconLock } from '@tabler/icons-react';
import { useTranslation } from 'react-i18next';
import { metricTypeBadgeColor, summarizeAggregation } from '../lib/aggregation';
import type { Metric } from '../types';

interface MetricsTableProps {
  metrics: Metric[];
  onView: (metric: Metric) => void;
}

export function MetricsTable({ metrics, onView }: MetricsTableProps) {
  const { t } = useTranslation();

  return (
    <Table striped highlightOnHover withTableBorder withColumnBorders={false}>
      <Table.Thead>
        <Table.Tr>
          <Table.Th>{t('metrics.key')}</Table.Th>
          <Table.Th>{t('metrics.name')}</Table.Th>
          <Table.Th>{t('metrics.type')}</Table.Th>
          <Table.Th>{t('metrics.aggregationSummary')}</Table.Th>
          <Table.Th>{t('metrics.status')}</Table.Th>
          <Table.Th>{t('metrics.actions')}</Table.Th>
        </Table.Tr>
      </Table.Thead>
      <Table.Tbody>
        {metrics.map((metric) => (
          <Table.Tr key={metric.id}>
            <Table.Td>
              <Text size="sm" ff="monospace">
                {metric.key}
              </Text>
            </Table.Td>
            <Table.Td>
              <Text size="sm" fw={500}>
                {metric.name}
              </Text>
            </Table.Td>
            <Table.Td>
              <Badge size="sm" color={metricTypeBadgeColor(metric.metric_type)} variant="light">
                {t(`metrics.types.${metric.metric_type}`)}
              </Badge>
            </Table.Td>
            <Table.Td>
              <Text size="sm" c="dimmed" ff="monospace">
                {summarizeAggregation(metric.metric_type, metric.aggregation)}
              </Text>
            </Table.Td>
            <Table.Td>
              <Group gap={4}>
                <Badge size="sm" color={metric.status === 'active' ? 'green' : 'gray'} variant="light">
                  {t(`metrics.statuses.${metric.status}`)}
                </Badge>
                {metric.is_builtin ? (
                  <Tooltip label={t('metrics.builtinHint')}>
                    <ActionIcon size="xs" variant="subtle" aria-label={t('metrics.builtinHint')}>
                      <IconLock size={12} />
                    </ActionIcon>
                  </Tooltip>
                ) : null}
              </Group>
            </Table.Td>
            <Table.Td>
              <Tooltip label={t('metrics.viewDetails')}>
                <ActionIcon
                  variant="subtle"
                  onClick={() => {
                    onView(metric);
                  }}
                  aria-label={t('metrics.viewDetailsNamed', { name: metric.name })}
                >
                  <IconEye size={16} />
                </ActionIcon>
              </Tooltip>
            </Table.Td>
          </Table.Tr>
        ))}
      </Table.Tbody>
    </Table>
  );
}
