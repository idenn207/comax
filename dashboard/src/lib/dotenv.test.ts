import { describe, expect, it } from 'vitest';

import { formatDotenv } from './dotenv';

describe('formatDotenv', () => {
  it('returns empty string for empty input', () => {
    expect(formatDotenv([])).toBe('');
  });

  it('emits simple KEY=VALUE pairs with trailing newline', () => {
    const out = formatDotenv([{ key: 'PORT', value: '8080' }]);
    expect(out).toBe('PORT=8080\n');
  });

  it('sorts keys alphabetically for deterministic clipboard output', () => {
    const out = formatDotenv([
      { key: 'B', value: '2' },
      { key: 'A', value: '1' },
    ]);
    expect(out).toBe('A=1\nB=2\n');
  });

  it('quotes values containing whitespace', () => {
    const out = formatDotenv([{ key: 'GREETING', value: 'hello world' }]);
    expect(out).toBe('GREETING="hello world"\n');
  });

  it('escapes backslash and double-quote inside quoted values', () => {
    const out = formatDotenv([{ key: 'X', value: 'a"b\\c' }]);
    expect(out).toBe('X="a\\"b\\\\c"\n');
  });

  it('escapes newlines in quoted values', () => {
    const out = formatDotenv([{ key: 'PEM', value: 'line1\nline2' }]);
    expect(out).toBe('PEM="line1\\nline2"\n');
  });

  it('keeps empty values as bare =', () => {
    const out = formatDotenv([{ key: 'EMPTY', value: '' }]);
    expect(out).toBe('EMPTY=\n');
  });
});
