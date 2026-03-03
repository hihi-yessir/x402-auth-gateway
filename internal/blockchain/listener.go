// Access Granted event 감지 -> Settle 호출
package blockchain

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"

	service "github.com/hihi-yessir/auth-os-gateway/internal/services"
)

// fmt 사용 확인용 (listener에서 직접 사용하지 않더라도 다른 함수에서 사용)
var _ = fmt.Sprintf

// 이벤트 시그니처 (keccak256 해시로)
var (
	accessGrantedSig = crypto.Keccak256Hash([]byte("AccessGranted(uint256,address,uint8)"))
	accessDeniedSig  = crypto.Keccak256Hash([]byte("AccessDenied(uint256,bytes32)"))
)

// 이벤트 리스너 시작 (자동 재연결 포함)
func StartEventListener() {
	rpcURL := os.Getenv("RPC_URL") //wss
	AuthOSConsumer := os.Getenv("AUTHOSCONSUMER_CONTRACT_ADDRESS")

	if rpcURL == "" || AuthOSConsumer == "" {
		log.Println("경고: 블록체인 환경변수 누락, 이벤트 리스너 비활성화")
		return
	}

	contractAddr := common.HexToAddress(AuthOSConsumer)

	// 재연결 루프
	for {
		log.Println("블록체인 WebSocket 연결 시도...")

		err := listenEvents(rpcURL, contractAddr)
		if err != nil {
			log.Printf("이벤트 리스너 종료: %v\n", err)
		}

		// 5초 후 재연결
		log.Println("5초 후 재연결 시도...")
		time.Sleep(5 * time.Second)
	}
}

// 실제 이벤트 구독 & 수신 (연결 끊기면 에러 반환)
func listenEvents(rpcURL string, contractAddr common.Address) error {
	client, err := ethclient.Dial(rpcURL)
	if err != nil {
		return fmt.Errorf("이더리움 클라이언트 연결 실패: %v", err)
	}
	defer client.Close()

	query := ethereum.FilterQuery{
		Addresses: []common.Address{contractAddr},
	}

	logs := make(chan types.Log)
	sub, err := client.SubscribeFilterLogs(context.Background(), query, logs)
	if err != nil {
		return fmt.Errorf("이벤트 구독 실패: %v", err)
	}

	log.Println("AuthOSConsumer 이벤트 리스너 연결 성공!")

	// 이벤트 수신 루프
	for {
		select {
		case err := <-sub.Err():
			return fmt.Errorf("구독 에러 (재연결 예정): %v", err)

		case vLog := <-logs:
			handleEvent(vLog)
		}
	}
}

// 이벤트 처리
func handleEvent(vLog types.Log) {
	eventSig := vLog.Topics[0]

	if eventSig == accessGrantedSig {
		agentId := parseAgentId(vLog.Topics[1])
		log.Printf("Event 수신 ! AccessGranted: agentId=%s\n", agentId)
		onAccessGranted(agentId)
	}

	if eventSig == accessDeniedSig {
		agentId := parseAgentId(vLog.Topics[1])
		log.Printf("Event 수신! AccessDenied: agentId=%s\n", agentId)
		onAccessDenied(agentId)
	}
}

// agentId 파싱 (indexed uint256)
func parseAgentId(topic common.Hash) string {
	return topic.Big().String()
}

// AccessGranted 처리
func onAccessGranted(agentId string) {
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
	} else {
		log.Printf("Settle 성공: txHash=%s\n", txHash)
	}

	// 2. AI 서비스 실행
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
