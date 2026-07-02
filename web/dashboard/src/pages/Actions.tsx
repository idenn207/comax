import { useState } from 'react';
import { Button, Heading, Text } from '@radix-ui/themes';
import { Link } from '@tanstack/react-router';

import { AppShell } from '../components/AppShell';
import { PageHeader } from '../components/PageHeader';

/**
 * Actions page — `/integrations/github-actions`.
 *
 * A static reference for the M3 composite action. No new backend: it shows
 * copyable workflow snippets (process-env default + github-env opt-in), a
 * link to issue a token, and the honest scope limits. Vocabulary and
 * layout mirror GitHub's own Actions docs (PRODUCT.md 핵심 레퍼런스).
 */

const PROCESS_ENV_YAML = `- uses: idenn207/comax-secrets@<ref>
  with:
    server: https://secrets.example.com
    token: \${{ secrets.COMAX_TOKEN }}
    project: my-app
    env: ci
    cli-path: \${{ github.workspace }}/bin/secret
    run: npm test   # 이 command의 자식 프로세스에만 주입`;

const GITHUB_ENV_YAML = `- uses: idenn207/comax-secrets@<ref>
  with:
    server: https://secrets.example.com
    token: \${{ secrets.COMAX_TOKEN }}
    project: my-app
    env: ci
    cli-path: \${{ github.workspace }}/bin/secret
    export-to: github-env   # 이후 모든 스텝의 env에 노출 (마스킹됨)`;

export function ActionsPage() {
  return (
    <AppShell
      active="actions"
      crumbs={[{ label: '연동', to: '/' }, { label: 'GitHub Actions' }]}
    >
      <PageHeader title="GitHub Actions" />

      <div style={{ maxWidth: '72ch', display: 'flex', flexDirection: 'column', gap: 'var(--space-2)' }}>
        <Text as="p" color="gray">
          composite action 하나로 서버의 시크릿을 워크플로 스텝에 주입합니다. GitHub Secret을 키마다
          등록할 필요가 없습니다. 기본은 <strong>process-env</strong>(지정한 command의 자식
          프로세스에만), 필요할 때만 <strong>github-env</strong>(job 전역)로 opt-in 합니다.
        </Text>

        <Heading as="h2" size="4" mt="5" mb="1">
          1. 토큰 발급
        </Heading>
        <Text as="p" color="gray">
          먼저 <Link to="/settings/tokens">서비스 토큰</Link>을 발급하고, 출력된 plaintext를 GitHub 저장소의 secret(예: <code className="mono">COMAX_TOKEN</code>)으로
          저장하세요. 발급된 토큰은 non-admin이라 유출돼도 추가 토큰을 만들 수 없습니다.
        </Text>

        <Heading as="h2" size="4" mt="5" mb="1">
          2. process-env (기본)
        </Heading>
        <Text as="p" color="gray">
          <code className="mono">run:</code> 에 넘긴 command에만 시크릿이 주입됩니다. 워크플로의 다른
          스텝은 그 값을 보지 못합니다.
        </Text>
        <CopyBlock label="process-env 워크플로 스니펫" code={PROCESS_ENV_YAML} />

        <Heading as="h2" size="4" mt="5" mb="1">
          3. github-env (opt-in)
        </Heading>
        <Text as="p" color="gray">
          <code className="mono">export-to: github-env</code> 는 이후 모든 스텝의 env에 시크릿을
          노출합니다. 각 값은 <code className="mono">::add-mask::</code> 로 로그에서 마스킹됩니다. 뒤따르는
          스텝이 시크릿을 자기 env로 받아야 할 때만 사용하세요.
        </Text>
        <CopyBlock label="github-env 워크플로 스니펫" code={GITHUB_ENV_YAML} />

        <Heading as="h2" size="4" mt="5" mb="1">
          scope 한계
        </Heading>
        <ul style={{ margin: 0, paddingLeft: '1.2rem', color: 'var(--color-muted)' }}>
          <li>
            마스킹은 best-effort 입니다. 짧거나 저엔트로피인 값은 부분적으로 새어나올 수 있습니다.
            process-env 기본 모드는 값을 job 로그에 올리지 않아 이 한계를 회피합니다.
          </li>
          <li>
            CI 토큰은 project/env read scope가 없습니다(M4 이연). non-admin 토큰도 현재는 모든
            project/env 시크릿을 read 할 수 있으므로, 발급 admin-only + 회수로 범위를 관리하세요.
          </li>
          <li>
            회수는 소급 방지가 아닙니다. 회수 이전에 이미 read 된 값은 되돌릴 수 없습니다.
          </li>
        </ul>
      </div>
    </AppShell>
  );
}

interface CopyBlockProps {
  label: string;
  code: string;
}

/**
 * A copyable code block. Styling reuses existing design tokens (surface +
 * Radix radius/alpha border) inline rather than adding a new globals.css
 * rule, keeping the snippet self-contained and on-system.
 */
function CopyBlock({ label, code }: CopyBlockProps) {
  const [copied, setCopied] = useState(false);

  async function copy() {
    try {
      await navigator.clipboard.writeText(code);
      setCopied(true);
    } catch {
      setCopied(false);
    }
  }

  return (
    <div style={{ position: 'relative' }}>
      <pre
        className="mono"
        aria-label={label}
        style={{
          margin: 0,
          padding: '14px 16px',
          background: 'var(--color-surface-hover)',
          border: '1px solid var(--gray-a5)',
          borderRadius: 'var(--radius-3)',
          overflowX: 'auto',
          fontSize: '0.8125rem',
          lineHeight: 1.6,
        }}
      >
        <code>{code}</code>
      </pre>
      <Button
        type="button"
        variant="soft"
        size="1"
        onClick={copy}
        aria-label={`${label} 복사`}
        style={{ position: 'absolute', top: '8px', right: '8px' }}
      >
        {copied ? '복사됨' : '복사'}
      </Button>
    </div>
  );
}
