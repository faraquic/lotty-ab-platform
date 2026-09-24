import { ActionIcon, Badge, Table, Text, Tooltip } from '@mantine/core';
import { IconEye } from '@tabler/icons-react';
import { useTranslation } from 'react-i18next';
import { Link } from 'react-router';
import { statusBadgeColor } from '../lib/transitions';
import type { Experiment } from '../types';

interface ExperimentsTableProps {
  experiments: Experiment[];
  flagNames: Map<string, string>;
}

export function ExperimentsTable({ experiments, flagNames }: ExperimentsTableProps) {
  const { t } = useTranslation();

  return (
    <Table striped highlightOnHover withTableBorder withColumnBorders={false}>
      <Table.Thead>
        <Table.Tr>
          <Table.Th>{t('experiments.name')}</Table.Th>
          <Table.Th>{t('experiments.status')}</Table.Th>
          <Table.Th>{t('experiments.flag')}</Table.Th>
          <Table.Th>{t('experiments.version')}</Table.Th>
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
              <Text size="sm">v{experiment.current_version?.version_num ?? '—'}</Text>
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
