import { ActionIcon, Badge, Group, Menu, Table, Text, Tooltip } from '@mantine/core';
import { IconEye, IconFilter, IconFilterFilled } from '@tabler/icons-react';
import { useTranslation } from 'react-i18next';
import { Link } from 'react-router';
import { EXPERIMENT_STATUSES } from '../types';
import { statusBadgeColor } from '../lib/transitions';
import type { Experiment, ExperimentStatus } from '../types';

interface ExperimentsTableProps {
  experiments: Experiment[];
  flagNames: Map<string, string>;
  status: ExperimentStatus | null;
  onStatusChange: (status: ExperimentStatus | null) => void;
}

export function ExperimentsTable({
  experiments,
  flagNames,
  status,
  onStatusChange,
}: ExperimentsTableProps) {
  const { t } = useTranslation();

  return (
    <Table striped highlightOnHover withTableBorder withColumnBorders={false}>
      <Table.Thead>
        <Table.Tr>
          <Table.Th>{t('experiments.name')}</Table.Th>
          <Table.Th>
            <Group gap={4} wrap="nowrap">
              {t('experiments.status')}
              <Menu shadow="md" width={200} position="bottom-start">
                <Menu.Target>
                  <Tooltip label={t('experiments.statusFilter')}>
                    <ActionIcon
                      variant="subtle"
                      size="sm"
                      color={status === null ? undefined : 'cyan'}
                      aria-label={t('experiments.statusFilter')}
                    >
                      {status === null ? <IconFilter size={14} /> : <IconFilterFilled size={14} />}
                    </ActionIcon>
                  </Tooltip>
                </Menu.Target>
                <Menu.Dropdown>
                  <Menu.Item
                    onClick={() => {
                      onStatusChange(null);
                    }}
                  >
                    {t('experiments.statusFilterAll')}
                  </Menu.Item>
                  {EXPERIMENT_STATUSES.map((value) => (
                    <Menu.Item
                      key={value}
                      onClick={() => {
                        onStatusChange(value);
                      }}
                    >
                      {t(`experiments.statuses.${value}`)}
                    </Menu.Item>
                  ))}
                </Menu.Dropdown>
              </Menu>
            </Group>
          </Table.Th>
          <Table.Th>{t('experiments.flag')}</Table.Th>
          <Table.Th>{t('experiments.variants')}</Table.Th>
          <Table.Th>{t('experiments.actions')}</Table.Th>
        </Table.Tr>
      </Table.Thead>
      <Table.Tbody>
        {experiments.map((experiment) => (
          <Table.Tr key={experiment.id}>
            <Table.Td>
              <Text size="sm" fw={500}>
                {experiment.name}
              </Text>
            </Table.Td>
            <Table.Td>
              <Badge size="sm" color={statusBadgeColor(experiment.status)} variant="light">
                {t(`experiments.statuses.${experiment.status}`)}
              </Badge>
            </Table.Td>
            <Table.Td>
              <Text size="sm" c="dimmed">
                {flagNames.get(experiment.flag_id) ?? experiment.flag_id}
              </Text>
            </Table.Td>
            <Table.Td>
              <Text size="sm">{experiment.variants.length}</Text>
            </Table.Td>
            <Table.Td>
              <Tooltip label={t('experiments.viewDetails')}>
                <ActionIcon
                  variant="subtle"
                  component={Link}
                  to={`/experiments/${experiment.id}`}
                  aria-label={t('experiments.viewDetailsNamed', { name: experiment.name })}
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
