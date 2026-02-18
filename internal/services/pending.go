package service

import "sync"

// 결제 대기 데이터 저장소 (메모리)
var PendingPayments sync.Map

type PaymentContext struct {
	Signature []byte
	Required  []byte
	AgentID   string
}
