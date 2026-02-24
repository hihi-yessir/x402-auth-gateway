// Access Grated evnet감지 -> Settle 호출
package blockchain

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"

	service "github.com/hihi-yessir/auth-os-gateway/internal/services"
)

// 이벤트 시그니처 (keccak256 해시로)
var (
	accessGrantedSig = crypto.Keccak256Hash([]byte("AccessGranted(uint256,address,uint8)"))
	accessDeniedSig  = crypto.Keccak256Hash([]byte("AccessDenied(uint256,bytes32)"))
)

// 이벤트 리스너 시작
func StartEventListener() {
	rpcURL := os.Getenv("RPC_URL") //wss
	AuthOSConsumer := os.Getenv("AUTHOSCONSUMER_CONTRACT_ADDRESS")

	if rpcURL == "" || AuthOSConsumer == "" {
		log.Println("경고: 블록체인 환경변수 누락, 이벤트 리스너 비활성화")
		return
	}

	client, err := ethclient.Dial(rpcURL)
	if err != nil {
		log.Printf("이더리움 클라이언트 연결 실패: %v\n", err)
		return
	}

	contractAddr := common.HexToAddress(AuthOSConsumer)

	// 필터 쿼리 설정
	query := ethereum.FilterQuery{
		Addresses: []common.Address{contractAddr},
	}

	// 이벤트 구독
	logs := make(chan types.Log)
	sub, err := client.SubscribeFilterLogs(context.Background(), query, logs)
	if err != nil {
		log.Printf("이벤트 구독 실패: %v\n", err)
		return
	}

	log.Println("AuthOSConsumer 컨트랙트 이벤트 리스너 시작됨")

	// 이벤트 처리 (고루틴)
	go func() {
		for {
			select {
			case err := <-sub.Err():
				log.Printf("구독 에러: %v\n", err)
				return

			case vLog := <-logs:
				handleEvent(vLog)
			}
		}
	}()
}

// 이벤트 처리
func handleEvent(vLog types.Log) {
	// 이벤트 토픽으로 이벤트 타입 구분
	eventSig := vLog.Topics[0]

	// AccessGranted 이벤트
	if eventSig == accessGrantedSig { // 직접 비교!
		agentId := parseAgentId(vLog.Topics[1])
		log.Printf("Event 수신 ! AccessGranted: agentId=%s\n", agentId)
		onAccessGranted(agentId) //!!! 소문자로 바꿔라
	}

	// AccessDenied 이벤트
	if eventSig == accessDeniedSig {
		agentId := parseAgentId(vLog.Topics[1])
		log.Printf("Event 수신! AccessDenied: agentId=%s\n", agentId)
		onAccessDenied(agentId) //!!! 소문자로 바꿔라
	}
}

// agentId 파싱 (indexed uint256)
func parseAgentId(topic common.Hash) string {
	return topic.Big().String()
}

// AccessGranted 처리
func onAccessGranted(agentId string) {
	// 1. PendingPayments에서 결제 데이터 조회
	val, ok := service.PendingPayments.Load(agentId)
	if !ok {
		log.Printf("결제 데이터 없음: agentId=%s\n", agentId)
		return
	}
	ctx := val.(service.PaymentContext)

	// 1. Settle 호출
	txHash, err := service.Settle(ctx.Signature, ctx.Required)
	if err != nil {
		log.Printf("Settle 실패: %v\n", err)
		// Settle 실패해도 AI 서비스는 실행 (이미 검증됨)
	} else {
		log.Printf("Settle 성공: txHash=%s\n", txHash)
	}

	// 2. AI 서비스 실행 (TODO)
	log.Printf("AI 서비스 실행: agentId=%s\n", agentId)
	log.Printf("Generation API 호출: type=%s, prompt=%s\n", ctx.Type, ctx.Prompt)
	jobResp, err := service.GenerateContent(ctx.Type, ctx.Prompt)
	if err != nil {
		log.Printf("Generation API 실패: %v\n", err)
		ctx.ResultCh <- &service.EventResult{Granted: false, Error: err}
		service.PendingPayments.Delete(agentId)
		return
	}
	log.Printf("생성 요청 성공: jobId=%s\n", jobResp.JobID)

	// 3. 결과 채널에 전달
	ctx.ResultCh <- &service.EventResult{
		Granted: true,
		TxHash:  txHash,
		JobID:   jobResp.JobID,
	}

	// 4. PendingPayments에서 삭제 정리
	service.PendingPayments.Delete(agentId)
}

// AccessDenied 처리
func onAccessDenied(agentId string) {
	val, ok := service.PendingPayments.Load(agentId)
	if !ok {
		log.Printf("결제 데이터 없음: agentId=%s\n", agentId)
		return
	}
	ctx := val.(service.PaymentContext)

	// 1. Refund 호출
	txHash, err := service.Refund(ctx.Signature, ctx.Required)
	if err != nil {
		log.Printf("Refund 실패: %v\n", err)
	} else {
		log.Printf("Refund 성공: txHash=%s\n", txHash)
	}

	log.Printf("AccessDenied: agentId=%s\n", agentId)

	// 2. 결과 채널에 전달
	ctx.ResultCh <- &service.EventResult{
		Granted: false,
		Error:   fmt.Errorf("access denied by blockchain"),
	}

	// 3. 정리
	service.PendingPayments.Delete(agentId)
}
