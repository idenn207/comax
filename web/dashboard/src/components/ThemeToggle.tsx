import { SegmentedControl } from '@radix-ui/themes';

import type { ThemePreference } from '../lib/theme';
import { useThemeContext } from './ThemeRoot';

/**
 * Three-segment theme picker (system / 라이트 / 다크). Rendered in the
 * AppShell header so it never moves between routes. Keyboard nav, focus
 * ring, and arrow-key cycling come from SegmentedControl out of the box.
 *
 * The visible label is Korean per CLAUDE.md ("UI 라벨/설명문은 한국어
 * 유지"); the underlying value stays in the canonical English form so
 * tests, telemetry, and storage keys do not split brain.
 */

const OPTIONS: Array<{ value: ThemePreference; label: string; description: string }> = [
  { value: 'system', label: '자동', description: 'OS 설정 따름' },
  { value: 'light', label: '라이트', description: '라이트 테마' },
  { value: 'dark', label: '다크', description: '다크 테마' },
];

export function ThemeToggle() {
  const { preference, setPreference } = useThemeContext();

  return (
    <SegmentedControl.Root
      value={preference}
      onValueChange={(value) => setPreference(value as ThemePreference)}
      size="1"
      aria-label="테마 선택"
    >
      {OPTIONS.map((opt) => (
        // The accessible name MUST contain the visible label (WCAG 2.5.3
        // Label in Name) — otherwise voice-control users cannot say "자동"
        // and axe label-content-name-mismatch fires. We let the visible
        // text become the accessible name and put the longer phrasing in
        // `title` so it still surfaces as a tooltip.
        <SegmentedControl.Item key={opt.value} value={opt.value} title={opt.description}>
          {opt.label}
        </SegmentedControl.Item>
      ))}
    </SegmentedControl.Root>
  );
}
