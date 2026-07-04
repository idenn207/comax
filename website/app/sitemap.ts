import type { MetadataRoute } from 'next';
import { siteUrl } from '@/lib/site';
import { docHref, flatDocs } from '@/lib/docs';

export default function sitemap(): MetadataRoute.Sitemap {
  const paths = ['/', ...flatDocs.map((d) => docHref(d.slug))];
  return paths.map((path) => ({
    url: new URL(path, siteUrl).toString(),
    changeFrequency: 'weekly',
    priority: path === '/' ? 1 : 0.7,
  }));
}
