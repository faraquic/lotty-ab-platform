import { ActionIcon, Badge, Table, Text, Tooltip } from '@mantine/core';
import { IconEye } from '@tabler/icons-react';
import { useTranslation } from 'react-i18next';
import { countComments, countUnresolvedComments, reviewStatusBadgeColor } from '../lib/threads';
import type { Review } from '../types';

interface ReviewsTableProps {
  reviews: Review[];
  experimentNames: Map<string, string>;
  onView: (review: Review) => void;
}

export function ReviewsTable({ reviews, experimentNames, onView }: ReviewsTableProps) {
  const { t } = useTranslation();

  return (
    <Table striped highlightOnHover withTableBorder withColumnBorders={false}>
      <Table.Thead>
        <Table.Tr>
          <Table.Th>{t('reviews.experiment')}</Table.Th>
          <Table.Th>{t('reviews.version')}</Table.Th>
          <Table.Th>{t('reviews.status')}</Table.Th>
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
                    onClick={() => {
                      onView(review);
                    }}
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
