import { useMemo, useState } from 'react';
import { Box, Button, Text } from '@radix-ui/themes';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { List, type RowComponentProps } from 'react-window';

import { ApiError } from '../lib/api';
import { diffLines } from '../lib/diff';
import { getVersionDetail, listVersions, queryKeys, rollbackSecret } from '../lib/queries';
import type { ResolvedSecret, SecretVersionListEntry } from '../lib/types';
import { Alert } from './Alert';
import { ConfirmDialog } from './ConfirmDialog';
import { DiffViewer } from './DiffViewer';
import { Drawer } from './Drawer';
import { useToast } from './Toast';

/** Virtualize the version list past this many rows; below it the inline
 *  <ul> with a 4px gap reads better and skips react-window's mount cost.
 *  Mirrors DiffViewer's INLINE_THRESHOLD so the two surfaces share one
 *  policy for "when does this need windowing". */
const INLINE_THRESHOLD = 50;
/** Visible button is 56px (= --drawer-version-row-height); each row in
 *  react-window adds a 4px buffer below so virtualized rhythm matches
 *  the inline list's gap-4. */
const VIRTUALIZED_ROW_HEIGHT = 60;

/**
 * Per-key version timeline + diff viewer + rollback action.
 *
 * Lives inside a right-anchored drawer (380px) so the secret table behind
 * stays partially visible — the operator never loses the row that led
 * here. Layout is a single vertical stack: sticky header (key + close)
 * → version list (max 40vh) → diff region (fills remaining height) →
 * sticky footer (close / rollback).
 *
 * Version list comes from GET .../versions for the whole env (M2 read
 * endpoint) — filtered client-side per secret_id so two keys that share
 * a current version number don't collide.
 *
 * Selection model: at most one version selected at a time. Selecting
 * triggers GET .../versions/{v} on demand (cached by TanStack Query so
 * re-selecting is free). Diff renders against the current resolved
 * secret. Switching to a different row's "이력" button replaces the
 * drawer's contents in place (single-drawer model).
 */
interface VersionTimelinePanelProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  projectName: string;
  envName: string;
  secret: ResolvedSecret;
}

export function VersionTimelinePanel({
  open,
  onOpenChange,
  projectName,
  envName,
  secret,
}: VersionTimelinePanelProps) {
  const queryClient = useQueryClient();
  const toast = useToast();
  const [selected, setSelected] = useState<number | null>(null);
  const [confirmRollbackOpen, setConfirmRollbackOpen] = useState(false);

  const versionsQuery = useQuery({
    queryKey: queryKeys.versions(projectName, envName),
    queryFn: ({ signal }) => listVersions(projectName, envName, signal),
    enabled: open,
  });

  const filteredVersions = useMemo(() => {
    const all = versionsQuery.data ?? [];
    return all
      .filter((v) => v.secret_id === secret.secret_id)
      .sort((a, b) => b.version - a.version);
  }, [versionsQuery.data, secret.secret_id]);

  const detailQuery = useQuery({
    queryKey: queryKeys.versionDetail(projectName, envName, secret.key, selected ?? 0),
    queryFn: ({ signal }) => getVersionDetail(projectName, envName, secret.key, selected!, signal),
    enabled: open && selected !== null && selected !== secret.version,
  });

  const rollbackMutation = useMutation({
    mutationFn: (target: number) => rollbackSecret(projectName, envName, secret.key, target),
    onSuccess: async (updated) => {
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: queryKeys.secrets(projectName, envName) }),
        queryClient.invalidateQueries({ queryKey: queryKeys.versions(projectName, envName) }),
      ]);
      toast.notify(
        'success',
        `"${updated.key}" v${updated.version} 으로 롤백 (이전 v${secret.version})`,
      );
      setSelected(null);
      onOpenChange(false);
    },
    onError: (err: unknown) => {
      toast.notify('error', err instanceof ApiError ? err.message : '롤백에 실패했습니다.');
    },
  });

  const selectedDetail = detailQuery.data;
  const canDiff = selected !== null && selected !== secret.version;
  const canRollback = canDiff;

  // Diff is computed in the parent so the +N / −M summary can render in
  // the drawer-section-label-row alongside the "차이" eyebrow, instead of
  // duplicating inside DiffViewer's own header. diffLines is O(n*m) but
  // the inputs are short (a secret value), and useMemo keeps re-renders
  // cheap when only the selected version changes.
  const diffOps = useMemo(() => {
    if (!canDiff || !selectedDetail) return null;
    return diffLines(selectedDetail.value, secret.value);
  }, [canDiff, selectedDetail, secret.value]);

  const diffSummary = useMemo(() => {
    if (!diffOps) return null;
    let added = 0;
    let removed = 0;
    for (const op of diffOps) {
      if (op.kind === 'added') added += 1;
      else if (op.kind === 'removed') removed += 1;
    }
    return { added, removed };
  }, [diffOps]);

  return (
    <>
      <Drawer
        open={open}
        onOpenChange={(next) => {
          if (!next) setSelected(null);
          onOpenChange(next);
        }}
        ariaLabel={`${secret.key} 버전 이력`}
      >
        <Drawer.Header>
          <div className="drawer-title-row">
            <Drawer.Title asChild>
              <h2 className="drawer-title-key">{secret.key}</h2>
            </Drawer.Title>
            <span
              className="chip chip-mono chip-accent drawer-current-chip"
              aria-label={`현재 버전 ${secret.version}`}
            >
              현재 v{secret.version}
            </span>
          </div>
          <Drawer.Close asChild>
            <button type="button" className="icon-button" aria-label="닫기">
              <svg width="14" height="14" viewBox="0 0 14 14" fill="none" aria-hidden="true">
                <path
                  d="M3 3L11 11M11 3L3 11"
                  stroke="currentColor"
                  strokeWidth="1.5"
                  strokeLinecap="round"
                />
              </svg>
            </button>
          </Drawer.Close>
        </Drawer.Header>

        <Drawer.Description asChild>
          <span className="visually-hidden">
            버전 이력과 차이를 확인하고 이전 버전으로 롤백합니다.
          </span>
        </Drawer.Description>

        <Drawer.Body>
          {versionsQuery.error ? (
            <Box px="5" pt="3">
              <Alert
                variant="page"
                message={
                  '버전 이력을 불러오지 못했습니다.' +
                  (versionsQuery.error instanceof ApiError ? ` (${versionsQuery.error.code})` : '')
                }
              />
            </Box>
          ) : null}

          <div className="drawer-section-label">버전</div>
          {versionsQuery.isLoading ? (
            <div className="drawer-section-pad">
              <Text size="2" color="gray" role="status">
                불러오는 중…
              </Text>
            </div>
          ) : null}
          {filteredVersions.length === 0 && !versionsQuery.isLoading ? (
            <div className="drawer-section-pad">
              <Text size="2" color="gray">
                이력이 없습니다.
              </Text>
            </div>
          ) : null}
          {filteredVersions.length > INLINE_THRESHOLD ? (
            <div
              className="drawer-version-list-virtual"
              role="list"
              aria-label={`${secret.key} 버전 목록`}
            >
              <List
                rowCount={filteredVersions.length}
                rowHeight={VIRTUALIZED_ROW_HEIGHT}
                rowProps={{
                  versions: filteredVersions,
                  currentVersion: secret.version,
                  selectedVersion: selected,
                  onSelect: setSelected,
                }}
                rowComponent={VirtualizedVersionRow}
                defaultHeight={320}
                style={{ height: '100%' }}
              />
            </div>
          ) : (
            <ul className="drawer-version-list">
              {filteredVersions.map((v) => (
                <li key={v.id}>
                  <VersionRowButton
                    version={v}
                    isCurrent={v.version === secret.version}
                    isSelected={v.version === selected}
                    onSelect={setSelected}
                  />
                </li>
              ))}
            </ul>
          )}

          <div className="drawer-section-label-row">
            <span>차이</span>
            {diffSummary && selectedDetail ? (
              <span className="drawer-section-label-meta" aria-live="polite">
                +{diffSummary.added} / −{diffSummary.removed} · v{selectedDetail.version} → v
                {secret.version}
              </span>
            ) : null}
          </div>
          <div className="drawer-diff-region">
            {selected === null ? (
              <Text size="2" color="gray">
                위에서 비교할 버전을 골라 차이를 확인하고, 필요하면 그 버전의 ciphertext 로 새
                버전을 만들어 롤백합니다.
              </Text>
            ) : null}
            {selected === secret.version ? (
              <Text size="2" color="gray">
                선택한 버전이 현재 값과 동일합니다.
              </Text>
            ) : null}
            {canDiff && detailQuery.isLoading ? (
              <Text size="2" color="gray" role="status">
                값 불러오는 중…
              </Text>
            ) : null}
            {canDiff && detailQuery.error ? (
              <Alert
                variant="page"
                message={
                  '버전 값을 불러오지 못했습니다.' +
                  (detailQuery.error instanceof ApiError ? ` (${detailQuery.error.code})` : '')
                }
              />
            ) : null}
            {canDiff && selectedDetail && diffOps ? (
              <DiffViewer
                leftLabel={`v${selectedDetail.version}`}
                rightLabel={`v${secret.version} (현재)`}
                ops={diffOps}
              />
            ) : null}
          </div>
        </Drawer.Body>

        <Drawer.Footer>
          <Drawer.Close asChild>
            <Button variant="soft" color="gray">
              닫기
            </Button>
          </Drawer.Close>
          <Button
            disabled={!canRollback || rollbackMutation.isPending}
            onClick={() => setConfirmRollbackOpen(true)}
          >
            {rollbackMutation.isPending ? '롤백 중…' : '이 버전으로 롤백'}
          </Button>
        </Drawer.Footer>
      </Drawer>

      {selected !== null ? (
        <ConfirmDialog
          open={confirmRollbackOpen}
          onOpenChange={setConfirmRollbackOpen}
          title={`v${selected} 로 롤백`}
          description={
            <Text size="2">
              이전 ciphertext 로 새 버전이 만들어집니다. 현재 v{secret.version} 이후 새 v
              {secret.version + 1} 가 생성되며 감사 로그에 기록됩니다.
            </Text>
          }
          confirmLabel="롤백"
          intent="warning"
          onConfirm={async () => {
            await rollbackMutation.mutateAsync(selected);
          }}
        />
      ) : null}
    </>
  );
}

interface VersionRowButtonProps {
  version: SecretVersionListEntry;
  isCurrent: boolean;
  isSelected: boolean;
  onSelect: (version: number) => void;
  style?: React.CSSProperties;
}

/**
 * One row in the version list. Used inline (≤50 versions) and inside the
 * virtualized List (>50). Fixed height (.drawer-version-button) so
 * react-window doesn't need to measure rows. Custom button instead of
 * Radix's because Radix Button's height is fluid and the two-line layout
 * (v# + timestamp) needs a fixed slot.
 */
function VersionRowButton({
  version,
  isCurrent,
  isSelected,
  onSelect,
  style,
}: VersionRowButtonProps) {
  return (
    <button
      type="button"
      className="drawer-version-button"
      onClick={() => onSelect(version.version)}
      aria-pressed={isSelected}
      aria-label={`버전 ${version.version}${isCurrent ? ' (현재)' : ''} 선택`}
      style={style}
    >
      <span className="drawer-version-button-label">
        v{version.version}
        {isCurrent ? ' · 현재' : ''}
      </span>
      <span className="drawer-version-button-meta">
        {new Date(version.created_at).toLocaleString('ko-KR')}
      </span>
    </button>
  );
}

interface VirtualizedVersionRowProps {
  versions: SecretVersionListEntry[];
  currentVersion: number;
  selectedVersion: number | null;
  onSelect: (version: number) => void;
}

function VirtualizedVersionRow({
  index,
  style,
  versions,
  currentVersion,
  selectedVersion,
  onSelect,
  ariaAttributes,
}: RowComponentProps<VirtualizedVersionRowProps>) {
  const v = versions[index];
  // Buffer the bottom 4px inside the row wrapper so virtualized rhythm
  // matches the inline list's gap-4. The button itself stays at
  // --drawer-version-row-height.
  return (
    <div style={{ ...style, paddingBottom: 4 }} {...ariaAttributes}>
      <VersionRowButton
        version={v}
        isCurrent={v.version === currentVersion}
        isSelected={v.version === selectedVersion}
        onSelect={onSelect}
      />
    </div>
  );
}
