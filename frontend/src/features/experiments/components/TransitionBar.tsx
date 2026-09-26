import { Button, Stack, Text } from '@mantine/core';
import {
  IconAlertTriangle,
  IconArchive,
  IconCheck,
  IconDeviceFloppy,
  IconPlayerPause,
  IconPlayerPlay,
  IconPlayerTrackNext,
  IconRocket,
  IconSend,
} from '@tabler/icons-react';
import type { ComponentType } from 'react';
import { useTranslation } from 'react-i18next';
import { canComplete, canRollout, lifecycleActionsFor } from '../lib/transitions';
import type { TransitionAction } from '../api/useExperiments';
import type { Experiment } from '../types';

interface TransitionBarProps {
  experiment: Experiment;
  canWrite: boolean;
  canSave: boolean;
  isAdmin: boolean;
  isPending: boolean;
  onTransition: (action: TransitionAction) => void;
  onComplete: () => void;
  onRollout: () => void;
}

type IconProps = { size?: number | string };

function actionIcon(action: TransitionAction | 'save' | 'complete' | 'rollout'): ComponentType<IconProps> {
  switch (action) {
    case 'save':
      return IconDeviceFloppy;
    case 'submit':
      return IconSend;
    case 'start':
      return IconPlayerPlay;
    case 'pause':
      return IconPlayerPause;
    case 'resume':
      return IconPlayerTrackNext;
    case 'archive':
      return IconArchive;
    case 'complete':
      return IconCheck;
    case 'rollout':
      return IconRocket;
    case 'internal-pause':
    case 'internal-rollback':
      return IconAlertTriangle;
  }
}

export function TransitionBar({
  experiment,
  canWrite,
  canSave,
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

  const hasActions =
    canSave || actions.length > 0 || showComplete || showRollout || showInternal;

  if (!canWrite) {
    return (
      <Text size="sm" c="dimmed">
        {t('experiments.transitionsReadOnlyHint')}
      </Text>
    );
  }

  if (!hasActions && !experiment.guardrail_paused) {
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
      <Stack gap={6}>
        {canSave ? (
          <Button
            size="sm"
            variant="default"
            justify="flex-start"
            type="submit"
            form="experiment-overview-form"
            leftSection={<IconDeviceFloppy size={16} />}
            disabled={isPending}
          >
            {t('experiments.save')}
          </Button>
        ) : null}
        {actions.map((action) => {
          const Icon = actionIcon(action);
          return (
            <Button
              key={action}
              size="sm"
              variant={action === 'submit' || action === 'start' ? 'filled' : 'default'}
              justify="flex-start"
              leftSection={<Icon size={16} />}
              loading={isPending}
              disabled={isPending}
              onClick={() => {
                onTransition(action);
              }}
            >
              {t(`experiments.transitions.${action}`)}
            </Button>
          );
        })}
        {showComplete ? (
          <Button
            size="sm"
            variant="default"
            justify="flex-start"
            leftSection={<IconCheck size={16} />}
            disabled={isPending}
            onClick={onComplete}
          >
            {t('experiments.transitions.complete')}
          </Button>
        ) : null}
        {showRollout ? (
          <Button
            size="sm"
            variant="default"
            justify="flex-start"
            leftSection={<IconRocket size={16} />}
            disabled={isPending}
            onClick={onRollout}
          >
            {t('experiments.transitions.rollout')}
          </Button>
        ) : null}
        {showInternal ? (
          <>
            <Button
              size="sm"
              variant="default"
              color="orange"
              justify="flex-start"
              leftSection={<IconAlertTriangle size={16} />}
              disabled={isPending}
              onClick={() => {
                onTransition('internal-pause');
              }}
            >
              {t('experiments.transitions.internalPause')}
            </Button>
            <Button
              size="sm"
              variant="default"
              color="orange"
              justify="flex-start"
              leftSection={<IconAlertTriangle size={16} />}
              disabled={isPending}
              onClick={() => {
                onTransition('internal-rollback');
              }}
            >
              {t('experiments.transitions.internalRollback')}
            </Button>
          </>
        ) : null}
      </Stack>
    </Stack>
  );
}
