package service

import (
	"fmt"
	"log"
	"os"
	"strconv"
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

// Worker: CRE 호출만 순차 처리
func creWorker(id int) {
	for job := range creJobQueue {
		log.Printf("Worker[%d]: agentId=%s CRE 호출 시작\n", id, job.AgentID)
		err := CallCRE(job.AgentID)
		if err != nil {
			log.Printf("Worker[%d]: CRE 호출 실패: %v\n", id, err)
			job.ResultCh <- &EventResult{Granted: false, Error: fmt.Errorf("CRE 호출 실패: %v", err)}
		}
		log.Printf("Worker[%d]: agentId=%s CRE 호출 완료\n", id, job.AgentID)
	}
}

// 외부에서 CRE 작업 제출
func SubmitCREJob(agentId string, resultCh chan *EventResult) {
	creJobQueue <- CREJob{AgentID: agentId, ResultCh: resultCh}
}
