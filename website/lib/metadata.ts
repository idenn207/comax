import type { Metadata } from 'next';
import { siteConfig, siteUrl } from '@/lib/site';

type PageMetaInput = {
  title?: string;
  description?: string;
  /** Path without host, e.g. "/docs/quickstart". */
  path?: string;
};

/**
 * Build per-page metadata anchored to SITE_URL. Canonical + OG URLs resolve
 * against metadataBase so they carry the real deploy host at build time
 * (Codex F1). Titles compose under a shared template.
 */
export function pageMetadata({ title, description, path = '/' }: PageMetaInput = {}): Metadata {
  const url = new URL(path, siteUrl).toString();
  const resolvedTitle = title ? `${title} · ${siteConfig.name}` : `${siteConfig.name} — ${siteConfig.tagline}`;
  const resolvedDescription = description ?? siteConfig.description;

  return {
    title: resolvedTitle,
    description: resolvedDescription,
    alternates: { canonical: url },
    openGraph: {
      type: 'website',
      url,
      siteName: siteConfig.name,
      title: resolvedTitle,
      description: resolvedDescription,
    },
    twitter: {
      card: 'summary_large_image',
      title: resolvedTitle,
      description: resolvedDescription,
    },
  };
}
