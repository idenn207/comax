/**
 * Format a snapshot of resolved secrets as a .env file body.
 *
 * Rules:
 *   - One KEY=VALUE per line.
 *   - Values containing newlines, quotes, '#', '=' or whitespace are
 *     double-quoted with \n / \" / \\ escaped. Values with neither stay
 *     bare so the output round-trips cleanly through dotenv parsers.
 *   - Trailing newline so concatenated copies don't merge lines.
 *
 * We sort by key for deterministic clipboard output — operators paste
 * this into PR descriptions and the noise from random map iteration
 * is real friction.
 */

const NEEDS_QUOTE = /[\s"#'=\\]/;

export interface DotenvEntry {
  key: string;
  value: string;
}

export function formatDotenv(entries: DotenvEntry[]): string {
  if (entries.length === 0) return '';
  const sorted = [...entries].sort((a, b) => a.key.localeCompare(b.key));
  const lines = sorted.map(({ key, value }) => `${key}=${formatValue(value)}`);
  return `${lines.join('\n')}\n`;
}

function formatValue(value: string): string {
  if (value === '') return '';
  if (NEEDS_QUOTE.test(value)) {
    const escaped = value
      .replace(/\\/g, '\\\\')
      .replace(/"/g, '\\"')
      .replace(/\n/g, '\\n')
      .replace(/\r/g, '\\r');
    return `"${escaped}"`;
  }
  return value;
}
