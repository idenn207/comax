/**
 * Mirror of the server's validateName rule (internal/server/validate.go).
 * Kept narrow on purpose: project/env/key names appear in URL paths, so
 * anything more permissive would force percent-encoding on the client +
 * loosen the server's defense. The dashboard reuses these for form
 * validation so the operator sees the failure synchronously instead of
 * after a 400 round-trip.
 *
 * Allowed: A-Z, a-z, 0-9, '_', '-', '.', '+'. Length 1..128.
 */

const NAME_REGEX = /^[A-Za-z0-9_.+-]+$/;
const MAX_NAME_LEN = 128;

/**
 * Single Korean-facing hint string that mirrors NAME_REGEX + MAX_NAME_LEN.
 * Dialogs that accept a project/env/key name import this so the FormField
 * hint can show the rule *before* the operator submits and learns it from
 * a red error. If the regex or length cap changes, this string must change
 * with it — keeping both in this module makes that pairing the local rule.
 */
export const NAME_FORMAT_HINT = '영문/숫자/. _ + - 만, 1~128자';

export function isValidName(value: string): boolean {
  if (value.length === 0 || value.length > MAX_NAME_LEN) return false;
  return NAME_REGEX.test(value);
}

export function nameError(field: string, value: string): string | null {
  if (value === '') return `${field} is required.`;
  if (value.length > MAX_NAME_LEN) {
    return `${field} must be at most ${MAX_NAME_LEN} characters.`;
  }
  if (!NAME_REGEX.test(value)) {
    return `${field} may only contain letters, digits, period, underscore, hyphen, plus.`;
  }
  return null;
}
