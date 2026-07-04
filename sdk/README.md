# @comax-secrets/sdk

Comax Secrets 서버에서 런타임에 시크릿을 가져오는 Node/TypeScript SDK. Next.js·Node·Edge에서 **fetch + cache + reload**를 제공하는 단일 패키지이며, 런타임 의존성이 없다(global `fetch`만 사용).

- 서버가 참조/오버라이드를 이미 해석한 **평문 시크릿**을 반환하므로 SDK는 해석 로직을 다시 구현하지 않는다.
- M3 **서비스 토큰**(Bearer)으로 인증한다.
- `./webhook` 서브패스로 M4 웹훅 서명을 검증해 "시크릿 변경 → 캐시 reload"를 구현할 수 있다.

> ⚠️ **서버 전용.** 서비스 토큰은 환경의 시크릿 읽기 권한을 주므로, 토큰이 브라우저로 흘러가는 곳(클라이언트 컴포넌트, 번들된 클라이언트 코드)에서 절대 사용하지 않는다. Route Handler·Server Component·middleware 등 서버 런타임에서만 쓴다.

## 설치

```bash
npm install @comax-secrets/sdk
```

Node 18+ (global `fetch`) 필요. Edge / Workers / Deno / Bun에서도 동작한다.

## 빠른 시작

```ts
import { createClientFromEnv } from "@comax-secrets/sdk";

// COMAX_URL, COMAX_TOKEN, COMAX_PROJECT, COMAX_ENV 를 읽는다.
const secrets = createClientFromEnv();

const dbUrl = await secrets.get("DATABASE_URL");
```

명시적 옵션:

```ts
import { createClient } from "@comax-secrets/sdk";

const secrets = createClient({
  baseUrl: "https://secrets.example.com",
  token: process.env.COMAX_TOKEN!,
  project: "api",
  env: "prod",
  ttlMs: 60_000, // 캐시 신선도(기본 60s)
  timeoutMs: 10_000, // 요청 타임아웃(기본 10s)
});
```

## Next.js

### 모듈-스코프 싱글턴

서버리스/Edge는 콜드 스타트마다 모듈 상태가 초기화된다. 캐시는 **warm 인스턴스 내부**에서 서버 왕복을 줄이는 용도다(영속 캐시가 아니다). 클라이언트를 모듈 스코프에 한 번 만들어 재사용한다.

```ts
// lib/secrets.ts
import { createClientFromEnv } from "@comax-secrets/sdk";

export const secrets = createClientFromEnv();
```

### Route Handler

```ts
// app/api/config/route.ts
import { secrets } from "@/lib/secrets";

export async function GET() {
  const apiKey = await secrets.get("STRIPE_KEY");
  // ... apiKey로 서버측 호출. 값 자체는 응답에 넣지 않는다.
  return Response.json({ ok: true });
}
```

### Server Component

```ts
// app/page.tsx
import { secrets } from "@/lib/secrets";

export default async function Page() {
  const flag = await secrets.get("FEATURE_FLAG");
  return <main>{flag === "on" ? <Beta /> : <Stable />}</main>;
}
```

### Edge Runtime

```ts
export const runtime = "edge";
```

`fetch`와 Web Crypto만 쓰므로 Edge에서 그대로 동작한다. `process.env`가 제한된 런타임이라면 값을 직접 넘긴다:

```ts
const secrets = createClient({
  baseUrl: env.COMAX_URL,
  token: env.COMAX_TOKEN,
  project: "api",
  env: "prod",
  fetch, // 런타임의 fetch 주입(선택)
});
```

## 캐시 & reload

```ts
await secrets.get("KEY"); // 단일-키 fetch + per-key 캐시 (기본)
secrets.has("KEY"); // 캐시에 신선하게 있으면 true (fetch 없음)
secrets.reload("KEY"); // 한 키 무효화 → 다음 read가 refetch
secrets.reload(); // 전체 무효화

await secrets.getAll(); // 환경 전체를 { KEY: value }로 (bulk inject, opt-in)
await secrets.preload(); // 환경 전체 Secret[] 를 미리 채움
```

`get(key)`는 **단일-키 엔드포인트**만 사용한다 — 하나의 시크릿만 필요할 때 환경 전체 평문을 메모리에 올리지 않는다. 벌크 주입이 필요하면 `getAll()`/`preload()`를 명시적으로 쓴다.

long-running Node 서버라면 백그라운드 갱신을 켤 수 있다(Edge/serverless에는 지속 타이머가 없으므로 부적합):

```ts
const secrets = createClient({ /* ... */, refreshIntervalMs: 30_000 });
secrets.startAutoRefresh();
// 종료 시: secrets.stopAutoRefresh();
```

## Webhook 기반 reload

M4 웹훅으로 시크릿 변경 이벤트를 받아 캐시를 무효화한다. 서명 검증은 `./webhook` 서브패스에서 제공한다.

```ts
// app/api/comax-webhook/route.ts
import { secrets } from "@/lib/secrets";
import { verifyWebhookSignature, HEADER_SIGNATURE, HEADER_TIMESTAMP } from "@comax-secrets/sdk/webhook";

export async function POST(req: Request) {
  const body = await req.text(); // 원문 그대로(재직렬화 금지)
  const result = await verifyWebhookSignature({
    secret: process.env.COMAX_WEBHOOK_SECRET!,
    signatureHeader: req.headers.get(HEADER_SIGNATURE),
    timestampHeader: req.headers.get(HEADER_TIMESTAMP),
    body,
  });
  if (!result.valid) {
    return new Response("invalid signature", { status: 401 });
  }
  secrets.reload(); // 변경 반영
  return new Response("ok");
}
```

서명 스킴은 서버 `internal/webhook/signer.go`와 동일하다: `sha256=hex(HMAC-SHA256(secret, "<ts>.<body>"))`. timestamp가 서명 재료에 묶여 있어 replay를 막고, 기본 300초 tolerance 밖의 delivery는 거부한다.

## 에러 처리

모든 실패는 `ComaxError`(또는 하위 타입)로 던져진다. `.code`/`.status`로 분기한다. **시크릿 값·토큰은 에러 메시지에 절대 포함되지 않는다.**

```ts
import { ComaxAuthError, ComaxNotFoundError, ComaxError } from "@comax-secrets/sdk";

try {
  await secrets.get("KEY");
} catch (err) {
  if (err instanceof ComaxAuthError) {
    // 401/403 — 토큰 문제
  } else if (err instanceof ComaxNotFoundError) {
    // 404 — 키/환경 없음
  } else if (err instanceof ComaxError && err.code === "timeout") {
    // 타임아웃/취소
  }
}
```

| 코드 | 의미 |
|---|---|
| `unauthorized` / `forbidden` | 토큰 누락·무효·권한 부족 |
| `not_found` / `version_not_found` | 리소스 없음 |
| `conflict` / `already_bootstrapped` | 충돌 |
| `bad_request` | 잘못된 입력/설정 |
| `timeout` | 요청 타임아웃 또는 caller abort |
| `network` | 네트워크 실패 |
| `invalid_response` | 응답 envelope 파싱 실패 |

## License

MIT
