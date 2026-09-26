import type { TransitionAction } from '../api/useExperiments';
import { parseDefaultValueInput } from '@/features/flags/lib/flagValue';
import type { FlagType } from '@/features/flags/types';
import type { Experiment, ExperimentStatus, ExperimentVariantInput } from '../types';
import { isValidTargetingDsl } from './targeting';

export const DEFAULT_WEIGHTS_TOTAL = 10000;
export const MAX_WEIGHTS_TOTAL = 10000;
export const MAX_DESCRIPTION_LENGTH = 4096;
export const MAX_NAME_LENGTH = 256;

export type LifecycleAction =
  | Extract<TransitionAction, 'submit' | 'start' | 'pause' | 'resume' | 'archive'>;

export function lifecycleActionsFor(status: ExperimentStatus): LifecycleAction[] {
  switch (status) {
    case 'draft':
      return ['submit'];
    case 'approved':
      return ['start'];
    case 'running':
      return ['pause'];
    case 'paused':
      return ['resume'];
    case 'completed':
      return ['archive'];
    case 'review':
    case 'archived':
    case 'rejected':
      return [];
  }
}

export function canEditDraft(experiment: Experiment): boolean {
  return experiment.status === 'draft';
}

export function canManageDistribution(experiment: Experiment): boolean {
  return experiment.status === 'draft';
}

export function canComplete(experiment: Experiment): boolean {
  return experiment.status === 'running' || experiment.status === 'paused';
}

export function canRollout(experiment: Experiment): boolean {
  return experiment.status === 'running' || experiment.status === 'paused';
}

export function isTerminalStatus(status: ExperimentStatus): boolean {
  return status === 'archived' || status === 'rejected' || status === 'completed';
}

export interface VariantDraft {
  name: string;
  valueRaw: string;
  weightRaw: string;
  isControl: boolean;
}

export function emptyVariantDraft(isControl: boolean): VariantDraft {
  return { name: '', valueRaw: '', weightRaw: '', isControl };
}

export function parseVariantValue(
  raw: string,
  flagType: FlagType,
): { ok: true; value: unknown } | { ok: false } {
  const trimmed = raw.trim();
  if (trimmed.length === 0) {
    return { ok: false };
  }
  const parsed = parseDefaultValueInput(flagType, flagType === 'string' ? raw : trimmed);
  if (parsed === null) {
    return { ok: false };
  }
  return { ok: true, value: parsed };
}

export interface VariantsValidation {
  variants: ExperimentVariantInput[] | null;
  weightsSum: number;
  error: 'count' | 'name' | 'duplicate' | 'value' | 'weight' | 'control' | 'sum' | null;
}

export function validateVariantDrafts(
  drafts: VariantDraft[],
  weightsTotal: number,
  flagType: FlagType,
): VariantsValidation {
  const weightsSum = drafts.reduce((sum, draft) => {
    const bp = normalizePercentInput(draft.weightRaw);
    return bp !== null && bp > 0 ? sum + bp : sum;
  }, 0);
  if (drafts.length < 2) {
    return { variants: null, weightsSum, error: 'count' };
  }
  const seen = new Set<string>();
  const variants: ExperimentVariantInput[] = [];
  let controls = 0;
  for (const draft of drafts) {
    const name = draft.name.trim();
    if (name.length === 0 || name.length > 128) {
      return { variants: null, weightsSum, error: 'name' };
    }
    if (seen.has(name)) {
      return { variants: null, weightsSum, error: 'duplicate' };
    }
    seen.add(name);
    const parsed = parseVariantValue(draft.valueRaw, flagType);
    if (!parsed.ok) {
      return { variants: null, weightsSum, error: 'value' };
    }
    const weight = parsePercentToBp(draft.weightRaw);
    if (weight === null) {
      return { variants: null, weightsSum, error: 'weight' };
    }
    if (draft.isControl) {
      controls += 1;
    }
    variants.push({ name, value: parsed.value, weight_bp: weight, is_control: draft.isControl });
  }
  if (controls !== 1) {
    return { variants: null, weightsSum, error: 'control' };
  }
  if (weightsSum !== weightsTotal) {
    return { variants: null, weightsSum, error: 'sum' };
  }
  return { variants, weightsSum, error: null };
}

export function parseTargetingInput(
  raw: string,
): { ok: true; targeting: string | null } | { ok: false } {
  const trimmed = raw.trim();
  if (trimmed.length === 0) {
    return { ok: true, targeting: null };
  }
  return isValidTargetingDsl(trimmed) ? { ok: true, targeting: trimmed } : { ok: false };
}

export function normalizeWeightsTotal(raw: string): number | null {
  const trimmed = raw.trim();
  if (trimmed.length === 0) {
    return DEFAULT_WEIGHTS_TOTAL;
  }
  const value = Number(trimmed);
  if (!Number.isInteger(value) || value < 1 || value > MAX_WEIGHTS_TOTAL) {
    return null;
  }
  return value;
}

// Allocation is stored as basis points (1..10000) but exchanged with the UI as
// percentages with two decimal places (0.01% .. 100.00%).

export function bpToPercent(bp: number): number {
  return bp / 100;
}

export function percentToBp(percent: number): number {
  return Math.round(percent * 100);
}

export function formatPercent(bp: number): string {
  return `${bpToPercent(bp).toFixed(2)}%`;
}

// normalizePercentInput converts a percent string into basis points. Blank
// input falls back to the default total (100%).
export function normalizePercentInput(raw: string): number | null {
  const trimmed = raw.trim();
  if (trimmed.length === 0) {
    return DEFAULT_WEIGHTS_TOTAL;
  }
  return parsePercentToBp(trimmed);
}

// parsePercentToBp converts a non-empty percent string into basis points,
// returning null for invalid or out-of-range input.
export function parsePercentToBp(raw: string): number | null {
  const trimmed = raw.trim();
  if (trimmed.length === 0) {
    return null;
  }
  const percent = Number(trimmed);
  if (!Number.isFinite(percent)) {
    return null;
  }
  const bp = percentToBp(percent);
  if (bp < 1 || bp > MAX_WEIGHTS_TOTAL) {
    return null;
  }
  return bp;
}

export function statusBadgeColor(status: ExperimentStatus): string {
  switch (status) {
    case 'draft':
      return 'gray';
    case 'review':
      return 'yellow';
    case 'approved':
      return 'cyan';
    case 'running':
      return 'green';
    case 'paused':
      return 'orange';
    case 'completed':
      return 'blue';
    case 'archived':
      return 'gray';
    case 'rejected':
      return 'red';
  }
}
