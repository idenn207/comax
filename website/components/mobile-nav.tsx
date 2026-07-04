'use client';

import { useState } from 'react';
import Link from 'next/link';
import * as Dialog from '@radix-ui/react-dialog';
import { Menu, X } from 'lucide-react';
import { mainNav } from '@/lib/site';

export function MobileNav() {
  const [open, setOpen] = useState(false);

  return (
    <Dialog.Root open={open} onOpenChange={setOpen}>
      <Dialog.Trigger asChild>
        <button
          type="button"
          aria-label="메뉴 열기"
          className="inline-flex h-9 w-9 items-center justify-center rounded-md border border-border text-text-subtle transition-colors duration-fast hover:bg-surface-hover hover:text-text md:hidden"
        >
          <Menu className="h-[18px] w-[18px]" aria-hidden />
        </button>
      </Dialog.Trigger>
      <Dialog.Portal>
        <Dialog.Overlay className="fixed inset-0 z-40 bg-[oklch(0%_0_0/0.4)]" />
        <Dialog.Content className="fixed inset-y-0 right-0 z-50 flex w-[min(20rem,85vw)] flex-col gap-1 border-l border-border bg-surface-elevated p-4 shadow-lg focus:outline-none">
          <div className="mb-4 flex items-center justify-between">
            <Dialog.Title className="text-sm font-semibold text-text">메뉴</Dialog.Title>
            <Dialog.Close asChild>
              <button
                type="button"
                aria-label="메뉴 닫기"
                className="inline-flex h-9 w-9 items-center justify-center rounded-md border border-border text-text-subtle hover:bg-surface-hover hover:text-text"
              >
                <X className="h-[18px] w-[18px]" aria-hidden />
              </button>
            </Dialog.Close>
          </div>
          <nav className="flex flex-col gap-1">
            {mainNav.map((item) =>
              item.external ? (
                <a
                  key={item.href}
                  href={item.href}
                  className="rounded-md px-3 py-2 text-md text-text-subtle transition-colors hover:bg-surface-hover hover:text-text"
                  target="_blank"
                  rel="noreferrer noopener"
                >
                  {item.title}
                </a>
              ) : (
                <Link
                  key={item.href}
                  href={item.href}
                  onClick={() => setOpen(false)}
                  className="rounded-md px-3 py-2 text-md text-text-subtle transition-colors hover:bg-surface-hover hover:text-text"
                >
                  {item.title}
                </Link>
              ),
            )}
          </nav>
        </Dialog.Content>
      </Dialog.Portal>
    </Dialog.Root>
  );
}
