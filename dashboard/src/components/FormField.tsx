import { Text } from '@radix-ui/themes';
import type { ReactNode } from 'react';

export interface FormFieldRenderProps {
  id: string;
  'aria-invalid'?: boolean;
  'aria-describedby'?: string;
  'aria-errormessage'?: string;
  required?: boolean;
}

interface FormFieldProps {
  id: string;
  label: string;
  // hint is a single plain sentence. Restricting it from ReactNode → string
  // prevents callers from passing markup that breaks the aria-describedby
  // wiring (the hint <p> must own its text node).
  hint?: string;
  error?: string | null;
  required?: boolean;
  children: (props: FormFieldRenderProps) => ReactNode;
}

/**
 * Stacked label + (hint) + control + (error). Owns the id wiring + ARIA
 * (aria-invalid, aria-describedby, aria-errormessage) so the control
 * announces correctly without each dialog re-deriving the same ids.
 *
 * Render-prop API (children: (fieldProps) => ReactNode) instead of
 * React.cloneElement: Radix's TextField.Root / TextArea / Select.Trigger
 * each accept ARIA differently, and an explicit spread keeps the wiring
 * legible at the call site.
 *
 * Visual treatment matches the pre-extraction dialog pattern
 * (<Text as="div" size="2" mb="1" weight="medium"> for the label,
 * <Text role="alert" color="red" size="1"> for the error) so the
 * frozen 1920×1080 Playwright snapshots stay unchanged after migration.
 */
export function FormField({ id, label, hint, error, required, children }: FormFieldProps) {
  const hintId = hint ? `${id}-hint` : undefined;
  const errorId = error ? `${id}-error` : undefined;
  const describedBy = [hintId, errorId].filter(Boolean).join(' ') || undefined;

  // <label> contains ONLY the visible label text so getByLabelText('이름')
  // returns the exact match. The required '*' is a sibling, not a child,
  // so aria-hidden actually does its job (textContent on a label is
  // taken whole; aria-hidden inside a label is ignored by the label-text
  // matcher). Hint and error are also siblings, not nested.
  return (
    <div>
      <Text as="div" size="2" mb="1" weight="medium">
        <label htmlFor={id}>{label}</label>
        {required ? (
          <Text as="span" color="red" ml="1" aria-hidden="true">
            *
          </Text>
        ) : null}
      </Text>
      {hint ? (
        <Text as="p" id={hintId} size="1" color="gray" mb="2">
          {hint}
        </Text>
      ) : null}
      {children({
        id,
        'aria-invalid': error ? true : undefined,
        'aria-describedby': describedBy,
        'aria-errormessage': errorId,
        required: required || undefined,
      })}
      {error ? (
        <Text as="p" id={errorId} role="alert" color="red" size="1" mt="1">
          {error}
        </Text>
      ) : null}
    </div>
  );
}
