import { describe, expect, it } from 'vitest';
import { screen } from '@testing-library/react';
import { TextField } from '@radix-ui/themes';

import { renderWithProviders } from '../test/renderWithProviders';
import { FormField } from './FormField';

describe('FormField', () => {
  it('renders the label, wires htmlFor → id, and finds the control by label text', () => {
    renderWithProviders(
      <FormField id="email" label="이메일">
        {(fieldProps) => <TextField.Root {...fieldProps} />}
      </FormField>,
    );

    const input = screen.getByLabelText('이메일') as HTMLInputElement;
    expect(input.id).toBe('email');
    // No error → no aria-invalid / aria-errormessage on the control.
    expect(input).not.toHaveAttribute('aria-invalid');
    expect(input).not.toHaveAttribute('aria-errormessage');
    expect(input).not.toHaveAttribute('aria-describedby');
  });

  it('renders a hint and wires aria-describedby to it', () => {
    renderWithProviders(
      <FormField id="env" label="환경" hint="비워두면 상속 없음">
        {(fieldProps) => <TextField.Root {...fieldProps} />}
      </FormField>,
    );

    const input = screen.getByLabelText('환경');
    expect(input).toHaveAttribute('aria-describedby', 'env-hint');
    const hint = document.getElementById('env-hint');
    expect(hint).not.toBeNull();
    expect(hint).toHaveTextContent('비워두면 상속 없음');
  });

  it('renders an error with role="alert" and wires aria-invalid + aria-errormessage', () => {
    renderWithProviders(
      <FormField id="key" label="키" error="key is required.">
        {(fieldProps) => <TextField.Root {...fieldProps} />}
      </FormField>,
    );

    const input = screen.getByLabelText('키');
    expect(input).toHaveAttribute('aria-invalid', 'true');
    expect(input).toHaveAttribute('aria-errormessage', 'key-error');
    expect(input).toHaveAttribute('aria-describedby', 'key-error');

    const alert = screen.getByRole('alert');
    expect(alert).toHaveAttribute('id', 'key-error');
    expect(alert).toHaveTextContent('key is required.');
  });

  it('joins hint and error ids in aria-describedby when both are present', () => {
    renderWithProviders(
      <FormField id="name" label="이름" hint="영문/숫자/_-." error="이름이 필요합니다.">
        {(fieldProps) => <TextField.Root {...fieldProps} />}
      </FormField>,
    );

    const input = screen.getByLabelText('이름');
    expect(input).toHaveAttribute('aria-describedby', 'name-hint name-error');
    expect(input).toHaveAttribute('aria-invalid', 'true');
  });

  it('marks the control as required and renders a visible required mark', () => {
    renderWithProviders(
      <FormField id="token" label="서비스 토큰" required>
        {(fieldProps) => <TextField.Root {...fieldProps} />}
      </FormField>,
    );

    const input = screen.getByLabelText('서비스 토큰') as HTMLInputElement;
    expect(input.required).toBe(true);
    expect(screen.getByText('*')).toHaveAttribute('aria-hidden', 'true');
  });
});
