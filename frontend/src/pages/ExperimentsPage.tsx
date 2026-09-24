import {
  Alert,
  Button,
  Container,
  Group,
  Pagination,
  Select,
  Skeleton,
  Stack,
  Text,
  Title,
} from '@mantine/core';
import { notifications } from '@mantine/notifications';
import { IconAlertCircle, IconPlus, IconRefresh } from '@tabler/icons-react';
import { useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { CreateExperimentModal } from '@/features/experiments/components/CreateExperimentModal';
import { ExperimentsTable } from '@/features/experiments/components/ExperimentsTable';
import {
  EXPERIMENTS_PAGE_SIZE,
  useCreateExperiment,
  useExperimentsList,
} from '@/features/experiments/api/useExperiments';
import { resolveExperimentsErrorMessage } from '@/features/experiments/lib/experimentsErrorMessage';
import { FLAGS_PAGE_SIZE, useFlagsList } from '@/features/flags/api/useFlags';
import { useMe } from '@/features/users/api/useUsers';
import { lastPageIndex, offsetForPage, totalPages } from '@/shared/lib/pagination';
import type { CreateExperimentRequest, ExperimentStatus } from '@/features/experiments/types';
import { EXPERIMENT_STATUSES } from '@/features/experiments/types';

const LIMIT = EXPERIMENTS_PAGE_SIZE;

export function ExperimentsPage() {
  const { t } = useTranslation();
  const [page, setPage] = useState(0);
  const [status, setStatus] = useState<ExperimentStatus | null>(null);
  const [createOpened, setCreateOpened] = useState(false);

  const offset = offsetForPage(page, LIMIT);
  const list = useExperimentsList({ limit: LIMIT, offset, status });
  const meQuery = useMe();
  const flagsQuery = useFlagsList({ limit: FLAGS_PAGE_SIZE, offset: 0 });

  const role = meQuery.data?.role;
  const canWrite = role === 'admin' || role === 'experimenter';

  const meta = list.data?.meta;
  const experiments = list.data?.data ?? [];
  const pages = totalPages(meta?.total ?? 0, LIMIT);

  const flagNames = useMemo(() => {
    const map = new Map<string, string>();
    for (const flag of flagsQuery.data?.data ?? []) {
      map.set(flag.id, flag.key);
    }
    return map;
  }, [flagsQuery.data]);

  const flagOptions = useMemo(
    () =>
      (flagsQuery.data?.data ?? []).map((flag) => ({
        value: flag.id,
        label: `${flag.key} — ${flag.name}`,
      })),
    [flagsQuery.data],
  );

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

  const createMutation = useCreateExperiment();

  const handleCreate = (request: CreateExperimentRequest): void => {
    createMutation.mutate(request, {
      onSuccess: () => {
        notifications.show({
          id: 'experiments-created',
          title: t('experiments.successTitle'),
          message: t('experiments.experimentCreated'),
          color: 'green',
        });
        setCreateOpened(false);
        const currentTotal = list.data?.meta.total ?? 0;
        setPage(lastPageIndex(currentTotal + 1, LIMIT));
      },
      onError: (error) => {
        notifications.show({
          id: 'experiments-create-error',
          title: t('experiments.errorTitle'),
          message: resolveExperimentsErrorMessage(error, t),
          color: 'red',
        });
      },
    });
  };

  const isInitialLoading = list.isPending;

  return (
    <Container size="lg" py="md">
      <Stack gap="md">
        <Group justify="space-between" align="center">
          <Title order={2} size="1.25rem" fw={600}>
            {t('experiments.title')}
          </Title>
          {canWrite ? (
            <Button
              size="sm"
              leftSection={<IconPlus size={16} />}
              onClick={() => {
                setCreateOpened(true);
              }}
              disabled={list.isError}
            >
              {t('experiments.createExperiment')}
            </Button>
          ) : (
            <Text size="sm" c="dimmed">
              {t('experiments.readOnlyHint')}
            </Text>
          )}
        </Group>

        <Group gap="sm">
          <Select
            size="sm"
            placeholder={t('experiments.statusFilter')}
            clearable
            value={status}
            data={EXPERIMENT_STATUSES.map((value) => ({
              value,
              label: t(`experiments.statuses.${value}`),
            }))}
            onChange={(next) => {
              setStatus((next ?? '') === '' ? null : (next as ExperimentStatus));
              setPage(0);
            }}
            style={{ minWidth: 200 }}
            aria-label={t('experiments.statusFilter')}
          />
        </Group>

        {isInitialLoading ? (
          <Stack gap="xs" aria-label={t('experiments.title')}>
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
            title={t('experiments.errorTitle')}
            icon={<IconAlertCircle size={16} />}
          >
            <Stack gap="sm" align="flex-start">
              <Text size="sm">{resolveExperimentsErrorMessage(list.error, t)}</Text>
              <Button
                size="xs"
                variant="default"
                leftSection={<IconRefresh size={14} />}
                loading={list.isFetching}
                onClick={() => {
                  void list.refetch();
                }}
              >
                {t('experiments.retry')}
              </Button>
            </Stack>
          </Alert>
        ) : null}

        {!isInitialLoading && !list.isError && meta !== undefined && meta.total === 0 ? (
          <Stack gap="sm" align="center" py="xl">
            <Text size="sm" fw={600}>
              {t('experiments.noExperiments')}
            </Text>
            <Text size="sm" c="dimmed">
              {t('experiments.noExperimentsHint')}
            </Text>
            {canWrite ? (
              <Button
                size="sm"
                leftSection={<IconPlus size={16} />}
                onClick={() => {
                  setCreateOpened(true);
                }}
              >
                {t('experiments.createExperiment')}
              </Button>
            ) : null}
          </Stack>
        ) : null}

        {!isInitialLoading && !list.isError && experiments.length > 0 ? (
          <>
            <ExperimentsTable experiments={experiments} flagNames={flagNames} />
            <Group justify="space-between" align="center">
              <Text size="xs" c="dimmed">
                {t('experiments.paginationSummary', {
                  from: offset + 1,
                  to: offset + experiments.length,
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
                  aria-label={t('experiments.title')}
                />
              ) : null}
            </Group>
          </>
        ) : null}
      </Stack>

      <CreateExperimentModal
        opened={createOpened}
        isPending={createMutation.isPending}
        flagOptions={flagOptions}
        flagsLoading={flagsQuery.isPending}
        onClose={() => {
          setCreateOpened(false);
        }}
        onSubmit={handleCreate}
      />
    </Container>
  );
}
