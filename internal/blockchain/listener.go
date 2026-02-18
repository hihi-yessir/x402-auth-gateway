// Access Grated evnet감지 -> Settle 호출
package blockchain

import (
	"context"
	"log"
	"os"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"

	service "github.com/hihi-yessir/auth-os-gateway/internal/services"
)

// ACE 컨트랙트 이벤트 시그니처
var (
	accessGrantedSig = []byte("AccessGranted(uint256,address)")
	accessDeniedSig  = []byte("AccessDenied(uint256,address)")
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
	eventSig := vLog.Topics[0].Hex()

	// AccessGranted 이벤트
	if eventSig == common.BytesToHash(accessGrantedSig).Hex() {
		agentId := parseAgentId(vLog.Topics[1])
		log.Printf("AccessGranted: agentId=%s\n", agentId)
		onAccessGranted(agentId)
	}

	// AccessDenied 이벤트
	if eventSig == common.BytesToHash(accessDeniedSig).Hex() {
		agentId := parseAgentId(vLog.Topics[1])
		log.Printf("AccessDenied: agentId=%s\n", agentId)
		onAccessDenied(agentId)
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

	// 2. AI 서비스 실행 (TODO)
	log.Printf("AI 서비스 실행: agentId=%s\n", agentId)
	// service.CallAIService(agentId)

	// 3. Settle 호출
	txHash, err := service.Settle(ctx.Signature, ctx.Required)
	if err != nil {
		log.Printf("Settle 실패: %v\n", err)
		return
	}

	log.Printf("Settle 성공: txHash=%s\n", txHash)

	// 4. PendingPayments에서 삭제
	service.PendingPayments.Delete(agentId)
}

// AccessDenied 처리
func onAccessDenied(agentId string) {
	// 1. PendingPayments에서 결제 데이터 조회
	val, ok := service.PendingPayments.Load(agentId)
	if !ok {
		log.Printf("결제 데이터 없음: agentId=%s\n", agentId)
		return
	}
	ctx := val.(service.PaymentContext)

	// 2. Refund 호출
	txHash, err := service.Refund(ctx.Signature, ctx.Required)
	if err != nil {
		log.Printf("Refund 실패: %v\n", err)
		return
	}

	log.Printf("Refund 완료: txHash=%s\n", txHash)

	// 3. PendingPayments에서 삭제
	service.PendingPayments.Delete(agentId)
}
