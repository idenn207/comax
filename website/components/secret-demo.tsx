'use client';

import { useMemo, useState } from 'react';
import { AlertTriangle, Database, Eye, EyeOff } from 'lucide-react';
import { cn } from '@/lib/cn';

/**
 * A live miniature of the operator dashboard, used as the landing's real
 * "product imagery" (not a stock photo). Switch environments and reveal values;
 * a key that exists elsewhere but is missing here surfaces as a first-class
 * signal (dedicated "빠짐" badge + banner), which is the whole reason the
 * product exists. Mirrors the shipped dashboard's masking + drift behaviour.
 */

type EnvId = 'dev' | 'staging' | 'prod';

const ENVS: { id: EnvId; label: string; mono: string }[] = [
  { id: 'dev', label: '개발', mono: 'dev' },
  { id: 'staging', label: '스테이징', mono: 'staging' },
  { id: 'prod', label: '프로덕션', mono: 'prod' },
];

const KEYS = ['DATABASE_URL', 'STRIPE_SECRET', 'REDIS_URL', 'SMTP_PASSWORD'] as const;

const VALUES: Record<EnvId, Record<(typeof KEYS)[number], string | null>> = {
  dev: {
    DATABASE_URL: 'postgres://localhost:5432/app',
    STRIPE_SECRET: 'sk_test_4eC39HqLy...',
    REDIS_URL: 'redis://localhost:6379',
    SMTP_PASSWORD: 'devpass123',
  },
  staging: {
    DATABASE_URL: 'postgres://stg.db.internal/app',
    STRIPE_SECRET: 'sk_test_51H8x2Kp...',
    REDIS_URL: 'redis://stg.cache:6379',
    SMTP_PASSWORD: 'a9F2k7Lm4Q',
  },
  prod: {
    DATABASE_URL: 'postgres://db.internal/app',
    STRIPE_SECRET: null,
    REDIS_URL: 'redis://cache.internal:6379',
    SMTP_PASSWORD: 'Z3n8Rt5Yq1',
  },
};

const MASK = '••••••••••••';

export function SecretDemo() {
  const [env, setEnv] = useState<EnvId>('dev');
  const [revealed, setRevealed] = useState<Record<string, boolean>>({});

  const vals = VALUES[env];
  const missing = useMemo(() => KEYS.filter((k) => vals[k] == null), [vals]);

  function selectEnv(id: EnvId) {
    setEnv(id);
    setRevealed({});
  }
  function toggle(key: string) {
    setRevealed((r) => ({ ...r, [key]: !r[key] }));
  }

  return (
    <div className="overflow-hidden rounded-lg border border-border bg-surface-elevated shadow-md">
      {/* Toolbar — app name + env segmented control */}
      <div className="flex flex-wrap items-center justify-between gap-3 border-b border-border px-4 py-3.5">
        <div className="flex items-center gap-2 font-mono text-xs text-muted">
          <Database className="h-[15px] w-[15px]" aria-hidden />
          checkout-app
        </div>
        <div
          role="group"
          aria-label="환경 선택"
          className="flex flex-wrap gap-0.5 rounded-md border border-border bg-surface-hover p-[3px]"
        >
          {ENVS.map((e) => {
            const active = env === e.id;
            return (
              <button
                key={e.id}
                type="button"
                aria-pressed={active}
                onClick={() => selectEnv(e.id)}
                className={cn(
                  'inline-flex items-center gap-1.5 rounded-sm px-3 py-1.5 text-xs font-semibold transition-colors duration-fast',
                  active
                    ? 'bg-surface-elevated text-text shadow-sm'
                    : 'text-text-subtle hover:text-text',
                )}
              >
                {e.label}
                <span className="font-mono font-medium opacity-70">{e.mono}</span>
              </button>
            );
          })}
        </div>
      </div>

      {/* Missing-key banner — the drift signal */}
      {missing.length > 0 && (
        <p className="flex items-center gap-2.5 border-b border-border bg-danger-soft px-4 py-2.5 text-xs text-danger-strong">
          <AlertTriangle className="h-3.5 w-3.5 shrink-0" aria-hidden />
          <span>
            <strong className="font-bold">{missing.join(', ')}</strong>: 다른 환경엔 있지만 이
            환경에 누락됨
          </span>
        </p>
      )}

      {/* Rows */}
      <ul>
        {KEYS.map((key) => {
          const value = vals[key];
          const isMissing = value == null;
          const isRevealed = !!revealed[key];
          return (
            <li
              key={key}
              className="grid grid-cols-[1fr_auto] items-center gap-3 border-t border-border px-3.5 py-[13px] first:border-t-0"
            >
              <div className="min-w-0">
                <div className="font-mono text-sm font-semibold text-text">{key}</div>
                <div
                  className={cn(
                    'mt-0.5 truncate font-mono text-sm',
                    isMissing
                      ? 'text-danger-strong'
                      : isRevealed
                        ? 'text-text'
                        : 'text-text-faint',
                  )}
                >
                  {isMissing ? '이 환경에 없음' : isRevealed ? value : MASK}
                </div>
              </div>
              {isMissing ? (
                <span className="inline-flex h-[22px] items-center rounded-full bg-danger-soft px-2 text-xs font-semibold text-danger-strong">
                  빠짐
                </span>
              ) : (
                <button
                  type="button"
                  onClick={() => toggle(key)}
                  aria-pressed={isRevealed}
                  aria-label={`${key} 값 ${isRevealed ? '숨기기' : '보기'}`}
                  className="inline-grid h-[26px] w-[26px] place-items-center rounded-md border border-transparent text-text-subtle transition-colors duration-fast hover:border-border hover:bg-surface-hover hover:text-text"
                >
                  {isRevealed ? (
                    <EyeOff className="h-4 w-4" aria-hidden />
                  ) : (
                    <Eye className="h-4 w-4" aria-hidden />
                  )}
                </button>
              )}
            </li>
          );
        })}
      </ul>
    </div>
  );
}
