import * as Dialog from '@radix-ui/react-dialog';
import { Theme } from '@radix-ui/themes';
import type { ReactNode } from 'react';

/**
 * Right-anchored drawer (380px) built on Radix's Dialog primitive so the
 * focus trap, scroll lock, and escape-to-close come for free. Visual
 * surface is painted by `.drawer-panel` / `.drawer-backdrop` in
 * globals.css — Radix wires the data-state attribute the keyframes hook.
 *
 * Used instead of `@radix-ui/themes` Dialog when the operator's table
 * context behind the panel must stay partially visible. ConfirmDialog
 * still layers on top via the higher --z-modal band.
 *
 * Theme inheritance: Dialog.Portal renders into document.body, escaping
 * the app-root <Theme>. Without an inner <Theme appearance="inherit">,
 * any nested Radix Themes components (Button, Text, Callout) would lose
 * the --accent-*, --gray-* CSS variables. We chain asChild so the panel
 * element carries .drawer-panel + .radix-themes on a single div, with
 * hasBackground={false} so themes leaves --color-surface alone.
 *
 * KNOWN ISSUE (carry-over for next session): the `appearance="inherit"`
 * string is not a valid value in Radix Themes 3.3.0 — ThemeImpl never
 * matches it against 'light'/'dark', so the `.light` and `.dark` class
 * never get added. Result: Radix Themes children (Button/Text/Callout)
 * inside the drawer ignore the data-theme="dark" token swap and render
 * with Radix's light defaults. Removing the prop swaps the failure mode
 * (panel background goes transparent because .radix-themes.dark +
 * data-has-background="false" combine to override our --color-surface
 * with higher specificity than .drawer-panel). The correct fix is to
 * drop the inner <Theme> entirely and pass Dialog.Portal a `container`
 * prop pointing at the outer .radix-themes element so DOM cascade
 * carries the dark tokens across the portal. Tracked in memory:
 * dashboard-ui-drawer-darkmode-portal-cascade.
 *
 * Composition: <Drawer> takes open state + accessible name. Re-exports
 * Title/Description/Close primitives so dot-notation usage matches the
 * themes Dialog idiom elsewhere in the codebase.
 */

interface DrawerRootProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  ariaLabel: string;
  children: ReactNode;
}

function DrawerRoot({ open, onOpenChange, ariaLabel, children }: DrawerRootProps) {
  return (
    <Dialog.Root open={open} onOpenChange={onOpenChange} modal>
      <Dialog.Portal>
        <Dialog.Overlay className="drawer-backdrop" />
        <Dialog.Content asChild aria-label={ariaLabel}>
          <Theme asChild appearance="inherit" hasBackground={false}>
            <div className="drawer-panel">{children}</div>
          </Theme>
        </Dialog.Content>
      </Dialog.Portal>
    </Dialog.Root>
  );
}

function DrawerHeader({ children }: { children: ReactNode }) {
  return <header className="drawer-header">{children}</header>;
}

function DrawerBody({ children }: { children: ReactNode }) {
  return <div className="drawer-body">{children}</div>;
}

function DrawerFooter({ children }: { children: ReactNode }) {
  return <footer className="drawer-footer">{children}</footer>;
}

export const Drawer = Object.assign(DrawerRoot, {
  Header: DrawerHeader,
  Body: DrawerBody,
  Footer: DrawerFooter,
  Title: Dialog.Title,
  Description: Dialog.Description,
  Close: Dialog.Close,
});
