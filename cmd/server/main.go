package main

import (
	"log"

	"github.com/gin-gonic/gin"
	"github.com/hihi-yessir/auth-os-gateway/internal/blockchain"
	"github.com/hihi-yessir/auth-os-gateway/internal/handlers"
	service "github.com/hihi-yessir/auth-os-gateway/internal/services"
	"github.com/joho/godotenv"
)

func main() {
	// .env 파일 로드
	if err := godotenv.Load(); err != nil {
		log.Println(".env 파일 없음", err)
	}
	//CRE 설정 초기화
	if err := service.InitCREConfig(); err != nil {
		log.Printf("경고: CRE 설정 실패 - %v\n", err)
	}

	// 블록체인 이벤트 리스너 시작
	go blockchain.StartEventListener()

	// Gin 라우터 설정
	r := gin.Default()
	r.POST("/api/generate", handlers.Generate)
	r.GET("/api/status/:agentId", handlers.Status)
	r.GET("/api/jobs/:jobId/stream", handlers.StreamJob)  // SSE 스트리밍
	r.GET("/api/artifacts/:jobId", handlers.GetArtifact)  // 이미지/비디오 프록시

	r.Run(":8081") //여기서 라우터 매칭 함수들 매 연결마다 go루틴됨!!!
}
