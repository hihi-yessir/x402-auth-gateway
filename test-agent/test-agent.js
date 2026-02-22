require('dotenv').config({ path: '../.env' });
const { wrapFetchWithPayment } = require('@x402/fetch');
const { x402Client } = require('@x402/core/client');
const { registerExactEvmScheme } = require('@x402/evm/exact/client');
const { privateKeyToAccount } = require('viem/accounts');
const { createPublicClient, http } = require('viem');
const { baseSepolia } = require('viem/chains');

// 설정 부분
const AGENT_PRIVATE_KEY = process.env.AGENT_PRIVATE_KEY;
const GATEWAY_URL = process.env.GATEWAY_URL || 'http://localhost:8081';
const AGENT_ID = process.env.AGENT_ID || '963';
const RPC_URL = process.env.RPC_URL || 'https://sepolia.base.org';

async function main() {
  console.log('\n=== x402 Payment Test ===\n');
  console.log('AGENT_ID:', AGENT_ID);

  if (!AGENT_PRIVATE_KEY) {
    throw new Error('AGENT_PRIVATE_KEY가 .env 파일에 없습니다!');
  }

  // 1. viem publicClient 설정 (EIP-712 도메인 해석을 돕기 위해 필요)
  const publicClient = createPublicClient({
    chain: baseSepolia,
    transport: http(RPC_URL),
  });

  // 2. x402 클라이언트 설정
  // 0x 접두어 체크 로직 추가 (예슬 님 아까 고생하셨던 부분! ㅋ)
  const formattedPrivKey = AGENT_PRIVATE_KEY.startsWith('0x') 
    ? AGENT_PRIVATE_KEY 
    : `0x${AGENT_PRIVATE_KEY}`;
    
  const signer = privateKeyToAccount(formattedPrivKey);
  const client = new x402Client();

  // Exact EVM 스키마 등록 시 publicClient를 함께 넘겨줍니다.
  registerExactEvmScheme(client, { 
    signer,
    publicClient 
  });

  console.log('Using Signer Address:', signer.address);
  console.log('Gateway URL:', GATEWAY_URL);

  // 3. wrapFetchWithPayment로 fetch 감싸기
  // 이 녀석이 내부적으로 402 에러를 받으면 서명을 시도합니다.
  const fetchWithPayment = wrapFetchWithPayment(fetch, client);

  console.log('\nSending request to Gateway...');

  try {
    // 4. 결제 요청 실행
    const response = await fetchWithPayment(`${GATEWAY_URL}/api/generate`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'X-AGENT-ID': AGENT_ID
      },
      // 필요하다면 요청 바디도 추가 (지금은 빈 객체 예시)
      body: JSON.stringify({ prompt: "Hello, WhiteWall!" })
    });

    const result = await response.json();
    console.log(`\nResponse [${response.status}]:`, JSON.stringify(result, null, 2));

  } catch (error) {
    console.error('\nPayment flow failed:');
    console.error(error.message);
    
    if (error.message.includes('EIP-712 domain')) {
      console.log('\nTip: Gateway 서버가 402 응답 시 Domain Name과 Version을 제대로 주는지 확인');
    }
  }
}

main().catch(console.error);