import { describe, expect, it } from 'vitest';
import { isValidTargetingDsl, parseTargetingDsl } from './targeting';

describe('targeting DSL parser', () => {
  it('accepts expressions from the grammar', () => {
    const valid = [
      'country == "DE"',
      'country != "US" AND plan IN ["free", "pro"]',
      'NOT (age >= 18)',
      'a == 1 OR b == 2 AND NOT c == "x"',
      'plan NOT IN ["free"]',
      'score > 10 AND score <= 20',
      "a == 'single'",
      'flag == true',
    ];
    for (const expression of valid) {
      expect(isValidTargetingDsl(expression), expression).toBe(true);
    }
  });

  it('treats blank input as valid (no targeting)', () => {
    expect(isValidTargetingDsl('')).toBe(true);
    expect(isValidTargetingDsl('   ')).toBe(true);
  });

  it('rejects invalid expressions', () => {
    const invalid = [
      'country =',
      'country',
      '== 1',
      '(a == 1',
      'a IN',
      'a IN [',
      'a IN [1,',
      'a ~ 1',
      'a == 1 AND',
    ];
    for (const expression of invalid) {
      expect(isValidTargetingDsl(expression), expression).toBe(false);
    }
  });

  it('applies OR lower precedence than AND', () => {
    const node = parseTargetingDsl('a == 1 OR b == 2 AND c == 3');
    expect(node.type).toBe('or');
    if (node.type === 'or') {
      expect(node.right.type).toBe('and');
    }
  });
});
