package service

import "sync"

// 결제 승인 대기 데이터 저장소 (메모리)
var PendingPayments sync.Map //(key) agentId : (value) PaymentContext

// 이벤트 결과
type EventResult struct {
	Granted bool
	TxHash  string
	JobID   string
	Error   error
}

// PaymentContext 확장
type PaymentContext struct {
	Signature []byte
	Required  []byte
	Type      string // "image" or "video"
	Prompt    string
	ResultCh  chan *EventResult // handler에게 최종 결과 전달
	EventCh   chan bool         // listener → worker: true=granted, false=denied
}
