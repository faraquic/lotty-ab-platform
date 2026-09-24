import { Button, Group, Stack, Text } from '@mantine/core';
import { useTranslation } from 'react-i18next';
import { canComplete, canRollout, lifecycleActionsFor } from '../lib/transitions';
import type { TransitionAction } from '../api/useExperiments';
import type { Experiment } from '../types';

interface TransitionBarProps {
  experiment: Experiment;
  canWrite: boolean;
  isAdmin: boolean;
  isPending: boolean;
  onTransition: (action: TransitionAction) => void;
  onComplete: () => void;
  onRollout: () => void;
}

export function TransitionBar({
  experiment,
  canWrite,
  isAdmin,
  isPending,
  onTransition,
  onComplete,
  onRollout,
}: TransitionBarProps) {
  const { t } = useTranslation();
  const actions = lifecycleActionsFor(experiment.status);
  const showComplete = canComplete(experiment);
  const showRollout = canRollout(experiment);
  const showInternal = isAdmin && experiment.status === 'running';

  if (!canWrite) {
    return (
      <Text size="sm" c="dimmed">
        {t('experiments.transitionsReadOnlyHint')}
      </Text>
    );
  }

  if (
    actions.length === 0 &&
    !showComplete &&
    !showRollout &&
    !showInternal &&
    !experiment.guardrail_paused
  ) {
    return (
      <Text size="sm" c="dimmed">
        {t('experiments.noTransitions')}
      </Text>
    );
  }

  return (
    <Stack gap="xs">
      {experiment.guardrail_paused ? (
        <Text size="sm" c="orange">
          {t('experiments.guardrailPausedHint')}
        </Text>
      ) : null}
      <Group gap="xs">
        {actions.map((action) => (
          <Button
            key={action}
            size="xs"
            variant={action === 'submit' || action === 'start' ? 'filled' : 'default'}
            loading={isPending}
            disabled={isPending}
            onClick={() => {
              onTransition(action);
            }}
          >
            {t(`experiments.transitions.${action}`)}
          </Button>
        ))}
        {showComplete ? (
          <Button
            size="xs"
            variant="default"
            disabled={isPending}
            onClick={onComplete}
          >
            {t('experiments.transitions.complete')}
          </Button>
        ) : null}
        {showRollout ? (
          <Button
            size="xs"
            variant="default"
            disabled={isPending}
            onClick={onRollout}
          >
            {t('experiments.transitions.rollout')}
          </Button>
        ) : null}
        {showInternal ? (
          <>
            <Button
              size="xs"
              variant="default"
              color="orange"
              disabled={isPending}
              onClick={() => {
                onTransition('internal-pause');
              }}
            >
              {t('experiments.transitions.internalPause')}
            </Button>
            <Button
              size="xs"
              variant="default"
              color="orange"
              disabled={isPending}
              onClick={() => {
                onTransition('internal-rollback');
              }}
            >
              {t('experiments.transitions.internalRollback')}
            </Button>
          </>
        ) : null}
      </Group>
    </Stack>
  );
}
