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
