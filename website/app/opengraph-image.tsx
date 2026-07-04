import { ImageResponse } from 'next/og';

// Site-wide default OG image. Text is ASCII only: next/og's default font is
// Latin-only, so Korean copy would render as tofu. Colors are hex (Satori has
// no oklch/CSS-var support) but track the monochrome-graphite + one blue token.
export const size = { width: 1200, height: 630 };
export const contentType = 'image/png';
export const alt = 'Comax Secrets — self-hosted secret management';

export default function OpengraphImage() {
  return new ImageResponse(
    (
      <div
        style={{
          width: '100%',
          height: '100%',
          display: 'flex',
          flexDirection: 'column',
          justifyContent: 'space-between',
          backgroundColor: '#17171b',
          padding: '80px',
          fontFamily: 'sans-serif',
        }}
      >
        <div style={{ display: 'flex', alignItems: 'center', gap: '18px' }}>
          <div
            style={{
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              width: '52px',
              height: '52px',
              borderRadius: '12px',
              border: '2px solid #3a3a42',
              color: '#f4f4f5',
              fontSize: '26px',
            }}
          >
            {'{ }'}
          </div>
          <div style={{ color: '#a1a1aa', fontSize: '30px' }}>Comax Secrets</div>
        </div>

        <div style={{ display: 'flex', flexDirection: 'column', gap: '20px' }}>
          <div
            style={{
              display: 'flex',
              color: '#f4f4f5',
              fontSize: '76px',
              fontWeight: 700,
              lineHeight: 1.1,
              letterSpacing: '-0.02em',
            }}
          >
            Self-hosted secret management
          </div>
          <div style={{ display: 'flex', color: '#a1a1aa', fontSize: '32px' }}>
            One SQLite mount. Worktree-aware CLI. GitHub Actions in one line.
          </div>
        </div>

        <div style={{ display: 'flex', alignItems: 'center', gap: '14px', fontSize: '26px' }}>
          <div style={{ display: 'flex', width: '12px', height: '12px', borderRadius: '9999px', backgroundColor: '#4f7cf5' }} />
          <div style={{ display: 'flex', color: '#71717a' }}>MIT · zero-dep SDK · Docker</div>
        </div>
      </div>
    ),
    { ...size },
  );
}
