import { describe, expect, it } from 'vitest';
import {
  lifecycleActionsFor,
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
        { name: 'control', valueRaw: 'false', weightRaw: '5000', isControl: true },
        { name: 'treatment', valueRaw: 'true', weightRaw: '5000', isControl: false },
      ],
      10000,
    );

    expect(result.error).toBeNull();
    expect(result.variants).toEqual([
      { name: 'control', value: false, weight_bp: 5000, is_control: true },
      { name: 'treatment', value: true, weight_bp: 5000, is_control: false },
    ]);
  });

  it('rejects fewer than two variants', () => {
    const result = validateVariantDrafts(
      [{ name: 'control', valueRaw: '1', weightRaw: '10000', isControl: true }],
      10000,
    );
    expect(result.error).toBe('count');
  });

  it('rejects missing control, duplicate controls, and weight mismatches', () => {
    const noControl = validateVariantDrafts(
      [
        { name: 'a', valueRaw: '1', weightRaw: '5000', isControl: false },
        { name: 'b', valueRaw: '2', weightRaw: '5000', isControl: false },
      ],
      10000,
    );
    expect(noControl.error).toBe('control');

    const twoControls = validateVariantDrafts(
      [
        { name: 'a', valueRaw: '1', weightRaw: '5000', isControl: true },
        { name: 'b', valueRaw: '2', weightRaw: '5000', isControl: true },
      ],
      10000,
    );
    expect(twoControls.error).toBe('control');

    const badSum = validateVariantDrafts(
      [
        { name: 'a', valueRaw: '1', weightRaw: '4000', isControl: true },
        { name: 'b', valueRaw: '2', weightRaw: '5000', isControl: false },
      ],
      10000,
    );
    expect(badSum.error).toBe('sum');
  });

  it('rejects duplicate names, invalid JSON values, and bad weights', () => {
    const duplicate = validateVariantDrafts(
      [
        { name: 'a', valueRaw: '1', weightRaw: '5000', isControl: true },
        { name: 'a', valueRaw: '2', weightRaw: '5000', isControl: false },
      ],
      10000,
    );
    expect(duplicate.error).toBe('duplicate');

    const badValue = validateVariantDrafts(
      [
        { name: 'a', valueRaw: 'not json', weightRaw: '5000', isControl: true },
        { name: 'b', valueRaw: '2', weightRaw: '5000', isControl: false },
      ],
      10000,
    );
    expect(badValue.error).toBe('value');

    const badWeight = validateVariantDrafts(
      [
        { name: 'a', valueRaw: '1', weightRaw: '0', isControl: true },
        { name: 'b', valueRaw: '2', weightRaw: '5000', isControl: false },
      ],
      10000,
    );
    expect(badWeight.error).toBe('weight');
  });
});

describe('variant value and targeting parsing', () => {
  it('parses any valid JSON variant value', () => {
    expect(parseVariantValue('false')).toEqual({ ok: true, value: false });
    expect(parseVariantValue('{"plan":"pro"}')).toEqual({ ok: true, value: { plan: 'pro' } });
    expect(parseVariantValue('  ')).toEqual({ ok: false });
    expect(parseVariantValue('{broken')).toEqual({ ok: false });
  });

  it('treats empty targeting as no targeting and rejects non-objects', () => {
    expect(parseTargetingInput('')).toEqual({ ok: true, targeting: null });
    expect(parseTargetingInput('{"country":"DE"}')).toEqual({
      ok: true,
      targeting: { country: 'DE' },
    });
    expect(parseTargetingInput('[1,2]')).toEqual({ ok: false });
    expect(parseTargetingInput('{broken')).toEqual({ ok: false });
  });

  it('normalizes weights total with the backend default', () => {
    expect(normalizeWeightsTotal('')).toBe(10000);
    expect(normalizeWeightsTotal('5000')).toBe(5000);
    expect(normalizeWeightsTotal('0')).toBeNull();
    expect(normalizeWeightsTotal('10001')).toBeNull();
    expect(normalizeWeightsTotal('nope')).toBeNull();
  });
});
