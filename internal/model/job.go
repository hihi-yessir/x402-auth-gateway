package model

import "time"

type Job struct {
	ID              string
	Status          string // "pending", "verified", "granted", "settled", "denied"
	PaymentPayload  []byte // settle 할 때 필요
	PaymentRequired []byte
	AgentID         string
	CreatedAt       time.Time
}

// 임시 저장 변수(나중에 DB로)
var Jobs = make(map[string]*Job)
