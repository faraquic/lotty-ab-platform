import { ActionIcon, Badge, Group, Menu, Table, Text, Tooltip } from '@mantine/core';
import { IconEye, IconFilter, IconFilterFilled } from '@tabler/icons-react';
import { useTranslation } from 'react-i18next';
import { Link } from 'react-router';
import { countComments, countUnresolvedComments, reviewStatusBadgeColor } from '../lib/threads';
import { REVIEW_STATUSES } from '../types';
import type { Review, ReviewStatus } from '../types';

interface ReviewsTableProps {
  reviews: Review[];
  experimentNames: Map<string, string>;
  status: ReviewStatus | null;
  onStatusChange: (status: ReviewStatus | null) => void;
}

export function ReviewsTable({
  reviews,
  experimentNames,
  status,
  onStatusChange,
}: ReviewsTableProps) {
  const { t } = useTranslation();

  return (
    <Table striped highlightOnHover withTableBorder withColumnBorders={false}>
      <Table.Thead>
        <Table.Tr>
          <Table.Th>{t('reviews.experiment')}</Table.Th>
          <Table.Th>{t('reviews.version')}</Table.Th>
          <Table.Th>
            <Group gap={4} wrap="nowrap">
              {t('reviews.status')}
              <Menu shadow="md" width={200} position="bottom-start">
                <Menu.Target>
                  <Tooltip label={t('reviews.statusFilter')}>
                    <ActionIcon
                      variant="subtle"
                      size="sm"
                      color={status === null ? undefined : 'cyan'}
                      aria-label={t('reviews.statusFilter')}
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
                    {t('reviews.statusFilterAll')}
                  </Menu.Item>
                  {REVIEW_STATUSES.map((value) => (
                    <Menu.Item
                      key={value}
                      onClick={() => {
                        onStatusChange(value);
                      }}
                    >
                      {t(`reviews.statuses.${value}`)}
                    </Menu.Item>
                  ))}
                </Menu.Dropdown>
              </Menu>
            </Group>
          </Table.Th>
          <Table.Th>{t('reviews.approvals')}</Table.Th>
          <Table.Th>{t('reviews.comments')}</Table.Th>
          <Table.Th>{t('reviews.actions')}</Table.Th>
        </Table.Tr>
      </Table.Thead>
      <Table.Tbody>
        {reviews.map((review) => {
          const unresolved = countUnresolvedComments(review.comments);
          return (
            <Table.Tr key={review.id}>
              <Table.Td>
                <Text size="sm" fw={500}>
                  {experimentNames.get(review.experiment_id) ?? review.experiment_id}
                </Text>
              </Table.Td>
              <Table.Td>
                <Text size="sm">v{review.version_num}</Text>
              </Table.Td>
              <Table.Td>
                <Badge size="sm" color={reviewStatusBadgeColor(review.status)} variant="light">
                  {t(`reviews.statuses.${review.status}`)}
                </Badge>
              </Table.Td>
              <Table.Td>
                <Text size="sm">{review.approvals.length}</Text>
              </Table.Td>
              <Table.Td>
                <Text size="sm">
                  {countComments(review.comments)}
                  {unresolved > 0 ? (
                    <Text span size="sm" c="orange">
                      {' '}
                      ({t('reviews.unresolvedCount', { count: unresolved })})
                    </Text>
                  ) : null}
                </Text>
              </Table.Td>
              <Table.Td>
                <Tooltip label={t('reviews.viewDetails')}>
                  <ActionIcon
                    variant="subtle"
                    component={Link}
                    to={`/reviews/${review.id}`}
                    aria-label={t('reviews.viewDetailsNamed', { id: review.id })}
                  >
                    <IconEye size={16} />
                  </ActionIcon>
                </Tooltip>
              </Table.Td>
            </Table.Tr>
          );
        })}
      </Table.Tbody>
    </Table>
  );
}
