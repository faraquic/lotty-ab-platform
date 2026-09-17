import type { Flag, FlagDefaultValue, FlagType, UpdateFlagRequest } from '../types';

export function formatDefaultValue(value: FlagDefaultValue): string {
  if (typeof value === 'string') {
    return value;
  }
  if (typeof value === 'boolean') {
    return value ? 'true' : 'false';
  }
  return String(value);
}

export function parseDefaultValueInput(
  flagType: FlagType,
  raw: string,
): FlagDefaultValue | null {
  switch (flagType) {
    case 'string':
      return raw;
    case 'bool':
      if (raw === 'true') {
        return true;
      }
      if (raw === 'false') {
        return false;
      }
      return null;
    case 'number': {
      if (raw.trim().length === 0) {
        return null;
      }
      let parsed: unknown;
      try {
        parsed = JSON.parse(raw) as unknown;
      } catch {
        return null;
      }
      if (typeof parsed !== 'number' || !Number.isFinite(parsed)) {
        return null;
      }
      return parsed;
    }
  }
}

export function defaultValuesEqual(a: FlagDefaultValue, b: FlagDefaultValue): boolean {
  if (typeof a !== typeof b) {
    return false;
  }
  return a === b;
}

export interface EditableFlagFields {
  key: string;
  name: string;
  defaultRaw: string;
  description: string;
}

export function buildFlagUpdatePayload(
  original: Flag,
  values: EditableFlagFields,
): UpdateFlagRequest | null {
  const payload: UpdateFlagRequest = {};
  const key = values.key.trim();
  if (key.length > 0 && key !== original.key) {
    payload.key = key;
  }
  const name = values.name.trim();
  if (name.length > 0 && name !== original.name) {
    payload.name = name;
  }
  const parsedDefault = parseDefaultValueInput(original.type, values.defaultRaw);
  if (parsedDefault !== null && !defaultValuesEqual(parsedDefault, original.default_value)) {
    payload.default_value = parsedDefault;
  }
  const description = values.description.trim();
  if (description.length > 0 && description !== (original.description ?? '')) {
    payload.description = description;
  }
  if (
    payload.key === undefined &&
    payload.name === undefined &&
    payload.default_value === undefined &&
    payload.description === undefined
  ) {
    return null;
  }
  return payload;
}
