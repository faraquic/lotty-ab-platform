import { describe, expect, it } from 'vitest';
import {
  formatPercent,
  lifecycleActionsFor,
  normalizePercentInput,
  normalizeWeightsTotal,
  parseTargetingInput,
  parseVariantValue,
  statusBadgeColor,
  validateVariantDrafts,
} from './transitions';

describe('experiment lifecycle matrix', () => {
  it('exposes submit only for drafts', () => {
    expect(lifecycleActionsFor('draft')).toEqual(['submit']);
  });

  it('exposes start only for approved experiments', () => {
    expect(lifecycleActionsFor('approved')).toEqual(['start']);
  });

  it('exposes pause for running and resume for paused', () => {
    expect(lifecycleActionsFor('running')).toEqual(['pause']);
    expect(lifecycleActionsFor('paused')).toEqual(['resume']);
  });

  it('exposes archive for completed experiments', () => {
    expect(lifecycleActionsFor('completed')).toEqual(['archive']);
  });

  it('exposes no direct transitions for review, archived and rejected', () => {
    expect(lifecycleActionsFor('review')).toEqual([]);
    expect(lifecycleActionsFor('archived')).toEqual([]);
    expect(lifecycleActionsFor('rejected')).toEqual([]);
  });

  it('maps every status to a badge color', () => {
    const statuses = [
      'draft',
      'review',
      'approved',
      'running',
      'paused',
      'completed',
      'archived',
      'rejected',
    ] as const;
    for (const status of statuses) {
      expect(statusBadgeColor(status).length).toBeGreaterThan(0);
    }
  });
});

describe('variant draft validation', () => {
  it('accepts two balanced variants with exactly one control', () => {
    const result = validateVariantDrafts(
      [
        { name: 'control', valueRaw: 'false', weightRaw: '50', isControl: true },
        { name: 'treatment', valueRaw: 'true', weightRaw: '50', isControl: false },
      ],
      10000,
      'bool',
    );

    expect(result.error).toBeNull();
    expect(result.variants).toEqual([
      { name: 'control', value: false, weight_bp: 5000, is_control: true },
      { name: 'treatment', value: true, weight_bp: 5000, is_control: false },
    ]);
  });

  it('rejects fewer than two variants', () => {
    const result = validateVariantDrafts(
      [{ name: 'control', valueRaw: '1', weightRaw: '100', isControl: true }],
      10000,
      'number',
    );
    expect(result.error).toBe('count');
  });

  it('rejects missing control, duplicate controls, and weight mismatches', () => {
    const noControl = validateVariantDrafts(
      [
        { name: 'a', valueRaw: '1', weightRaw: '50', isControl: false },
        { name: 'b', valueRaw: '2', weightRaw: '50', isControl: false },
      ],
      10000,
      'number',
    );
    expect(noControl.error).toBe('control');

    const twoControls = validateVariantDrafts(
      [
        { name: 'a', valueRaw: '1', weightRaw: '50', isControl: true },
        { name: 'b', valueRaw: '2', weightRaw: '50', isControl: true },
      ],
      10000,
      'number',
    );
    expect(twoControls.error).toBe('control');

    const badSum = validateVariantDrafts(
      [
        { name: 'a', valueRaw: '1', weightRaw: '40', isControl: true },
        { name: 'b', valueRaw: '2', weightRaw: '50', isControl: false },
      ],
      10000,
      'number',
    );
    expect(badSum.error).toBe('sum');
  });

  it('rejects duplicate names, invalid values, and bad weights', () => {
    const duplicate = validateVariantDrafts(
      [
        { name: 'a', valueRaw: '1', weightRaw: '50', isControl: true },
        { name: 'a', valueRaw: '2', weightRaw: '50', isControl: false },
      ],
      10000,
      'number',
    );
    expect(duplicate.error).toBe('duplicate');

    const badValue = validateVariantDrafts(
      [
        { name: 'a', valueRaw: 'not a number', weightRaw: '50', isControl: true },
        { name: 'b', valueRaw: '2', weightRaw: '50', isControl: false },
      ],
      10000,
      'number',
    );
    expect(badValue.error).toBe('value');

    const badWeight = validateVariantDrafts(
      [
        { name: 'a', valueRaw: '1', weightRaw: '0', isControl: true },
        { name: 'b', valueRaw: '2', weightRaw: '50', isControl: false },
      ],
      10000,
      'number',
    );
    expect(badWeight.error).toBe('weight');
  });

  it('rejects values that do not match the flag type', () => {
    const result = validateVariantDrafts(
      [
        { name: 'control', valueRaw: '1', weightRaw: '50', isControl: true },
        { name: 'treatment', valueRaw: 'yes', weightRaw: '50', isControl: false },
      ],
      10000,
      'number',
    );
    expect(result.error).toBe('value');
  });
});

describe('variant value and targeting parsing', () => {
  it('parses variant values according to the flag type', () => {
    expect(parseVariantValue('false', 'bool')).toEqual({ ok: true, value: false });
    expect(parseVariantValue('42', 'number')).toEqual({ ok: true, value: 42 });
    expect(parseVariantValue('pro', 'string')).toEqual({ ok: true, value: 'pro' });
    expect(parseVariantValue('  ', 'string')).toEqual({ ok: false });
    expect(parseVariantValue('not-a-number', 'number')).toEqual({ ok: false });
    expect(parseVariantValue('maybe', 'bool')).toEqual({ ok: false });
  });

  it('treats empty targeting as no targeting and rejects invalid DSL', () => {
    expect(parseTargetingInput('')).toEqual({ ok: true, targeting: null });
    expect(parseTargetingInput('country == "DE"')).toEqual({
      ok: true,
      targeting: 'country == "DE"',
    });
    expect(parseTargetingInput('country ~ "DE"')).toEqual({ ok: false });
    expect(parseTargetingInput('country ==')).toEqual({ ok: false });
  });

  it('normalizes percent allocation to basis points', () => {
    expect(normalizePercentInput('')).toBe(10000);
    expect(normalizePercentInput('50')).toBe(5000);
    expect(normalizePercentInput('0')).toBeNull();
    expect(normalizePercentInput('100.01')).toBeNull();
    expect(normalizePercentInput('nope')).toBeNull();
  });

  it('formats basis points as percentages with two decimals', () => {
    expect(formatPercent(5000)).toBe('50.00%');
    expect(formatPercent(3333)).toBe('33.33%');
  });

  it('normalizes weights total with the backend default', () => {
    expect(normalizeWeightsTotal('')).toBe(10000);
    expect(normalizeWeightsTotal('5000')).toBe(5000);
    expect(normalizeWeightsTotal('0')).toBeNull();
    expect(normalizeWeightsTotal('10001')).toBeNull();
    expect(normalizeWeightsTotal('nope')).toBeNull();
  });
});
