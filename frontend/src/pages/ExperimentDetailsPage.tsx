import {
  Alert,
  Badge,
  Button,
  Card,
  Container,
  Grid,
  Group,
  Skeleton,
  Stack,
  Table,
  Text,
  Textarea,
  TextInput,
  Title,
} from '@mantine/core';
import { useForm } from '@mantine/form';
import { modals } from '@mantine/modals';
import { notifications } from '@mantine/notifications';
import { IconAlertCircle, IconArrowLeft, IconRefresh } from '@tabler/icons-react';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Link, useParams } from 'react-router';
import { CompleteExperimentModal } from '@/features/experiments/components/CompleteExperimentModal';
import type { CompleteFormValues } from '@/features/experiments/components/CompleteExperimentModal';
import { CreateVersionModal } from '@/features/experiments/components/CreateVersionModal';
import { RolloutExperimentModal } from '@/features/experiments/components/RolloutExperimentModal';
import type { RolloutFormValues } from '@/features/experiments/components/RolloutExperimentModal';
import { TransitionBar } from '@/features/experiments/components/TransitionBar';
import { VariantsEditor } from '@/features/experiments/components/VariantsEditor';
import {
  useCompleteExperiment,
  useCreateExperimentVersion,
  useExperiment,
  useRolloutExperiment,
  useSetExperimentVariants,
  useTransitionExperiment,
  useUpdateExperiment,
} from '@/features/experiments/api/useExperiments';
import type { TransitionAction } from '@/features/experiments/api/useExperiments';
import { resolveExperimentsErrorMessage } from '@/features/experiments/lib/experimentsErrorMessage';
import { formatExperimentDateTime } from '@/features/experiments/lib/format';
import {
  MAX_DESCRIPTION_LENGTH,
  MAX_NAME_LENGTH,
  canManageDistribution,
  statusBadgeColor,
  validateVariantDrafts,
} from '@/features/experiments/lib/transitions';
import type { VariantDraft } from '@/features/experiments/lib/transitions';
import { useFlag } from '@/features/flags/api/useFlags';
import { formatDefaultValue } from '@/features/flags/lib/flagValue';
import type { FlagDefaultValue, FlagType } from '@/features/flags/types';
import { useMe } from '@/features/users/api/useUsers';
import type {
  CompleteExperimentRequest,
  CreateExperimentVersionRequest,
  Experiment,
} from '@/features/experiments/types';
import { bpToPercent } from '@/features/experiments/lib/transitions';

function variantDraftsFromExperiment(experiment: Experiment): VariantDraft[] {
  if (experiment.variants.length === 0) {
    return [
      { name: '', valueRaw: '', weightRaw: '', isControl: true },
      { name: '', valueRaw: '', weightRaw: '', isControl: false },
    ];
  }
  return experiment.variants.map((variant) => ({
    name: variant.name,
    valueRaw: formatDefaultValue(variant.value as FlagDefaultValue),
    weightRaw: String(bpToPercent(variant.weight_bp)),
    isControl: variant.is_control,
  }));
}

function OverviewEditor({
  experiment,
  isPending,
  onSave,
}: {
  experiment: Experiment;
  isPending: boolean;
  onSave: (values: { name: string; description: string }) => void;
}) {
  const { t, i18n } = useTranslation();
  const form = useForm<{ name: string; description: string }>({
    initialValues: { name: experiment.name, description: experiment.description ?? '' },
    validate: {
      name: (value) => {
        if (value.trim().length === 0) {
          return i18n.t('experiments.nameRequired');
        }
        if (value.length > MAX_NAME_LENGTH) {
          return i18n.t('experiments.nameMaxLength', { count: MAX_NAME_LENGTH });
        }
        return null;
      },
      description: (value) =>
        value.length > MAX_DESCRIPTION_LENGTH
          ? i18n.t('experiments.descriptionMaxLength', { count: MAX_DESCRIPTION_LENGTH })
          : null,
    },
  });

  return (
    <form id="experiment-overview-form" onSubmit={form.onSubmit(onSave)} noValidate>
      <Stack gap="sm">
        <TextInput label={t('experiments.name')} disabled={isPending} {...form.getInputProps('name')} />
        <Textarea
          label={t('experiments.description')}
          description={t('experiments.descriptionClearHint')}
          disabled={isPending}
          autosize
          minRows={2}
          {...form.getInputProps('description')}
        />
      </Stack>
    </form>
  );
}

export function ExperimentDetailsPage() {
  const { t, i18n } = useTranslation();
  const { id } = useParams();
  const experimentId = typeof id === 'string' ? id : null;

  const query = useExperiment(experimentId);
  const meQuery = useMe();
  const experiment = query.data ?? null;
  const flagQuery = useFlag(experiment?.flag_id ?? null);
  const flagType: FlagType = flagQuery.data?.type ?? 'string';
  const flagLabel = flagQuery.data?.name ?? experiment?.flag_id ?? '';

  const [versionOpened, setVersionOpened] = useState(false);
  const [completeOpened, setCompleteOpened] = useState(false);
  const [rolloutOpened, setRolloutOpened] = useState(false);
  const [draftState, setDraftState] = useState<{ forId: string; drafts: VariantDraft[] } | null>(
    null,
  );

  const role = meQuery.data?.role;
  const canWrite = role === 'admin' || role === 'experimenter';
  const isOwner = experiment !== null && meQuery.data?.id === experiment.owner_id;
  const canMutate = canWrite && (isOwner || role === 'admin');
  const isDraft = experiment?.status === 'draft';

  const updateMutation = useUpdateExperiment();
  const versionMutation = useCreateExperimentVersion();
  const variantsMutation = useSetExperimentVariants();
  const transitionMutation = useTransitionExperiment();
  const completeMutation = useCompleteExperiment();
  const rolloutMutation = useRolloutExperiment();

  const isPending =
    updateMutation.isPending ||
    versionMutation.isPending ||
    variantsMutation.isPending ||
    transitionMutation.isPending ||
    completeMutation.isPending ||
    rolloutMutation.isPending;

  const notifyError = (id: string, error: unknown): void => {
    notifications.show({
      id,
      title: t('experiments.errorTitle'),
      message: resolveExperimentsErrorMessage(
        error as Parameters<typeof resolveExperimentsErrorMessage>[0],
        t,
      ),
      color: 'red',
    });
  };

  const notifyOk = (id: string, message: string): void => {
    notifications.show({ id, title: t('experiments.successTitle'), message, color: 'green' });
  };

  if (query.isPending) {
    return (
      <Container size="lg" py="md">
        <Stack gap="xs" aria-label={t('experiments.details')}>
          <Skeleton height={28} radius="sm" />
          <Skeleton height={120} radius="sm" />
          <Skeleton height={120} radius="sm" />
        </Stack>
      </Container>
    );
  }

  if (query.isError) {
    return (
      <Container size="lg" py="md">
        <Alert
          variant="light"
          color="red"
          title={t('experiments.errorTitle')}
          icon={<IconAlertCircle size={16} />}
        >
          <Stack gap="sm" align="flex-start">
            <Text size="sm">{resolveExperimentsErrorMessage(query.error, t)}</Text>
            <Button
              size="xs"
              variant="default"
              leftSection={<IconRefresh size={14} />}
              loading={query.isFetching}
              onClick={() => {
                void query.refetch();
              }}
            >
              {t('experiments.retry')}
            </Button>
          </Stack>
        </Alert>
      </Container>
    );
  }

  if (experiment === null) {
    return null;
  }

  const drafts =
    draftState !== null && draftState.forId === experiment.id
      ? draftState.drafts
      : variantDraftsFromExperiment(experiment);
  const setDrafts = (next: VariantDraft[]): void => {
    setDraftState({ forId: experiment.id, drafts: next });
  };
  const weightsTotal = experiment.current_version?.weights_total ?? 10000;
  const variantsValidation = validateVariantDrafts(drafts, weightsTotal, flagType);

  const handleSaveOverview = (values: { name: string; description: string }): void => {
    const request: { version: number; name?: string; description?: string } = {
      version: experiment.version,
    };
    if (values.name.trim() !== experiment.name) {
      request.name = values.name.trim();
    }
    const description = values.description.trim();
    if (description !== (experiment.description ?? '')) {
      request.description = description;
    }
    if (request.name === undefined && request.description === undefined) {
      return;
    }
    updateMutation.mutate(
      { id: experiment.id, request },
      {
        onSuccess: () => {
          notifyOk('experiments-updated', t('experiments.experimentUpdated'));
        },
        onError: (error) => {
          notifyError('experiments-update-error', error);
        },
      },
    );
  };

  const handleTransition = (action: TransitionAction): void => {
    modals.openConfirmModal({
      title: t(`experiments.transitionTitles.${action}`),
      centered: true,
      children: (
        <Text size="sm">
          {t(`experiments.transitionConfirms.${action}`, { name: experiment.name })}
        </Text>
      ),
      labels: { confirm: t('experiments.confirm'), cancel: t('experiments.cancel') },
      confirmProps: {
        color: action === 'internal-rollback' ? 'red' : undefined,
      },
      onConfirm: () => {
        transitionMutation.mutate(
          { id: experiment.id, action, request: { version: experiment.version } },
          {
            onSuccess: () => {
              notifyOk('experiments-transition', t('experiments.transitionDone'));
            },
            onError: (error) => {
              notifyError('experiments-transition-error', error);
            },
          },
        );
      },
    });
  };

  const handleComplete = (values: CompleteFormValues): void => {
    if (values.decision === '') {
      return;
    }
    const request: CompleteExperimentRequest = {
      version: experiment.version,
      decision: values.decision,
      reason: values.reason.trim(),
      ...(values.decision === 'rollout_winner'
        ? { winner_variant_id: values.winner_variant_id }
        : {}),
    };
    completeMutation.mutate(
      { id: experiment.id, request },
      {
        onSuccess: () => {
          notifyOk('experiments-completed', t('experiments.experimentCompleted'));
          setCompleteOpened(false);
        },
        onError: (error) => {
          notifyError('experiments-complete-error', error);
        },
      },
    );
  };

  const handleRollout = (values: RolloutFormValues): void => {
    rolloutMutation.mutate(
      {
        id: experiment.id,
        request: {
          version: experiment.version,
          reason: values.reason.trim(),
          winner_variant_id: values.winner_variant_id,
        },
      },
      {
        onSuccess: () => {
          notifyOk('experiments-rollout', t('experiments.experimentRolledOut'));
          setRolloutOpened(false);
        },
        onError: (error) => {
          notifyError('experiments-rollout-error', error);
        },
      },
    );
  };

  const handleCreateVersion = (request: CreateExperimentVersionRequest): void => {
    versionMutation.mutate(
      { id: experiment.id, request: { ...request, version: experiment.version } },
      {
        onSuccess: (next) => {
          notifyOk('experiments-version', t('experiments.versionCreated'));
          setVersionOpened(false);
          setDraftState({ forId: next.id, drafts: variantDraftsFromExperiment(next) });
        },
        onError: (error) => {
          notifyError('experiments-version-error', error);
        },
      },
    );
  };

  const handleSaveVariants = (): void => {
    if (variantsValidation.variants === null) {
      return;
    }
    variantsMutation.mutate(
      {
        id: experiment.id,
        request: { version: experiment.version, variants: variantsValidation.variants },
      },
      {
        onSuccess: () => {
          notifyOk('experiments-variants', t('experiments.variantsSaved'));
        },
        onError: (error) => {
          notifyError('experiments-variants-error', error);
        },
      },
    );
  };

  const variantOptions = experiment.variants.map((variant) => ({
    value: variant.id,
    label: `${variant.name} — ${formatDefaultValue(variant.value as FlagDefaultValue)}${
      variant.is_control ? ` (${t('experiments.control')})` : ''
    }`,
  }));

  return (
    <Container size="lg" py="md">
      <Stack gap="md">
        <Group gap="sm">
          <Button
            size="xs"
            variant="subtle"
            leftSection={<IconArrowLeft size={14} />}
            component={Link}
            to="/experiments"
          >
            {t('experiments.backToList')}
          </Button>
        </Group>

        <Group justify="space-between" align="center">
          <Title order={2} size="1.25rem" fw={600}>
            {experiment.name}
          </Title>
          <Badge size="lg" color={statusBadgeColor(experiment.status)} variant="light">
            {t(`experiments.statuses.${experiment.status}`)}
          </Badge>
        </Group>

        {experiment.guardrail_paused ? (
          <Alert variant="light" color="orange" title={t('experiments.guardrailPaused')}>
            {t('experiments.guardrailPausedHint')}
          </Alert>
        ) : null}

        <Grid gutter="md">
          <Grid.Col span={{ base: 12, md: 8 }}>
            <Stack gap="md">
              <Card withBorder padding="md" radius="sm">
                <Stack gap="sm">
                  <Text size="sm" fw={600}>
                    {t('experiments.overview')}
                  </Text>
                  {isDraft && canMutate ? (
                    <OverviewEditor
                      key={`${experiment.id}:${String(experiment.version)}`}
                      experiment={experiment}
                      isPending={isPending}
                      onSave={handleSaveOverview}
                    />
                  ) : (
                    <Text size="sm" c="dimmed">
                      {experiment.description ?? t('experiments.noDescription')}
                    </Text>
                  )}
                  <Group gap="xl">
                    <Text size="xs" c="dimmed">
                      {t('experiments.fieldVersion', { version: experiment.version })}
                    </Text>
                    <Text size="xs" c="dimmed">
                      {t('experiments.fieldOwner', { owner: experiment.owner_id })}
                    </Text>
                    <Text size="xs" c="dimmed">
                      {t('experiments.fieldFlag', { flag: flagLabel })}
                    </Text>
                  </Group>
                  {experiment.completion_decision !== null ? (
                    <Text size="sm">
                      {t('experiments.completionInfo', {
                        decision: t(`experiments.decisions.${experiment.completion_decision}`),
                        reason: experiment.completion_reason ?? '',
                      })}
                    </Text>
                  ) : null}
                  <Text size="xs" c="dimmed">
                    {t('experiments.metaLine', {
                      created: formatExperimentDateTime(experiment.created_at, i18n.language),
                      updated: formatExperimentDateTime(experiment.updated_at, i18n.language),
                    })}
                  </Text>
                  {!canMutate && canWrite ? (
                    <Text size="xs" c="dimmed">
                      {t('experiments.ownerOnlyHint')}
                    </Text>
                  ) : null}
                  {!canWrite ? (
                    <Text size="xs" c="dimmed">
                      {t('experiments.readOnlyHint')}
                    </Text>
                  ) : null}
                </Stack>
              </Card>
            </Stack>
          </Grid.Col>
          <Grid.Col span={{ base: 12, md: 4 }}>
            <Card withBorder padding="md" radius="sm">
              <Stack gap="sm">
                <Text size="sm" fw={600}>
                  {t('experiments.actionsTitle')}
                </Text>
                <TransitionBar
                  experiment={experiment}
                  canWrite={canMutate}
                  canSave={isDraft && canMutate}
                  isAdmin={role === 'admin'}
                  isPending={isPending}
                  onTransition={handleTransition}
                  onComplete={() => {
                    setCompleteOpened(true);
                  }}
                  onRollout={() => {
                    setRolloutOpened(true);
                  }}
                />
              </Stack>
            </Card>
          </Grid.Col>
        </Grid>

        <Card withBorder padding="md" radius="sm">
          <Stack gap="sm">
            <Group justify="space-between" align="center">
              <Text size="sm" fw={600}>
                {t('experiments.currentVersion', {
                  num: experiment.current_version?.version_num ?? '—',
                })}
              </Text>
              {canManageDistribution(experiment) && canMutate ? (
                <Button
                  size="xs"
                  variant="default"
                  disabled={isPending}
                  onClick={() => {
                    setVersionOpened(true);
                  }}
                >
                  {t('experiments.newVersion')}
                </Button>
              ) : null}
            </Group>
            {experiment.current_version !== null ? (
              <Stack gap="xs">
                <Group gap="xl">
                  <Text size="xs" c="dimmed">
                    {t('experiments.fieldWeightsTotal', {
                      total: `${bpToPercent(experiment.current_version.weights_total).toFixed(2)}%`,
                    })}
                  </Text>
                  <Text size="xs" c="dimmed">
                    {t('experiments.fieldSalt', {
                      salt: experiment.current_version.distribution_salt,
                    })}
                  </Text>
                </Group>
                <Text size="xs" c="dimmed">
                  {t('experiments.fieldTargeting', {
                    targeting:
                      experiment.current_version.targeting === null
                        ? t('experiments.noTargeting')
                        : experiment.current_version.targeting,
                  })}
                </Text>
                {experiment.current_version.review_id !== null ? (
                  <Text size="xs">
                    {t('experiments.linkedReview', {
                      review: experiment.current_version.review_id,
                    })}
                  </Text>
                ) : null}
              </Stack>
            ) : (
              <Text size="sm" c="dimmed">
                {t('experiments.noVersion')}
              </Text>
            )}
          </Stack>
        </Card>

        <Card withBorder padding="md" radius="sm">
          <Stack gap="sm">
            <Group justify="space-between" align="center">
              <Text size="sm" fw={600}>
                {t('experiments.variantsTitle')}
              </Text>
              {canManageDistribution(experiment) && canMutate ? (
                <Button
                  size="xs"
                  loading={variantsMutation.isPending}
                  disabled={isPending || variantsValidation.variants === null}
                  onClick={handleSaveVariants}
                >
                  {t('experiments.saveVariants')}
                </Button>
              ) : null}
            </Group>
            {experiment.variants.length > 0 ? (
              <Table striped highlightOnHover withTableBorder>
                <Table.Thead>
                  <Table.Tr>
                    <Table.Th>{t('experiments.variantName')}</Table.Th>
                    <Table.Th>{t('experiments.variantValue')}</Table.Th>
                    <Table.Th>{t('experiments.variantWeight')}</Table.Th>
                    <Table.Th>{t('experiments.variantControl')}</Table.Th>
                  </Table.Tr>
                </Table.Thead>
                <Table.Tbody>
                  {experiment.variants.map((variant) => (
                    <Table.Tr key={variant.id}>
                      <Table.Td>
                        <Text size="sm">{variant.name}</Text>
                      </Table.Td>
                      <Table.Td>
                        <Text size="sm" ff="monospace">
                          {formatDefaultValue(variant.value as FlagDefaultValue)}
                        </Text>
                      </Table.Td>
                      <Table.Td>
                        <Text size="sm">{bpToPercent(variant.weight_bp).toFixed(2)}%</Text>
                      </Table.Td>
                      <Table.Td>
                        {variant.is_control ? (
                          <Badge size="sm" color="green" variant="light">
                            {t('experiments.control')}
                          </Badge>
                        ) : (
                          <Text size="sm" c="dimmed">
                            —
                          </Text>
                        )}
                      </Table.Td>
                    </Table.Tr>
                  ))}
                </Table.Tbody>
              </Table>
            ) : (
              <Text size="sm" c="dimmed">
                {t('experiments.noVariants')}
              </Text>
            )}
            {canManageDistribution(experiment) && canMutate ? (
              <VariantsEditor
                drafts={drafts}
                weightsTotal={weightsTotal}
                flagType={flagType}
                validationError={variantsValidation.error}
                weightsSum={variantsValidation.weightsSum}
                disabled={isPending}
                onChange={setDrafts}
              />
            ) : null}
          </Stack>
        </Card>
      </Stack>

      <CreateVersionModal
        opened={versionOpened}
        isPending={versionMutation.isPending}
        flagType={flagType}
        currentWeightsTotal={weightsTotal}
        currentTargeting={experiment.current_version?.targeting ?? null}
        onClose={() => {
          setVersionOpened(false);
        }}
        onSubmit={handleCreateVersion}
      />
      <CompleteExperimentModal
        opened={completeOpened}
        isPending={completeMutation.isPending}
        variantOptions={variantOptions}
        onClose={() => {
          setCompleteOpened(false);
        }}
        onSubmit={handleComplete}
      />
      <RolloutExperimentModal
        opened={rolloutOpened}
        isPending={rolloutMutation.isPending}
        variantOptions={variantOptions}
        onClose={() => {
          setRolloutOpened(false);
        }}
        onSubmit={handleRollout}
      />
    </Container>
  );
}
