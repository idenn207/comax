import { describe, expect, it } from 'vitest';

import { isValidName, nameError } from './validate';

describe('validate', () => {
  describe('isValidName', () => {
    it.each([
      ['simple', true],
      ['my-app', true],
      ['env.prod', true],
      ['under_score', true],
      ['plus+sign', true],
      ['1startsWithDigit', true],
      ['', false],
      ['has space', false],
      ['has/slash', false],
      ['quoted"value', false],
      ['emoji😀', false],
    ])('isValidName(%j) → %s', (value, expected) => {
      expect(isValidName(value)).toBe(expected);
    });

    it('rejects names over 128 chars', () => {
      expect(isValidName('a'.repeat(129))).toBe(false);
      expect(isValidName('a'.repeat(128))).toBe(true);
    });
  });

  describe('nameError', () => {
    it('returns null on a valid name', () => {
      expect(nameError('project', 'foo')).toBeNull();
    });

    it('flags empty string', () => {
      expect(nameError('project', '')).toMatch(/required/i);
    });

    it('flags forbidden characters', () => {
      expect(nameError('env', 'has space')).toMatch(/may only contain/);
    });

    it('flags overlong names', () => {
      expect(nameError('key', 'a'.repeat(200))).toMatch(/128/);
    });
  });
});
