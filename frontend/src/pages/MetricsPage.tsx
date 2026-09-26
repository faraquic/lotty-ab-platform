import {
  Alert,
  Button,
  Container,
  Group,
  Pagination,
  Skeleton,
  Stack,
  Switch,
  Text,
  Title,
} from '@mantine/core';
import { modals } from '@mantine/modals';
import { notifications } from '@mantine/notifications';
import { IconAlertCircle, IconPlus, IconRefresh } from '@tabler/icons-react';
import { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { CreateMetricModal } from '@/features/metrics/components/CreateMetricModal';
import { MetricDetailsModal } from '@/features/metrics/components/MetricDetailsModal';
import { MetricsTable } from '@/features/metrics/components/MetricsTable';
import {
  METRICS_PAGE_SIZE,
  useCreateMetric,
  useMetricsList,
  useUpdateMetric,
} from '@/features/metrics/api/useMetrics';
import { resolveMetricsErrorMessage } from '@/features/metrics/lib/metricsErrorMessage';
import { lastPageIndex, offsetForPage, totalPages } from '@/shared/lib/pagination';
import type { CreateMetricRequest, Metric, UpdateMetricRequest } from '@/features/metrics/types';

const LIMIT = METRICS_PAGE_SIZE;

export function MetricsPage() {
  const { t } = useTranslation();
  const [page, setPage] = useState(0);
  const [showArchived, setShowArchived] = useState(false);
  const [createOpened, setCreateOpened] = useState(false);
  const [selectedMetric, setSelectedMetric] = useState<Metric | null>(null);

  const offset = offsetForPage(page, LIMIT);
  const list = useMetricsList({ limit: LIMIT, offset, archived: showArchived });

  const meta = list.data?.meta;
  const metrics = list.data?.data ?? [];
  const pages = totalPages(meta?.total ?? 0, LIMIT);

  useEffect(() => {
    if (meta !== undefined && meta.total > 0) {
      const last = lastPageIndex(meta.total, LIMIT);
      if (page > last) {
        setPage(last);
      }
    }
    if (meta !== undefined && meta.total === 0 && page !== 0) {
      setPage(0);
    }
  }, [meta, page]);

  const createMutation = useCreateMetric();
  const updateMutation = useUpdateMetric();

  const handleCreate = (request: CreateMetricRequest): void => {
    createMutation.mutate(request, {
      onSuccess: () => {
        notifications.show({
          id: 'metrics-created',
          title: t('metrics.successTitle'),
          message: t('metrics.metricCreated'),
          color: 'green',
        });
        setCreateOpened(false);
        const currentTotal = list.data?.meta.total ?? 0;
        setPage(lastPageIndex(currentTotal + 1, LIMIT));
      },
      onError: (error) => {
        notifications.show({
          id: 'metrics-create-error',
          title: t('metrics.errorTitle'),
          message: resolveMetricsErrorMessage(error, t),
          color: 'red',
        });
      },
    });
  };

  const handleSave = (id: string, request: UpdateMetricRequest): void => {
    const archiving = request.status === 'archived';
    const apply = (): void => {
      updateMutation.mutate(
        { id, request },
        {
          onSuccess: (metric) => {
            notifications.show({
              id: 'metrics-updated',
              title: t('metrics.successTitle'),
              message: t(archiving ? 'metrics.metricArchived' : 'metrics.metricUpdated'),
              color: 'green',
            });
            setSelectedMetric(metric);
          },
          onError: (error) => {
            notifications.show({
              id: 'metrics-update-error',
              title: t('metrics.errorTitle'),
              message: resolveMetricsErrorMessage(error, t),
              color: 'red',
            });
          },
        },
      );
    };
    if (archiving) {
      modals.openConfirmModal({
        title: t('metrics.archiveTitle'),
        centered: true,
        children: <Text size="sm">{t('metrics.archiveConfirm')}</Text>,
        labels: { confirm: t('metrics.archive'), cancel: t('metrics.cancel') },
        confirmProps: { color: 'orange' },
        onConfirm: apply,
      });
      return;
    }
    apply();
  };

  const isInitialLoading = list.isPending;

  return (
    <Container size="lg" py="md">
      <Stack gap="md">
        <Group justify="space-between" align="center">
          <Title order={2} size="1.25rem" fw={600}>
            {t('metrics.title')}
          </Title>
          <Button
            size="sm"
            leftSection={<IconPlus size={16} />}
            onClick={() => {
              setCreateOpened(true);
            }}
            disabled={list.isError}
          >
            {t('metrics.createMetric')}
          </Button>
        </Group>

        <Group gap="sm">
          <Switch
            label={t('metrics.showArchived')}
            checked={showArchived}
            onChange={(event) => {
              setShowArchived(event.currentTarget.checked);
              setPage(0);
            }}
            aria-label={t('metrics.showArchived')}
          />
        </Group>

        {isInitialLoading ? (
          <Stack gap="xs" aria-label={t('metrics.title')}>
            <Skeleton height={38} radius="sm" />
            <Skeleton height={52} radius="sm" />
            <Skeleton height={52} radius="sm" />
            <Skeleton height={52} radius="sm" />
          </Stack>
        ) : null}

        {list.isError ? (
          <Alert
            variant="light"
            color="red"
            title={t('metrics.errorTitle')}
            icon={<IconAlertCircle size={16} />}
          >
            <Stack gap="sm" align="flex-start">
              <Text size="sm">{resolveMetricsErrorMessage(list.error, t)}</Text>
              <Button
                size="xs"
                variant="default"
                leftSection={<IconRefresh size={14} />}
                loading={list.isFetching}
                onClick={() => {
                  void list.refetch();
                }}
              >
                {t('metrics.retry')}
              </Button>
            </Stack>
          </Alert>
        ) : null}

        {!isInitialLoading && !list.isError && meta !== undefined && meta.total === 0 ? (
          <Stack gap="sm" align="center" py="xl">
            <Text size="sm" fw={600}>
              {t('metrics.noMetrics')}
            </Text>
            <Text size="sm" c="dimmed">
              {t('metrics.noMetricsHint')}
            </Text>
          </Stack>
        ) : null}

        {!isInitialLoading && !list.isError && metrics.length > 0 ? (
          <>
            <MetricsTable metrics={metrics} onView={setSelectedMetric} />
            <Group justify="space-between" align="center">
              <Text size="xs" c="dimmed">
                {t('metrics.paginationSummary', {
                  from: offset + 1,
                  to: offset + metrics.length,
                  total: meta?.total ?? 0,
                })}
              </Text>
              {pages > 1 ? (
                <Pagination
                  size="sm"
                  value={page + 1}
                  total={pages}
                  onChange={(next) => {
                    setPage(next - 1);
                  }}
                  aria-label={t('metrics.title')}
                />
              ) : null}
            </Group>
          </>
        ) : null}
      </Stack>

      <CreateMetricModal
        opened={createOpened}
        isPending={createMutation.isPending}
        onClose={() => {
          setCreateOpened(false);
        }}
        onSubmit={handleCreate}
      />
      <MetricDetailsModal
        metric={selectedMetric}
        isSaving={updateMutation.isPending}
        onClose={() => {
          setSelectedMetric(null);
        }}
        onSave={handleSave}
      />
    </Container>
  );
}
