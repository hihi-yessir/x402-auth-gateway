package service

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"time"
)

type CREJob struct {
	AgentID  string
	ResultCh chan *EventResult
}

var creJobQueue chan CREJob

// CRE Worker Pool 초기화
func InitCREWorkerPool() {
	workerCount := 1 // 기본값
	if v := os.Getenv("CRE_WORKER_COUNT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			workerCount = n
		}
	}

	creJobQueue = make(chan CREJob, 50)

	for i := 0; i < workerCount; i++ {
		go creWorker(i)
	}
	log.Printf("CRE Worker Pool 시작: workers=%d\n", workerCount)
}

// Worker: CRE 호출 → 이벤트 대기 → Settle/Refund → 이미지생성 전부 순차 처리
func creWorker(id int) {
	for job := range creJobQueue {
		log.Printf("Worker[%d]: agentId=%s 파이프라인 시작\n", id, job.AgentID)
		processFullPipeline(id, job)
		log.Printf("Worker[%d]: agentId=%s 파이프라인 완료\n", id, job.AgentID)
	}
}

func processFullPipeline(workerID int, job CREJob) {
	agentId := job.AgentID

	// 1. CRE 호출
	log.Printf("Worker[%d]: CRE 호출 중...\n", workerID)
	err := CallCRE(agentId)
	if err != nil {
		log.Printf("Worker[%d]: CRE 호출 실패: %v\n", workerID, err)
		job.ResultCh <- &EventResult{Granted: false, Error: fmt.Errorf("CRE 호출 실패: %v", err)}
		PendingPayments.Delete(agentId)
		return
	}
	log.Printf("Worker[%d]: CRE 호출 성공, 블록체인 이벤트 대기...\n", workerID)

	// 2. 블록체인 이벤트 대기 (Listener가 EventCh에 신호 보냄)
	val, ok := PendingPayments.Load(agentId)
	if !ok {
		job.ResultCh <- &EventResult{Granted: false, Error: fmt.Errorf("결제 데이터 없음")}
		return
	}
	ctx := val.(PaymentContext)

	select {
	case granted := <-ctx.EventCh:
		if granted {
			// 3. AccessGranted → Settle
			log.Printf("Worker[%d]: AccessGranted! Settle 호출 중...\n", workerID)
			txHash, err := Settle(ctx.Signature, ctx.Required)
			if err != nil {
				log.Printf("Worker[%d]: Settle 실패: %v\n", workerID, err)
			} else {
				log.Printf("Worker[%d]: Settle 성공: txHash=%s\n", workerID, txHash)
			}

			// 4. 이미지/비디오 생성 API 호출
			log.Printf("Worker[%d]: Generation API 호출: type=%s\n", workerID, ctx.Type)
			jobResp, err := GenerateContent(ctx.Type, ctx.Prompt)
			if err != nil {
				log.Printf("Worker[%d]: Generation API 실패: %v\n", workerID, err)
				job.ResultCh <- &EventResult{Granted: false, Error: err}
			} else {
				log.Printf("Worker[%d]: 생성 요청 성공: jobId=%s\n", workerID, jobResp.JobID)
				job.ResultCh <- &EventResult{
					Granted: true,
					TxHash:  txHash,
					JobID:   jobResp.JobID,
				}
			}
		} else {
			// AccessDenied → Refund
			log.Printf("Worker[%d]: AccessDenied! Refund 호출 중...\n", workerID)
			txHash, err := Refund(ctx.Signature, ctx.Required)
			if err != nil {
				log.Printf("Worker[%d]: Refund 실패: %v\n", workerID, err)
			} else {
				log.Printf("Worker[%d]: Refund 성공: txHash=%s\n", workerID, txHash)
			}
			job.ResultCh <- &EventResult{
				Granted: false,
				Error:   fmt.Errorf("access denied by blockchain"),
			}
		}

	case <-time.After(90 * time.Second):
		log.Printf("Worker[%d]: 블록체인 이벤트 타임아웃\n", workerID)
		job.ResultCh <- &EventResult{Granted: false, Error: fmt.Errorf("blockchain event timeout")}
	}

	PendingPayments.Delete(agentId)
}

// 외부에서 CRE 작업 제출
func SubmitCREJob(agentId string, resultCh chan *EventResult) {
	creJobQueue <- CREJob{AgentID: agentId, ResultCh: resultCh}
}
