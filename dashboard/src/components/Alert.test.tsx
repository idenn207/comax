import { describe, expect, it } from 'vitest';
import { screen, waitFor } from '@testing-library/react';

import { renderWithProviders } from '../test/renderWithProviders';
import { Alert } from './Alert';

describe('Alert', () => {
  describe('variant="form"', () => {
    it('renders nothing when message is null', () => {
      renderWithProviders(<Alert variant="form" message={null} />);
      expect(screen.queryByRole('alert')).toBeNull();
    });

    it('renders the message with role="alert"', () => {
      renderWithProviders(
        <Alert variant="form" message="저장에 실패했습니다. 잠시 후 다시 시도해 주세요." />,
      );
      expect(screen.getByRole('alert')).toHaveTextContent(
        '저장에 실패했습니다. 잠시 후 다시 시도해 주세요.',
      );
    });

    it('moves focus to the alert when a message appears', async () => {
      const { rerender } = renderWithProviders(<Alert variant="form" message={null} />);
      rerender(<Alert variant="form" message="네트워크 오류로 요청을 보낼 수 없습니다." />);

      await waitFor(() => {
        expect(screen.getByRole('alert')).toHaveFocus();
      });
    });

    it('exposes the alert as a focusable target (tabIndex=-1)', () => {
      renderWithProviders(<Alert variant="form" message="값을 다시 확인해 주세요." />);
      expect(screen.getByRole('alert')).toHaveAttribute('tabindex', '-1');
    });
  });

  describe('variant="page"', () => {
    it('renders nothing when message is null', () => {
      renderWithProviders(<Alert variant="page" message={null} />);
      expect(screen.queryByRole('alert')).toBeNull();
    });

    it('renders the message with role="alert"', () => {
      renderWithProviders(<Alert variant="page" message="버전 이력을 불러오지 못했습니다." />);
      expect(screen.getByRole('alert')).toHaveTextContent('버전 이력을 불러오지 못했습니다.');
    });

    it('does NOT pull focus to itself (focus stays with the operator)', () => {
      // The contract is "page-variant alerts must not steal focus".
      // Asserting "alert is not focused" is the direct shape of that
      // contract; tracking focus on a sibling through a rerender is
      // brittle in happy-dom because the body re-receives focus on
      // tree reconciliation, masking the actual signal.
      renderWithProviders(<Alert variant="page" message="동기화 실패" />);
      expect(screen.getByRole('alert')).not.toHaveFocus();
    });

    it('renders an optional action slot via children', () => {
      renderWithProviders(
        <Alert variant="page" message="프로젝트 목록을 불러오지 못했습니다">
          <button type="button" className="alert-page-action">
            목록 새로고침
          </button>
        </Alert>,
      );
      expect(screen.getByRole('button', { name: '목록 새로고침' })).toBeInTheDocument();
    });

    it('does NOT carry tabIndex (focus stays with the operator)', () => {
      renderWithProviders(<Alert variant="page" message="동기화 실패" />);
      expect(screen.getByRole('alert')).not.toHaveAttribute('tabindex');
    });
  });
});
