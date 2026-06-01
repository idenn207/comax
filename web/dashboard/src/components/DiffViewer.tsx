import { type CSSProperties } from 'react';
import { List, type RowComponentProps } from 'react-window';

import type { DiffOp } from '../lib/diff';

/**
 * Side-by-side diff viewer. left = older version (target), right =
 * current. Removed lines highlight on the left, added on the right.
 * Equal lines render in both columns so context is visible.
 *
 * The parent owns the diff computation (`diffLines(left, right)`) and
 * the summary line ("+N / −M"). DiffViewer only renders the frame, so
 * the calling surface (drawer / page) can place the summary wherever its
 * hierarchy needs (drawer-section-label-row in VersionTimelinePanel).
 *
 * Rendering modes (lazy virtualization):
 *   - 50 ops or fewer → inline render. Cheap, no react-window mount cost,
 *     no row-height quirks. Covers the typical secret value (a few KB).
 *   - more than 50 ops → react-window v2 List. Necessary for 1 MB+
 *     ciphertexts where mounting tens of thousands of rows would freeze
 *     the drawer.
 *
 * Both modes lock row height to --diff-row-height (whiteSpace: pre,
 * per-cell overflow-x: auto). The cost is that a long URL inside one
 * diff line scrolls horizontally inside its cell; the benefit is constant
 * row height so virtualization is trivial and the inline-to-virtualized
 * cutover is visually seamless.
 *
 * Visual treatment lives in globals.css (.diff-frame / .diff-row /
 * .diff-cell / .diff-gutter) rather than inline style here, so font,
 * row height, and border tokens propagate from the design system. The
 * earlier inline `fontFamily: 'ui-monospace, ...'` bypassed
 * --font-mono — a quiet drift caught by /impeccable critique.
 *
 * Accessibility: added/removed status is conveyed by both color AND a
 * visually-hidden Korean label per row, so color-blind users and screen
 * reader users get the same signal as sighted users (WCAG 1.4.1). In
 * virtualized mode react-window also supplies aria-posinset / aria-setsize
 * so screen readers know the position within the full set, not just
 * within the rendered window.
 */

interface DiffViewerProps {
  leftLabel: string;
  rightLabel: string;
  ops: DiffOp[];
}

const INLINE_THRESHOLD = 50;
const VIRTUALIZED_DEFAULT_HEIGHT = 320;

export function DiffViewer({ leftLabel, rightLabel, ops }: DiffViewerProps) {
  const useVirtualization = ops.length > INLINE_THRESHOLD;

  return (
    <div
      className="diff-frame"
      role="table"
      aria-label={`${leftLabel} 대비 ${rightLabel} 차이`}
      aria-rowcount={ops.length}
    >
      {ops.length === 0 ? (
        <div className="diff-empty" role="row">
          두 버전이 동일합니다.
        </div>
      ) : useVirtualization ? (
        <List
          rowCount={ops.length}
          rowHeight={getRowHeightPx()}
          rowProps={{ ops }}
          rowComponent={VirtualizedDiffRow}
          defaultHeight={VIRTUALIZED_DEFAULT_HEIGHT}
          style={{ height: '100%' }}
        />
      ) : (
        <div className="diff-frame-inline">
          {ops.map((op, idx) => (
            <DiffRow key={`${idx}-${op.kind}`} op={op} />
          ))}
        </div>
      )}
    </div>
  );
}

interface DiffRowProps {
  op: DiffOp;
  style?: CSSProperties;
}

function DiffRow({ op, style }: DiffRowProps) {
  if (op.kind === 'equal') {
    return (
      <div className="diff-row" role="row" style={style}>
        <Gutter line={op.leftLine} />
        <Cell text={op.left} />
        <Gutter line={op.rightLine} />
        <Cell text={op.right} />
      </div>
    );
  }
  if (op.kind === 'removed') {
    return (
      <div className="diff-row diff-row-removed" role="row" style={style}>
        <span className="visually-hidden">제거된 줄: </span>
        <Gutter line={op.leftLine} />
        <Cell prefix="−" text={op.left} variant="removed" />
        <Gutter />
        <Cell text="" />
      </div>
    );
  }
  return (
    <div className="diff-row diff-row-added" role="row" style={style}>
      <span className="visually-hidden">추가된 줄: </span>
      <Gutter />
      <Cell text="" />
      <Gutter line={op.rightLine} />
      <Cell prefix="+" text={op.right} variant="added" />
    </div>
  );
}

function VirtualizedDiffRow({
  index,
  style,
  ops,
  ariaAttributes,
}: RowComponentProps<{ ops: DiffOp[] }>) {
  const op = ops[index];
  return (
    <div style={style} {...ariaAttributes}>
      <DiffRow op={op} />
    </div>
  );
}

function Gutter({ line }: { line?: number }) {
  return (
    <div className="diff-gutter" role="cell">
      {line ?? ''}
    </div>
  );
}

function Cell({
  text,
  prefix,
  variant,
}: {
  text: string;
  prefix?: string;
  variant?: 'removed' | 'added';
}) {
  const variantClass =
    variant === 'removed' ? 'diff-cell-removed' : variant === 'added' ? 'diff-cell-added' : '';
  return (
    <div className={`diff-cell ${variantClass}`.trim()} role="cell">
      {prefix ? <span aria-hidden="true">{prefix} </span> : null}
      {text}
    </div>
  );
}

/**
 * react-window v2 requires a pixel rowHeight at mount time. We resolve
 * --diff-row-height from the document root once per render; the token
 * is the source of truth (tokens.css), and tests / SSR fall back to a
 * sensible default if the CSS hasn't loaded yet.
 */
function getRowHeightPx(): number {
  if (typeof window === 'undefined') return 24;
  const raw = getComputedStyle(document.documentElement)
    .getPropertyValue('--diff-row-height')
    .trim();
  const parsed = parseInt(raw, 10);
  return Number.isFinite(parsed) && parsed > 0 ? parsed : 24;
}
