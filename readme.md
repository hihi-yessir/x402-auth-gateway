📑 Project Spec: x402-auth-gateway (Go version)
1. 개요 (Overview)
본 서버는 AI 에이전트의 자율 경제 활동(영상 생성 등)에 대해 x402 결제와 Chainlink CRE 기반 신원 검증을 결합하여 책임 소재를 명확히 하는 인증 프록시 게이트웨이입니다.

2. 핵심 아키텍처 흐름 (Workflow)
단일 엔드포인트 POST /api/generate에서 헤더 상태에 따라 3단계 상태 머신으로 동작합니다.

Step 1: Payment Challenge (402 Required)
Trigger: 요청 헤더에 X-PAYMENT-SIG가 없음.

Action:

HTTP Status 402 Payment Required 반환.

Response Header에 X-Payment-Metadata 포함 (Price: 0.5 USDC, Recipient, Nonce 등).

Step 2: Payment Hold & CRE Trigger
Trigger: 요청 헤더에 X-PAYMENT-SIG (EIP-3009 서명) 포함.

Action:

Facilitator Verification: x402 Facilitator SDK를 호출하여 자금의 유효성을 검증하고 Hold(잠금) 상태로 전환.

JWT Signing: 게이트웨이의 Private Key를 사용하여 CRE 전용 JWT 생성 (Claims: agentId, exp, iat).

CRE HTTP Trigger: Authorization: Bearer <JWT> 헤더와 함께 Chainlink CRE의 HTTP 엔드포인트 호출.

Response: 에이전트에게 202 Accepted 반환.

Step 3: On-chain Event Monitoring & Settlement
Trigger: 온체인 ACE 컨트랙트에서 AccessGranted 또는 AccessDenied 이벤트 발생.

Action:

Success: AccessGranted 감지 시 실제 AI 영상 생성 API 호출 → 결과 반환 → Facilitator Settle (대금 정산).

Failure: AccessDenied 감지 시 에이전트에게 에러 전송 → Facilitator Refund (자금 환불).

3. 기술적 요구사항 (Implementation Details)
A. API Endpoint
POST /api/generate: 402 결제 및 인증 트리거 핸들러

GET /api/status/:jobId: (선택) 에이전트가 처리 상태를 확인할 수 있는 폴링 엔드포인트

B. 보안 프로토콜 (JWT)
Algorithm: RS256 또는 ES256 (CRE의 AuthorizedKeys 설정과 일치해야 함)

Key Management: 게이트웨이 서버의 Private Key는 환경 변수(.env)로 관리하며, 공개키는 CRE 워크플로우 등록 시 사용됨.

C. 블록체인 연동 (Go-Ethereum)
ethclient를 사용하여 ACE 컨트랙트의 이벤트를 실시간 구독(SubscribeFilterLogs).

AccessGranted(uint256 indexed agentId, address indexed owner) 이벤트 필터링.

D. x402 Facilitator 인터페이스
Validate(signature bytes): 서명 유효성 검사

Hold(amount uint256): 자금 점유

Settle(): 최종 정산 실행

Refund(): 취소 및 환불

4. 개발 환경 (Stack)
Language: Go (1.21+)

Framework: Gin-Gonic

Library: github.com/ethereum/go-ethereum, github.com/golang-jwt/jwt/v5