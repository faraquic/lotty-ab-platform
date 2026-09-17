import { ActionIcon, Badge, Group, Table, Text, Tooltip } from '@mantine/core';
import { IconPencil, IconTrash } from '@tabler/icons-react';
import { useTranslation } from 'react-i18next';
import { formatFlagDateTime } from '../lib/format';
import { formatDefaultValue } from '../lib/flagValue';
import type { Flag, FlagType } from '../types';

function typeBadgeColor(type: FlagType): string {
  switch (type) {
    case 'string':
      return 'cyan';
    case 'number':
      return 'violet';
    case 'bool':
      return 'amber';
  }
}

interface FlagsTableProps {
  flags: Flag[];
  canWrite: boolean;
  onView: (flag: Flag) => void;
  onDelete: (flag: Flag) => void;
  isMutating: boolean;
}

export function FlagsTable({ flags, canWrite, onView, onDelete, isMutating }: FlagsTableProps) {
  const { t, i18n } = useTranslation();
  const language = i18n.resolvedLanguage ?? 'en';

  return (
    <Table striped highlightOnHover withTableBorder verticalSpacing="sm">
      <Table.Thead>
        <Table.Tr>
          <Table.Th>{t('flags.key')}</Table.Th>
          <Table.Th>{t('flags.name')}</Table.Th>
          <Table.Th>{t('flags.type')}</Table.Th>
          <Table.Th>{t('flags.defaultValue')}</Table.Th>
          <Table.Th>{t('flags.updated')}</Table.Th>
          <Table.Th>{t('flags.actions')}</Table.Th>
        </Table.Tr>
      </Table.Thead>
      <Table.Tbody>
        {flags.map((flag) => (
          <Table.Tr key={flag.id}>
            <Table.Td>
              <Text size="sm" ff="monospace" fw={500}>
                {flag.key}
              </Text>
            </Table.Td>
            <Table.Td>
              <Text size="sm">{flag.name}</Text>
            </Table.Td>
            <Table.Td>
              <Badge size="sm" variant="light" color={typeBadgeColor(flag.type)}>
                {t(`flags.types.${flag.type}`)}
              </Badge>
            </Table.Td>
            <Table.Td>
              <Text size="sm" ff="monospace" lineClamp={1} title={formatDefaultValue(flag.default_value)}>
                {formatDefaultValue(flag.default_value)}
              </Text>
            </Table.Td>
            <Table.Td>
              <Text size="sm" c="dimmed">
                {formatFlagDateTime(flag.updated_at, language)}
              </Text>
            </Table.Td>
            <Table.Td>
              <Group gap="xs" wrap="nowrap">
                <Tooltip label={t('flags.viewEdit')}>
                  <ActionIcon
                    variant="subtle"
                    size="sm"
                    onClick={() => {
                      onView(flag);
                    }}
                    aria-label={t('flags.viewEditFlag', { name: flag.name })}
                  >
                    <IconPencil size={16} />
                  </ActionIcon>
                </Tooltip>
                {canWrite ? (
                  <Tooltip label={t('flags.deleteFlag')}>
                    <ActionIcon
                      variant="subtle"
                      size="sm"
                      color="red"
                      disabled={isMutating}
                      onClick={() => {
                        onDelete(flag);
                      }}
                      aria-label={t('flags.deleteFlagNamed', { name: flag.name })}
                    >
                      <IconTrash size={16} />
                    </ActionIcon>
                  </Tooltip>
                ) : null}
              </Group>
            </Table.Td>
          </Table.Tr>
        ))}
      </Table.Tbody>
    </Table>
  );
}
