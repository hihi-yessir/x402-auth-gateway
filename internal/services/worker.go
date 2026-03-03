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

	creJobQueue = make(chan CREJob, 50) // 대기열 버퍼

	for i := 0; i < workerCount; i++ {
		go creWorker(i)
	}
	log.Printf("CRE Worker Pool 시작: workers=%d\n", workerCount)
}

// Worker: 큐에서 하나씩 꺼내서 순차 처리
func creWorker(id int) {
	for job := range creJobQueue {
		log.Printf("CRE Worker[%d]: agentId=%s 처리 시작\n", id, job.AgentID)
		err := CallCRE(job.AgentID)
		if err != nil {
			log.Printf("CRE Worker[%d]: 호출 실패: %v\n", id, err)
			job.ResultCh <- &EventResult{Granted: false, Error: fmt.Errorf("CRE 호출 실패: %v", err)}
		}
		log.Printf("CRE Worker[%d]: agentId=%s 처리 완료\n", id, job.AgentID)
	}
}

// 외부에서 CRE 작업 제출
func SubmitCREJob(agentId string, resultCh chan *EventResult) {
	creJobQueue <- CREJob{AgentID: agentId, ResultCh: resultCh}
}
