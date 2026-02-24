package service

import (
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
)

// HealthCheck 핸들러
func HealthCheck(c *gin.Context) {
	c.JSON(200, gin.H{"status": "ok"})
}

// StartSelfPing - 서버 잠들기 방지 (9분마다)
func StartSelfPing() {
	serverURL := os.Getenv("SERVER_URL")
	if serverURL == "" {
		log.Println("SERVER_URL 없음 - self-ping 비활성화")
		return
	}

	ticker := time.NewTicker(9 * time.Minute)
	for range ticker.C {
		resp, err := http.Get(serverURL + "/health_check")
		if err != nil {
			log.Printf("Self-ping 실패: %v\n", err)
		} else {
			resp.Body.Close()
			log.Println("Self-ping OK")
		}
	}
}
