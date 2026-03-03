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

// AccessGranted 처리 - EventCh에 신호만 보냄 (실제 작업은 Worker가 처리)
func onAccessGranted(agentId string) {
	val, ok := service.PendingPayments.Load(agentId)
	if !ok {
		log.Printf("결제 데이터 없음: agentId=%s\n", agentId)
		return
	}
	ctx := val.(service.PaymentContext)
	ctx.EventCh <- true // Worker에게 "승인됨" 신호
}

// AccessDenied 처리 - EventCh에 신호만 보냄
func onAccessDenied(agentId string) {
	val, ok := service.PendingPayments.Load(agentId)
	if !ok {
		log.Printf("결제 데이터 없음: agentId=%s\n", agentId)
		return
	}
	ctx := val.(service.PaymentContext)
	ctx.EventCh <- false // Worker에게 "거부됨" 신호
}
