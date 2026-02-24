// POST api/generate
package handlers

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	service "github.com/hihi-yessir/auth-os-gateway/internal/services"
)

// 요청 바디 구조체
type GenerateRequest struct {
	AgentID string `json:"agentId"`
	Type    string `json:"type"` // "image" 또는 "video"
	Prompt  string `json:"prompt"`
}

type PaymentOption struct {
	Scheme            string `json:"scheme"`
	Network           string `json:"network"`
	Asset             string `json:"asset"`
	PayTo             string `json:"payTo"`
	Amount            string `json:"amount"` // x402 표준: "amount" (not "maxAmountRequired")
	MaxTimeoutSeconds int    `json:"maxTimeoutSeconds"`
	Extra             Extra  `json:"extra"`
}

type Extra struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type PaymentPayload struct {
	Signature string         `json:"signature"`
	Payload   PaymentDetails `json:"payload"`
}

type PaymentDetails struct {
	From  string `json:"from"` //agent wallet
	To    string `json:"to"`   //우리 주소
	Value string `json:"value"`
	Nonce string `json:"nonce"`
}

func getPaymentRequirements() PaymentOption {
	return PaymentOption{
		Scheme:            "exact",
		Network:           os.Getenv("PAYMENT_NETWORK"), // eip155:84532
		Asset:             os.Getenv("PAYMENT_ASSET"),
		PayTo:             os.Getenv("PAYMENT_PAY_TO"),
		Amount:            os.Getenv("PAYMENT_AMOUNT"), // 0.5 USDC = "500000"
		MaxTimeoutSeconds: 3600,
		Extra: Extra{
			Name:    "USDC",      // Circle 공식: "USDC"
			Version: "2",         // Base Sepolia USDC는 version 2!
		},
	}
}

func Generate(c *gin.Context) {
	// 결제 서명 헤더 확인 (v2: PAYMENT-SIGNATURE, v1: X-PAYMENT)
	payment := c.GetHeader("PAYMENT-SIGNATURE")
	if payment == "" {
		payment = c.GetHeader("X-PAYMENT") // v1 fallback
	}
	// paymentSig 없으면 -> 402 반환
	if payment == "" {
		option := getPaymentRequirements()
		// x402 v2 형식: PAYMENT-REQUIRED 헤더 사용
		requirements := map[string]interface{}{
			"x402Version": 2,
			"accepts":     []PaymentOption{option},
		}

		reqJSON, _ := json.Marshal(requirements)
		reqBase64 := base64.StdEncoding.EncodeToString(reqJSON)

		c.Header("PAYMENT-REQUIRED", reqBase64) // x402 v2 표준 헤더
		c.JSON(402, gin.H{})                    // body는 빈 객체
		return
	}

	//body parsing
	var req GenerateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "잘못된 요청 형식"})
		return
	}

	agentId := req.AgentID
	if agentId == "" {
		c.JSON(400, gin.H{"error": "agentId 필수"})
		return
	}

	//hedaer X-PAYMENT 있으면 -> 파싱 -> 검증 후 JWT 생성
	decoded, err := base64.StdEncoding.DecodeString(payment)
	if err != nil {
		c.JSON(400, gin.H{"error": "잘못된 X-PAYMENT 형식"})
		return
	}
	fmt.Printf("클라이언트가 보낸 X-PAYMENT: %s\n", string(decoded))

	var paymentData PaymentPayload
	if err := json.Unmarshal(decoded, &paymentData); err != nil {
		c.JSON(400, gin.H{"error": "X-PAYMENT 데이터 파싱 실패"})
		return
	}

	//파싱 성공 -> Facilitator검증
	paymentRequired := getPaymentRequirements()
	requiredBytes, _ := json.Marshal(paymentRequired)

	//facilitator hold 확인 -> 실패 시 400 반환
	valid, err := service.ValidatePayment(decoded, requiredBytes)
	if err != nil || !valid {
		c.JSON(400, gin.H{"error": "결제 검증 실패"})
		return
	}

	// 5. 결과 채널 생성
	resultCh := make(chan *service.EventResult, 1)
	// 6. hold 확인 후 PendingPayments에 저장 (채널 + type + prompt 포함!)
	service.PendingPayments.Store(agentId, service.PaymentContext{
		Signature: decoded,
		Required:  requiredBytes,
		Type:      req.Type,
		Prompt:    req.Prompt,
		ResultCh:  resultCh,
	})

	//CRE 호출
	go func() {
		err := service.CallCRE(agentId)
		if err != nil {
			fmt.Printf("CRE 호출 실패: %v\n", err)
			resultCh <- &service.EventResult{Granted: false, Error: err}
		}
	}()
	// 8. 이벤트 대기 (타임아웃 60초)
	select {
	case result := <-resultCh:
		if result.Granted {
			c.JSON(202, gin.H{
				"agentId": agentId,
				"jobId":   result.JobID,
				"txHash":  result.TxHash,
				"status":  "processing",
			})
		} else {
			errMsg := "Access denied"
			if result.Error != nil {
				errMsg = result.Error.Error()
			}
			c.JSON(403, gin.H{"error": errMsg})
		}
	case <-time.After(90 * time.Second):
		service.PendingPayments.Delete(agentId)
		c.JSON(408, gin.H{"error": "Timeout waiting for blockchain event"})
	}
}

func Status(c *gin.Context) {
	agentId := c.Param("agentId")

	// PendingPayments에서 조회
	if _, exists := service.PendingPayments.Load(agentId); exists {
		c.JSON(200, gin.H{
			"agentId": agentId,
			"status":  "pending", // 아직 처리 중
		})
		return
	}

	// 없으면 완료됐거나 없는 요청
	c.JSON(200, gin.H{
		"agentId": agentId,
		"status":  "completed or not found",
	})
}

// SSE 스트리밍 - Job 상태
func StreamJob(c *gin.Context) {
	jobID := c.Param("jobId")

	// SSE 헤더 설정
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Access-Control-Allow-Origin", "*")

	// 클라이언트 연결 확인용 채널
	clientGone := c.Request.Context().Done()

	// 폴링 루프 (2초 간격)
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	// 첫 번째 즉시 폴링
	sendStatus := func() bool {
		status, err := service.PollJobStatus(jobID)
		if err != nil {
			fmt.Fprintf(c.Writer, "data: {\"error\": \"%s\"}\n\n", err.Error())
			c.Writer.Flush()
			return false
		}

		// artifact_url이 있으면 전체 URL로 변환
		response := map[string]interface{}{
			"job_id": status.JobID,
			"type":   status.Type,
			"status": status.Status,
		}

		if status.Error != "" {
			response["error"] = status.Error
		}

		data, _ := json.Marshal(response)
		fmt.Fprintf(c.Writer, "data: %s\n\n", string(data))
		c.Writer.Flush()

		// 완료 또는 실패 시 true 반환 (종료)
		return status.Status == "completed" || status.Status == "failed"
	}

	// 첫 번째 즉시 전송
	if sendStatus() {
		return
	}

	for {
		select {
		case <-clientGone:
			return
		case <-ticker.C:
			if sendStatus() {
				return
			}
		}
	}
}

// Artifact 프록시 - WhiteWall에서 가져와서 프론트에 전달
func GetArtifact(c *gin.Context) {
	jobID := c.Param("jobId")

	// Service 레이어에서 artifact 가져오기
	artifact, err := service.FetchArtifact(jobID)
	if err != nil {
		c.JSON(500, gin.H{"error": "Failed to fetch artifact"})
		return
	}
	defer artifact.Body.Close()

	// WhiteWall 응답 그대로 전달 (image/png 또는 image/webp)
	c.Header("Content-Type", artifact.ContentType)
	c.Status(artifact.StatusCode)
	io.Copy(c.Writer, artifact.Body) // 파이프! 저장 없이 바로 전달
}
