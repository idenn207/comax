import { useState, type ReactNode } from 'react';
import { AlertDialog, Button, Flex } from '@radix-ui/themes';

/**
 * Destructive-action confirmation dialog. Used for delete + rollback.
 *
 * onConfirm runs while the primary button is disabled and surfaces async
 * errors back to the parent via the returned promise. The dialog stays
 * open during the in-flight state so the operator sees the spinner and
 * a thrown error doesn't accidentally dismiss it.
 */

interface ConfirmDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  title: string;
  description: ReactNode;
  confirmLabel?: string;
  cancelLabel?: string;
  color?: 'red' | 'amber' | 'indigo';
  onConfirm: () => Promise<void> | void;
}

export function ConfirmDialog({
  open,
  onOpenChange,
  title,
  description,
  confirmLabel = '확인',
  cancelLabel = '취소',
  color = 'red',
  onConfirm,
}: ConfirmDialogProps) {
  const [busy, setBusy] = useState(false);

  async function handleConfirm(event: React.MouseEvent<HTMLButtonElement>) {
    event.preventDefault();
    if (busy) return;
    setBusy(true);
    try {
      await onConfirm();
      onOpenChange(false);
    } catch {
      // Callers surface failures via their own useMutation.onError
      // (toast / inline alert). We swallow here so the click handler
      // doesn't leak an unhandled rejection, and leave the dialog
      // open so the operator can retry or cancel.
    } finally {
      setBusy(false);
    }
  }

  return (
    <AlertDialog.Root open={open} onOpenChange={onOpenChange}>
      <AlertDialog.Content maxWidth="450px">
        <AlertDialog.Title>{title}</AlertDialog.Title>
        <AlertDialog.Description size="2">{description}</AlertDialog.Description>
        <Flex gap="3" mt="4" justify="end">
          <AlertDialog.Cancel>
            <Button variant="soft" color="gray" disabled={busy}>
              {cancelLabel}
            </Button>
          </AlertDialog.Cancel>
          <Button color={color} onClick={handleConfirm} disabled={busy}>
            {busy ? '처리 중…' : confirmLabel}
          </Button>
        </Flex>
      </AlertDialog.Content>
    </AlertDialog.Root>
  );
}
