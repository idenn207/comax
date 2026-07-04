# Webhooks (M4)

시크릿이 바뀔 때(생성·수정·롤백·삭제) 등록한 URL로 **서명된 이벤트**를 POST한다.
운영자는 이 트리거를 받아 Docker Swarm 서비스 재시작 같은 후속 동작을 돌린다.

핵심 원칙:

- **페이로드에 시크릿 값이 없다.** 이벤트는 무엇이 바뀌었는지(project/env/key/
  version/action)만 알린다. 수신자는 트리거를 받은 뒤 자신의 인증된 CLI로 값을
  다시 pull 한다.
- **at-least-once.** 배달은 최소 한 번 보장이며 중복될 수 있다. 수신자는
  **멱등**해야 한다(같은 `X-Comax-Delivery` id를 두 번 처리해도 안전하게).
- **admin 전용.** 등록·삭제·활성화 전환은 admin 토큰만 가능하다.

## 등록

### CLI

```bash
# 프로젝트 comax 의 모든 환경, 모든 이벤트를 구독
secret webhook create --project comax --url https://deploy.internal/redeploy

# prod 환경의 upsert/delete 만 구독
secret webhook create \
  --project comax --env prod \
  --url http://deploy.internal/hook \
  --events secret.upsert,secret.delete
```

`create` 는 **서명 시크릿을 stdout 에 한 번만** 출력한다(안내는 stderr). 지금
복사해서 수신 서버의 서명 검증에 저장한다. 다시 조회할 수 없다.

```bash
secret webhook list                 # 등록된 웹훅 표
secret webhook deliveries --id 3     # 최근 배달 상태
secret webhook disable --id 3        # 소프트 비활성화(새 이벤트 중단, 이력 유지)
secret webhook enable  --id 3        # 다시 활성화
secret webhook delete --id 3         # 삭제(배달 이력도 함께 제거)
```

### 활성 / 비활성 (소프트 disable)

`disable` 한 웹훅은 등록·배달 이력을 그대로 둔 채 **새 이벤트만 중단**한다
(`MatchForEvent` 가 `enabled=1` 만 고른다). 이미 큐잉된 배달은 계속 빠져나간다.
`enable` 로 되돌린다 — 삭제 후 재등록의 **가역 대안**이다. delete 와 달리 서명
시크릿을 다시 발급하지 않으므로 수신자 설정을 바꾸지 않고 잠시 껐다 켤 수 있다.

API: `PATCH /api/v1/webhooks/{id}` 본문 `{"enabled": false|true}` (admin 전용).

### 대시보드

`연동 → 웹훅`(`/integrations/webhooks`). 등록 시 서명 시크릿을 1회 노출하는
다이얼로그가 뜬다. admin 세션만 목록·등록·삭제·활성 전환이 보이며, 각 행의
`비활성화`/`활성화` 버튼으로 상태를 토글한다.

## 페이로드 스키마

`Content-Type: application/json` 본문:

```json
{
  "action": "secret.upsert",
  "project": "comax",
  "env": "prod",
  "key": "DB_URL",
  "version": 7,
  "timestamp": 1751000000
}
```

- `action` — `secret.upsert` | `secret.rollback` | `secret.delete`
- `version` — 변경 후 버전. `secret.delete` 에는 없다(생략).
- `timestamp` — 이벤트 기록 시각(unix seconds).
- **값 필드는 없다.** 구조적으로 시크릿 평문이 실릴 자리가 없다.

배달 id 는 본문이 아니라 헤더로 전달된다(GitHub 의 `X-GitHub-Delivery` 와 동일).

## 헤더

| 헤더 | 내용 |
|---|---|
| `X-Comax-Signature` | `sha256=<hex>` — HMAC-SHA256(`"<timestamp>.<body>"`) |
| `X-Comax-Timestamp` | 서명에 쓰인 unix seconds. replay 방지에 쓴다 |
| `X-Comax-Event` | 이벤트 종류(`secret.upsert` 등) |
| `X-Comax-Delivery` | 배달 id. **멱등 키로 사용** |

## 서명 검증

서명은 `"<timestamp>.<body>"` 에 대한 HMAC-SHA256(서명 시크릿)이다. 등록 시 받은
서명 시크릿으로 재계산해 **상수시간 비교**한다. `X-Comax-Timestamp` 가 현재
시각에서 너무 벌어졌으면(예: 5분) 거부해 replay 를 막는다.

Node 수신자 예:

```js
const crypto = require('node:crypto');

function verify(req, rawBody, signingSecret) {
  const ts = req.headers['x-comax-timestamp'];
  const sig = req.headers['x-comax-signature'] || '';
  // replay 방지: 타임스탬프가 5분 이상 어긋나면 거부
  if (Math.abs(Date.now() / 1000 - Number(ts)) > 300) return false;
  const mac = crypto.createHmac('sha256', signingSecret);
  mac.update(ts + '.' + rawBody); // rawBody 는 수신한 바이트 그대로
  const want = 'sha256=' + mac.digest('hex');
  return crypto.timingSafeEqual(Buffer.from(sig), Buffer.from(want));
}
```

`rawBody` 는 파싱 전 원본 바이트여야 한다(재직렬화하면 서명이 깨진다).

## 재시도와 배달 상태

배달은 `pending → in_progress → {delivered | dead}` 로 흐른다.

- 2xx 응답 → `delivered`.
- 그 외(또는 전송 실패) → 지수 백오프로 재시도(`pending` 재진입).
- `maxAttempts`(기본 5) 초과 → `dead`(더는 재시도하지 않음). dead 행은
  대시보드/`webhook deliveries` 에서 확인한다.
- 워커가 배달 도중 크래시하면 lease(`claimed_at`)가 만료된 뒤 다른 tick 이
  회수해 재시도한다. 동시 워커가 있어도 원자적 claim 으로 한 행은 한 워커만
  가져간다.

**Transactional outbox**: 배달 행은 시크릿 변경과 **같은 트랜잭션**에서 큐잉된다.
커밋된 변경만 배달 대상이 되므로, 롤백된 변경은 유령 배달을 만들지 않는다.

## 이벤트 scope

- `project` (필수) + `env` (선택, 생략 시 프로젝트의 모든 환경) + `events` 필터.
- per-key 필터는 v1 범위 밖(YAGNI). 특정 키만 받으려면 수신자에서 `key` 로
  거른다.

## SSRF 정책

웹훅의 목적이 내부 서비스 호출이므로 **RFC1918 사설/loopback 은 허용**한다(Docker
overlay 는 사설 대역). 다만 **link-local / 클라우드 metadata(169.254.0.0/16,
IPv6 `fe80::/10`, 특히 `169.254.169.254`)는 기본 차단**한다.

계층:

1. `http`/`https` 스킴만.
2. 등록·배달 시 host 를 resolve 해 link-local/metadata 면 거부.
3. 리다이렉트 미추종(3xx 로 내부 주소로 튀는 것을 차단).
4. 배달 시 `DialContext` 에서 resolve 된 IP 를 재검증(등록 후 DNS rebinding 방어).
5. 운영자 정책 override(CIDR·IP, 콤마 구분):
   - `COMAX_WEBHOOK_ALLOW` — 기본 차단을 뚫고 허용할 대역(예: 특정 metadata).
   - `COMAX_WEBHOOK_DENY` — 추가로 막을 내부 control-plane 대역.

## 서명 시크릿 회전

v1 에는 in-place 회전 경로가 없다. 시크릿을 바꾸려면 **삭제 후 재등록**한다
(웹훅은 트리거일 뿐이라 blast radius 가 제한적: 위조 트리거는 운영자 자신의
수신자에만 도달하고, 수신자는 인증 CLI 로 값을 재-pull 한다). in-place 회전 +
배달 이력 보존은 backlog 이연(`.claude/plans/codex-findings-backlog.md`).

## 폴링 주기

배달 워커는 기본 10초마다 outbox 를 폴링한다. 조정:

- `--webhook-poll-interval 3s` (플래그) 또는 `COMAX_WEBHOOK_POLL=3s` (env).
