require('dotenv').config({ path: '../.env' });
const { wrapFetchWithPayment } = require('@x402/fetch');
const { x402Client } = require('@x402/core/client');
const { registerExactEvmScheme } = require('@x402/evm/exact/client');
const { privateKeyToAccount } = require('viem/accounts');
const { createPublicClient, http } = require('viem');
const { baseSepolia } = require('viem/chains');
const { EventSource } = require('eventsource'); // npm install eventsource

// 설정
const AGENT_PRIVATE_KEY = process.env.AGENT_PRIVATE_KEY;
const GATEWAY_URL = process.env.GATEWAY_URL || 'http://localhost:8081';
const AGENT_ID = process.env.AGENT_ID || '963';
const RPC_URL = process.env.HTTP_RPC_URL || 'https://sepolia.base.org';

async function main() {
  console.log('\n========================================');
  console.log('   Auth-OS Gateway Full Flow Test');
  console.log('========================================\n');

  if (!AGENT_PRIVATE_KEY) {
    throw new Error('AGENT_PRIVATE_KEY가 .env 파일에 없습니다!');
  }

  // 1. x402 클라이언트 설정
  console.log('[1/4] x402 클라이언트 설정...');
  const publicClient = createPublicClient({
    chain: baseSepolia,
    transport: http(RPC_URL),
  });

  const formattedPrivKey = AGENT_PRIVATE_KEY.startsWith('0x')
    ? AGENT_PRIVATE_KEY
    : `0x${AGENT_PRIVATE_KEY}`;

  const signer = privateKeyToAccount(formattedPrivKey);
  const client = new x402Client();
  registerExactEvmScheme(client, { signer, publicClient });

  console.log(`   Agent Address: ${signer.address}`);
  console.log(`   Gateway URL: ${GATEWAY_URL}`);
  console.log(`   Agent ID: ${AGENT_ID}`);

  // 2. 결제 포함 요청
  console.log('\n[2/4] AI 생성 요청 (x402 결제 포함)...');
  const fetchWithPayment = wrapFetchWithPayment(fetch, client);

  const requestBody = {
    agentId: AGENT_ID,
    type: 'image',  // 'image' 또는 'video'
    prompt: 'a flying whale in sunset, digital art, vibrant colors'
  };

  console.log(`   Type: ${requestBody.type}`);
  console.log(`   Prompt: "${requestBody.prompt}"`);

  let jobId, txHash;

  try {
    const response = await fetchWithPayment(`${GATEWAY_URL}/api/generate`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(requestBody)
    });

    const result = await response.json();

    if (response.status === 202) {
      jobId = result.jobId;
      txHash = result.txHash;
      console.log(`\n   결제 성공!`);
      console.log(`   Job ID: ${jobId}`);
      console.log(`   Tx Hash: ${txHash}`);
      console.log(`   BaseScan: https://sepolia.basescan.org/tx/${txHash}`);
    } else {
      console.error(`   실패 [${response.status}]:`, result);
      return;
    }
  } catch (error) {
    console.error('   결제 플로우 실패:', error.message);
    return;
  }

  // 3. SSE 스트리밍 연결
  console.log('\n[3/4] SSE 스트리밍 연결...');
  console.log(`   URL: ${GATEWAY_URL}/api/jobs/${jobId}/stream`);
  console.log('\n   --- 실시간 진행 상황 ---');

  await new Promise((resolve, reject) => {
    const eventSource = new EventSource(`${GATEWAY_URL}/api/jobs/${jobId}/stream`);

    eventSource.onmessage = (event) => {
      try {
        const data = JSON.parse(event.data);

        if (data.error && data.status !== 'failed') {
          console.log(`   ⚠️  Error: ${data.error}`);
          return;
        }

        // 상태 표시 (queued, processing, completed, failed)
        const statusIcons = {
          'queued': 'q',
          'processing': 'p',
          'completed': 'c',
          'failed': 'f'
        };
        const icon = statusIcons[data.status] || '?';
        console.log(`   ${icon} Status: ${data.status} (type: ${data.type})`);

        // 완료 시
        if (data.status === 'completed') {
          console.log('\n[4/4] 생성 완료!');
          if (data.artifact_url) {
            console.log(`   Artifact URL: ${data.artifact_url}`);
          }
          eventSource.close();
          resolve();
        }

        // 실패 시
        if (data.status === 'failed') {
          console.log(`\n   생성 실패: ${data.error || 'Unknown error'}`);
          eventSource.close();
          reject(new Error(data.error));
        }
      } catch (e) {
        console.log(`   Raw data: ${event.data}`);
      }
    };

    eventSource.onerror = (error) => {
      console.error('   SSE 연결 에러:', error);
      eventSource.close();
      reject(error);
    };

    // 타임아웃 (5분)
    setTimeout(() => {
      console.log('\n   타임아웃 (5분)');
      eventSource.close();
      resolve();
    }, 5 * 60 * 1000);
  });

  console.log('\n========================================');
  console.log('   Full Flow Test Complete!');
  console.log('========================================\n');
}

main()
  .then(() => process.exit(0))
  .catch((err) => {
    console.error(err);
    process.exit(1);
  });
