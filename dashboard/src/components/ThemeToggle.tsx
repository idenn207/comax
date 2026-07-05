import { DropdownMenu } from '@radix-ui/themes';

import type { ThemePreference } from '../lib/theme';
import { useThemeContext } from './ThemeRoot';

/**
 * Header-right theme switcher. An icon-button trigger that opens a Radix
 * DropdownMenu with system / light / dark. The icon swaps based on the
 * RESOLVED appearance, not the preference — system at night shows the
 * moon, system at day shows the sun, so the icon tells the operator what
 * the screen actually looks like rather than what setting they picked.
 *
 * Sidebar footer no longer owns this; theme switching is a global
 * utility and belongs adjacent to other globals (search trigger,
 * eventually account menu) on the header right edge. Adding more
 * controls later only widens the right column; nothing else shifts.
 *
 * Korean labels per CLAUDE.md (UI 라벨/설명문은 한국어 유지).
 */

const OPTIONS: Array<{ value: ThemePreference; label: string }> = [
  { value: 'system', label: '시스템' },
  { value: 'light', label: '라이트' },
  { value: 'dark', label: '다크' },
];

export function ThemeToggle() {
  const { preference, appearance, setPreference } = useThemeContext();
  const Icon = appearance === 'dark' ? IconMoon : IconSun;

  return (
    <DropdownMenu.Root>
      <DropdownMenu.Trigger>
        <button
          type="button"
          className="icon-button"
          aria-label={`테마 선택 (현재: ${labelFor(preference)})`}
        >
          <Icon />
        </button>
      </DropdownMenu.Trigger>
      <DropdownMenu.Content className="dropdown-content" sideOffset={6} align="end">
        {OPTIONS.map((opt) => {
          const selected = preference === opt.value;
          return (
            <DropdownMenu.Item
              key={opt.value}
              className="dropdown-item"
              data-state={selected ? 'checked' : 'unchecked'}
              onSelect={() => setPreference(opt.value)}
            >
              <IconFor value={opt.value} />
              <span>{opt.label}</span>
              {selected ? (
                <span className="dropdown-item-check" aria-hidden="true">
                  <IconCheck />
                </span>
              ) : null}
            </DropdownMenu.Item>
          );
        })}
      </DropdownMenu.Content>
    </DropdownMenu.Root>
  );
}

function labelFor(value: ThemePreference): string {
  return OPTIONS.find((o) => o.value === value)?.label ?? '시스템';
}

function IconFor({ value }: { value: ThemePreference }) {
  if (value === 'light') return <IconSun />;
  if (value === 'dark') return <IconMoon />;
  return <IconSystem />;
}

/* 1.5px stroke, currentColor, 16px box — matches the sidebar icon set. */

function IconSun() {
  return (
    <svg width="16" height="16" viewBox="0 0 24 24" fill="none" aria-hidden="true">
      <circle cx="12" cy="12" r="4" stroke="currentColor" strokeWidth="1.5" />
      <path
        d="M12 2v2M12 20v2M4.93 4.93l1.41 1.41M17.66 17.66l1.41 1.41M2 12h2M20 12h2M4.93 19.07l1.41-1.41M17.66 6.34l1.41-1.41"
        stroke="currentColor"
        strokeWidth="1.5"
        strokeLinecap="round"
      />
    </svg>
  );
}

function IconMoon() {
  return (
    <svg width="16" height="16" viewBox="0 0 24 24" fill="none" aria-hidden="true">
      <path
        d="M21 12.79A9 9 0 1 1 11.21 3 7 7 0 0 0 21 12.79Z"
        stroke="currentColor"
        strokeWidth="1.5"
        strokeLinejoin="round"
      />
    </svg>
  );
}

function IconSystem() {
  return (
    <svg width="16" height="16" viewBox="0 0 24 24" fill="none" aria-hidden="true">
      <rect x="3" y="4" width="18" height="13" rx="2" stroke="currentColor" strokeWidth="1.5" />
      <path d="M8 21h8M12 17v4" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
    </svg>
  );
}

function IconCheck() {
  return (
    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" aria-hidden="true">
      <path
        d="m5 12 5 5 9-11"
        stroke="currentColor"
        strokeWidth="1.75"
        strokeLinecap="round"
        strokeLinejoin="round"
      />
    </svg>
  );
}
