import { describe, expect, it } from 'vitest';
import {
  buildFlagUpdatePayload,
  defaultValuesEqual,
  formatDefaultValue,
  parseDefaultValueInput,
} from './flagValue';
import type { Flag } from '../types';

function makeFlag(overrides: Partial<Flag> = {}): Flag {
  return {
    id: 'fid-1',
    key: 'checkout_redesign',
    name: 'Checkout redesign',
    type: 'bool',
    default_value: false,
    description: null,
    created_by: null,
    updated_by: null,
    created_at: '2026-09-17T07:55:24.211Z',
    updated_at: '2026-09-17T07:55:24.211Z',
    ...overrides,
  };
}

describe('flag default values', () => {
  it('formats typed defaults for display', () => {
    expect(formatDefaultValue('hello')).toBe('hello');
    expect(formatDefaultValue(99.5)).toBe('99.5');
    expect(formatDefaultValue(true)).toBe('true');
    expect(formatDefaultValue(false)).toBe('false');
  });

  it('parses raw input per flag type', () => {
    expect(parseDefaultValueInput('string', 'anything')).toBe('anything');
    expect(parseDefaultValueInput('string', '')).toBe('');
    expect(parseDefaultValueInput('bool', 'true')).toBe(true);
    expect(parseDefaultValueInput('bool', 'false')).toBe(false);
    expect(parseDefaultValueInput('bool', '')).toBeNull();
    expect(parseDefaultValueInput('number', '1.5')).toBe(1.5);
    expect(parseDefaultValueInput('number', '0')).toBe(0);
    expect(parseDefaultValueInput('number', '-3')).toBe(-3);
    expect(parseDefaultValueInput('number', '')).toBeNull();
    expect(parseDefaultValueInput('number', 'abc')).toBeNull();
    expect(parseDefaultValueInput('number', '0x10')).toBeNull();
    expect(parseDefaultValueInput('number', 'NaN')).toBeNull();
    expect(parseDefaultValueInput('number', '1e999')).toBeNull();
  });

  it('compares typed defaults by value and type', () => {
    expect(defaultValuesEqual(false, false)).toBe(true);
    expect(defaultValuesEqual(1, 1)).toBe(true);
    expect(defaultValuesEqual('a', 'a')).toBe(true);
    expect(defaultValuesEqual(1, '1')).toBe(false);
    expect(defaultValuesEqual(true, false)).toBe(false);
  });

  it('builds an update payload with only changed fields', () => {
    const original = makeFlag();

    expect(
      buildFlagUpdatePayload(original, {
        key: original.key,
        name: original.name,
        defaultRaw: 'false',
        description: '',
      }),
    ).toBeNull();

    expect(
      buildFlagUpdatePayload(original, {
        key: original.key,
        name: 'New name',
        defaultRaw: 'false',
        description: '',
      }),
    ).toEqual({ name: 'New name' });

    expect(
      buildFlagUpdatePayload(original, {
        key: original.key,
        name: original.name,
        defaultRaw: 'true',
        description: '',
      }),
    ).toEqual({ default_value: true });

    expect(
      buildFlagUpdatePayload(original, {
        key: 'new_key',
        name: original.name,
        defaultRaw: 'false',
        description: 'Same-site checkout',
      }),
    ).toEqual({ key: 'new_key', description: 'Same-site checkout' });
  });

  it('never sends empty key/name/description', () => {
    const original = makeFlag({ description: 'Keep me' });

    expect(
      buildFlagUpdatePayload(original, {
        key: '  ',
        name: '',
        defaultRaw: 'false',
        description: '',
      }),
    ).toBeNull();
  });
});
