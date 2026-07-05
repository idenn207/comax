/**
 * Tiny line-level diff for the version diff viewer.
 *
 * Algorithm: longest common subsequence (LCS) on two arrays of lines,
 * then walk the LCS table back to emit a sequence of ops. O(n*m) time
 * and memory — fine because secrets are short and we only run this when
 * the operator clicks "View diff".
 *
 * We do not pull in `diff` or `jsdiff`: this stays lean for the binary
 * budget (Task 13) and the algorithm is small enough to read in one
 * pass. If a real word/character diff is needed later, swap to a proper
 * library at that point — premature dependency adds 10kB+ for a feature
 * the operator triggers maybe once a week.
 */

export type DiffOp =
  | { kind: 'equal'; left: string; right: string; leftLine: number; rightLine: number }
  | { kind: 'removed'; left: string; leftLine: number }
  | { kind: 'added'; right: string; rightLine: number };

export function diffLines(left: string, right: string): DiffOp[] {
  const leftLines = left === '' ? [] : left.split('\n');
  const rightLines = right === '' ? [] : right.split('\n');

  const n = leftLines.length;
  const m = rightLines.length;

  // lcs[i][j] = LCS length of leftLines[0..i) and rightLines[0..j)
  const lcs: number[][] = Array.from({ length: n + 1 }, () => new Array(m + 1).fill(0));
  for (let i = 1; i <= n; i++) {
    for (let j = 1; j <= m; j++) {
      if (leftLines[i - 1] === rightLines[j - 1]) {
        lcs[i][j] = lcs[i - 1][j - 1] + 1;
      } else {
        lcs[i][j] = Math.max(lcs[i - 1][j], lcs[i][j - 1]);
      }
    }
  }

  const ops: DiffOp[] = [];
  let i = n;
  let j = m;
  while (i > 0 || j > 0) {
    if (i > 0 && j > 0 && leftLines[i - 1] === rightLines[j - 1]) {
      ops.push({
        kind: 'equal',
        left: leftLines[i - 1],
        right: rightLines[j - 1],
        leftLine: i,
        rightLine: j,
      });
      i -= 1;
      j -= 1;
    } else if (j > 0 && (i === 0 || lcs[i][j - 1] >= lcs[i - 1][j])) {
      ops.push({ kind: 'added', right: rightLines[j - 1], rightLine: j });
      j -= 1;
    } else {
      ops.push({ kind: 'removed', left: leftLines[i - 1], leftLine: i });
      i -= 1;
    }
  }

  return ops.reverse();
}
