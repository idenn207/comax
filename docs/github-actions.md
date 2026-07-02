# GitHub Actions 통합 (M3)

Comax Secrets를 GitHub Actions 워크플로에 주입하는 composite action 사용법.
GitHub Secret에 시크릿을 하나하나 등록하지 않고, **CI 토큰 하나**로 서버의
시크릿을 워크플로 스텝에 넣는다.

## 주입 모델 두 가지

| 모델 | 노출 범위 | 언제 |
|---|---|---|
| **process-env** (기본, `run:`) | 지정한 command의 **자식 프로세스에만**. job의 다른 스텝은 시크릿을 못 본다. | 거의 항상. 기본값. |
| **github-env** (opt-in, `export-to: github-env`) | `$GITHUB_ENV`에 기록 → **이후 모든 스텝**의 env에 노출. | 뒤따르는 별도 스텝이 시크릿을 자기 env로 받아야 할 때만. |

기본은 process-env다. github-env는 노출을 job 전체로 넓히므로, 정말 필요한
경우에만 opt-in한다.

## 사전 준비

### 1. CI 토큰 발급 (admin only)

토큰 발급/회수는 **admin 토큰**(부트스트랩 토큰 또는 다른 admin)만 할 수 있다.
발급된 CI 토큰은 항상 **non-admin**이라, 유출돼도 추가 토큰을 만들거나 회수할
수 없다.

```bash
# admin 자격으로 로그인한 상태에서
secret token create --name ci-github
# → stdout에 plaintext 토큰이 한 번만 출력된다. 지금 복사해라.
```

발급된 plaintext를 GitHub 저장소의 **Actions secret**으로 등록한다
(예: `COMAX_TOKEN`). Settings → Secrets and variables → Actions → New secret.

토큰 목록/회수:

```bash
secret token list                 # id·name·admin·created·last-used·status
secret token revoke --id <ID>     # soft revoke — 즉시 인증 불가
```

### 2. CLI 바이너리 확보 (M3: `cli-path` 필수)

M3에서는 action에 `secret` CLI 바이너리 경로를 **직접 넘겨야 한다**
(`cli-path`). release 바이너리 자동 다운로드 fallback은 M8 예정이다.
워크플로에서 소스로 빌드하거나, 아티팩트로 받아 경로를 지정한다.

```yaml
- uses: actions/setup-go@v5
  with: { go-version: "1.25" }
- run: go build -o bin/secret ./cmd/cli   # 모노레포에서 직접 빌드
```

## 사용법

### process-env (기본)

```yaml
- name: Run tests with secrets
  uses: idenn207/comax-secrets@<ref>
  with:
    server: https://secrets.example.com
    token: ${{ secrets.COMAX_TOKEN }}
    project: my-app
    env: ci
    cli-path: ${{ github.workspace }}/bin/secret
    run: npm test        # 이 command의 자식 프로세스에만 시크릿이 주입된다
```

`run:`에 넘긴 command는 시크릿이 env로 주입된 상태로 실행된다. 워크플로의
다른 스텝은 그 시크릿을 볼 수 없다.

### github-env (opt-in, job 전역)

```yaml
- name: Load secrets job-wide
  uses: idenn207/comax-secrets@<ref>
  with:
    server: https://secrets.example.com
    token: ${{ secrets.COMAX_TOKEN }}
    project: my-app
    env: ci
    cli-path: ${{ github.workspace }}/bin/secret
    export-to: github-env      # 이후 모든 스텝의 env에 노출된다

- name: A later step
  run: echo "uses $DB_URL"     # 앞 스텝의 시크릿이 여기서도 보인다 (마스킹됨)
```

github-env 모드는 각 값에 GitHub `::add-mask::`를 등록해 로그에서 자동
마스킹한다.

## 자격증명 처리 (R2-2)

action은 자격증명을 `$RUNNER_TEMP/comax-creds.json`에 **일회성**으로 쓰고,
기본 `~/.config/comax/credentials.json` 경로는 **건드리지 않는다**. 마지막
cleanup 스텝(`if: always()`)이 성공/실패와 무관하게 이 파일을 삭제하므로,
러너 디스크에 자격증명이 남지 않는다. 토큰은 env로만 전달되어 프로세스
목록·명령줄에 노출되지 않는다.

## 마스킹 / scope 한계 (정직한 경계)

운영자가 실제 보호 범위 위에서 워크플로 위생을 설계할 수 있도록 한계를
명시한다.

- **마스킹은 best-effort다.** `::add-mask::`는 GitHub이 로그에서 그 문자열을
  가리는 것이지, 시크릿이 로그에 안 나가게 막는 게 아니다. 매우 짧거나
  저엔트로피인 값은 부분적으로 새어나올 수 있다. **process-env 기본 모드는
  값을 job 로그 경로에 아예 올리지 않으므로 이 한계를 회피**한다.
- **github-env는 노출을 job 전체로 넓힌다.** opt-in한 순간부터 뒤따르는 모든
  스텝(서드파티 action 포함)이 시크릿을 env로 볼 수 있다. 신뢰할 수 없는
  스텝이 뒤에 있다면 process-env를 써라.
- **CI 토큰은 project/env 단위 read scope가 없다 (M4 이연).** non-admin CI
  토큰도 현재는 그 서버의 **모든 project/env 시크릿을 read**할 수 있다.
  scope 컬럼·미들웨어 인가는 위협모델 재정의가 필요해 M4로 이연됐다. M3에서
  blast radius를 줄이는 수단은 **발급을 admin-only로 제한**하고, 유출 의심 시
  **즉시 revoke**하는 것이다(revoke는 bearer·대시보드 세션 양쪽을 끊는다).
- **revoke는 회수지 소급 방지가 아니다.** 회수 이전에 이미 read된 값은
  되돌릴 수 없다. 토큰이 유출됐다면 회수와 별개로 해당 시크릿 값을
  로테이션해라.

## 관련 문서

- [threat-model.md](threat-model.md) — CI 토큰 authz/revoke, read-scope 한계
- [quickstart.md](quickstart.md) — 서버·CLI 5분 워크스루
