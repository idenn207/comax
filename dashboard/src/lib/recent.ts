import { useEffect, useState } from 'react';

/**
 * Recently visited (project, env) destinations, surfaced by the command
 * palette in its empty-input state.
 *
 * Why in-memory FIFO (not localStorage):
 *   DESIGN.md forbids browser-only persistence so the same shell can
 *   port to a desktop app without inheriting web-storage quirks. The
 *   palette already opens on every session; "recent within this tab"
 *   is the right scope — across-session recall would need a server
 *   endpoint, which is backlog 8 (cross-env key index).
 *
 * The store is a module-level array plus a Set of subscribers. useRecent
 * snapshots into React state and re-subscribes on mount; pushRecent is
 * safe to call from a router subscription outside the React tree.
 */

export interface RecentEntry {
  project: string;
  env?: string;
}

const MAX_RECENT = 5;
let entries: RecentEntry[] = [];
const listeners = new Set<() => void>();

function emit(): void {
  for (const listener of listeners) listener();
}

function sameEntry(a: RecentEntry, b: RecentEntry): boolean {
  return a.project === b.project && a.env === b.env;
}

export function pushRecent(entry: RecentEntry): void {
  const top = entries[0];
  if (top && sameEntry(top, entry)) return;
  const filtered = entries.filter((e) => !sameEntry(e, entry));
  entries = [entry, ...filtered].slice(0, MAX_RECENT);
  emit();
}

export function getRecent(): readonly RecentEntry[] {
  return entries;
}

export function clearRecent(): void {
  if (entries.length === 0) return;
  entries = [];
  emit();
}

export function useRecent(): readonly RecentEntry[] {
  const [snapshot, setSnapshot] = useState<readonly RecentEntry[]>(entries);
  useEffect(() => {
    function onChange() {
      setSnapshot([...entries]);
    }
    listeners.add(onChange);
    // Sync once on mount in case entries shifted between render and effect.
    onChange();
    return () => {
      listeners.delete(onChange);
    };
  }, []);
  return snapshot;
}
