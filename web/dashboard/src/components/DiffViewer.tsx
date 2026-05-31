import { useMemo } from 'react';
import { Box, Flex, Text } from '@radix-ui/themes';

import { diffLines } from '../lib/diff';

/**
 * Side-by-side diff viewer. left = older version (target), right =
 * current. Removed lines highlight on the left, added on the right.
 * Equal lines render in both columns so context is visible.
 *
 * We render as a single <table role="table"> so screen readers narrate
 * "row 3, removed: X" coherently. The visual style uses semantic colors
 * from tokens.css (--color-danger-soft / --color-success-soft for row
 * backgrounds, --color-danger / --color-success for foreground text)
 * rather than raw Radix --red-a3 etc., so light and dark themes stay
 * perceptually flat.
 *
 * Accessibility: added/removed status is conveyed by both color AND a
 * visually-hidden Korean label per row, so color-blind users and screen
 * reader users get the same signal as sighted users (WCAG 1.4.1).
 */

interface DiffViewerProps {
  leftLabel: string;
  rightLabel: string;
  left: string;
  right: string;
}

export function DiffViewer({ leftLabel, rightLabel, left, right }: DiffViewerProps) {
  const ops = useMemo(() => diffLines(left, right), [left, right]);
  const summary = useMemo(() => {
    let added = 0;
    let removed = 0;
    for (const op of ops) {
      if (op.kind === 'added') added += 1;
      else if (op.kind === 'removed') removed += 1;
    }
    return { added, removed };
  }, [ops]);

  return (
    <Flex direction="column" gap="2">
      <Flex justify="between" align="center">
        <Text size="2" color="gray">
          {leftLabel} → {rightLabel}
        </Text>
        <Text size="1" color="gray" aria-live="polite">
          +{summary.added} / −{summary.removed}
        </Text>
      </Flex>
      <Box
        role="table"
        aria-label={`${leftLabel} 대비 ${rightLabel} 차이`}
        style={{
          border: '1px solid var(--color-border)',
          borderRadius: 'var(--radius-md)',
          overflow: 'hidden',
          fontFamily: 'ui-monospace, SFMono-Regular, Menlo, monospace',
          fontSize: '0.85rem',
        }}
      >
        {ops.length === 0 ? (
          <Box p="3" role="row">
            <Text size="2" color="gray">
              두 버전이 동일합니다.
            </Text>
          </Box>
        ) : (
          ops.map((op, idx) => <DiffRow key={`${idx}-${op.kind}`} op={op} />)
        )}
      </Box>
    </Flex>
  );
}

interface DiffRowProps {
  op: ReturnType<typeof diffLines>[number];
}

function DiffRow({ op }: DiffRowProps) {
  if (op.kind === 'equal') {
    return (
      <Box
        role="row"
        style={{
          display: 'grid',
          gridTemplateColumns: 'min-content 1fr min-content 1fr',
          borderTop: '1px solid var(--color-border)',
        }}
      >
        <Gutter line={op.leftLine} />
        <Cell text={op.left} />
        <Gutter line={op.rightLine} />
        <Cell text={op.right} />
      </Box>
    );
  }
  if (op.kind === 'removed') {
    return (
      <Box
        role="row"
        style={{
          display: 'grid',
          gridTemplateColumns: 'min-content 1fr min-content 1fr',
          borderTop: '1px solid var(--color-border)',
          background: 'var(--color-danger-soft)',
        }}
      >
        <span className="visually-hidden">제거된 줄: </span>
        <Gutter line={op.leftLine} />
        <Cell prefix="−" text={op.left} color="var(--color-danger)" />
        <Gutter />
        <Cell text="" />
      </Box>
    );
  }
  return (
    <Box
      role="row"
      style={{
        display: 'grid',
        gridTemplateColumns: 'min-content 1fr min-content 1fr',
        borderTop: '1px solid var(--color-border)',
        background: 'var(--color-success-soft)',
      }}
    >
      <span className="visually-hidden">추가된 줄: </span>
      <Gutter />
      <Cell text="" />
      <Gutter line={op.rightLine} />
      <Cell prefix="+" text={op.right} color="var(--color-success)" />
    </Box>
  );
}

function Gutter({ line }: { line?: number }) {
  return (
    <Box
      role="cell"
      style={{
        minWidth: '2.5rem',
        padding: '2px 8px',
        textAlign: 'right',
        color: 'var(--color-muted)',
        background: 'var(--color-surface-hover)',
        userSelect: 'none',
      }}
    >
      {line ?? ''}
    </Box>
  );
}

function Cell({ text, prefix, color }: { text: string; prefix?: string; color?: string }) {
  return (
    <Box
      role="cell"
      style={{
        padding: '2px 10px',
        whiteSpace: 'pre-wrap',
        wordBreak: 'break-all',
        color,
      }}
    >
      {prefix ? <span aria-hidden="true">{prefix} </span> : null}
      {text}
    </Box>
  );
}
