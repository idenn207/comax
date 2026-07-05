import { describe, expect, it } from 'vitest';

import { diffLines } from './diff';

describe('diffLines', () => {
  it('returns empty ops for two empty strings', () => {
    expect(diffLines('', '')).toEqual([]);
  });

  it('flags every line removed when right is empty', () => {
    const ops = diffLines('a\nb', '');
    expect(ops).toEqual([
      { kind: 'removed', left: 'a', leftLine: 1 },
      { kind: 'removed', left: 'b', leftLine: 2 },
    ]);
  });

  it('flags every line added when left is empty', () => {
    const ops = diffLines('', 'a\nb');
    expect(ops).toEqual([
      { kind: 'added', right: 'a', rightLine: 1 },
      { kind: 'added', right: 'b', rightLine: 2 },
    ]);
  });

  it('marks equal lines on both sides', () => {
    const ops = diffLines('a\nb', 'a\nb');
    expect(ops).toEqual([
      { kind: 'equal', left: 'a', right: 'a', leftLine: 1, rightLine: 1 },
      { kind: 'equal', left: 'b', right: 'b', leftLine: 2, rightLine: 2 },
    ]);
  });

  it('detects a single changed line via removed+added pair', () => {
    const ops = diffLines('a\nold\nc', 'a\nnew\nc');
    const kinds = ops.map((o) => o.kind);
    expect(kinds).toContain('removed');
    expect(kinds).toContain('added');
    expect(kinds.filter((k) => k === 'equal').length).toBe(2);
  });
});
