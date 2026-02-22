package service

import "sync"

// 결제 승인 대기 데이터 저장소 (메모리)
var PendingPayments sync.Map //(key) agentId : (value) PaymentContext

type PaymentContext struct {
	Signature []byte //결제 서명(agent wallet 내용 포함 되어있음)
	Required  []byte //결제 요구
}
