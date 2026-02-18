// POST api/generate
package handlers

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"

	"github.com/gin-gonic/gin"
	service "github.com/hihi-yessir/auth-os-gateway/internal/services"
)

type PaymentRequirements struct {
	X402Version int             `json:"x402Version"`
	Accepts     []PaymentOption `json:"accepts"`
}

type PaymentOption struct {
	Scheme         string `json:"scheme"`
	Network        string `json:"network"`
	Asset          string `json:"asset"`
	PayTo          string `json:"payTo"`
	AmountRequired string `json:"amountRequired"`
	Description    string `json:"description"`
	Resource       string `json:"resource"`
}

type PaymentPayload struct {
	Signature string         `json:"signature"`
	Payload   PaymentDetails `json:"payload"`
}

type PaymentDetails struct {
	From  string `json:"from"`
	To    string `json:"to"`
	Value string `json:"value"`
	Nonce string `json:"nonce"`
}

func getPaymentRequirements() PaymentRequirements {
	return PaymentRequirements{
		X402Version: 1,
		Accepts: []PaymentOption{
			{
				Scheme:         "exact",
				Network:        os.Getenv("PAYMENT_NETWORK"),
				Asset:          os.Getenv("PAYMENT_ASSET"),
				PayTo:          os.Getenv("PAYMENT_PAY_TO"),
				AmountRequired: os.Getenv("PAYMENT_AMOUNT"), // 0.5 USDC
				Description:    "AI 에이전트 이미지 생성 비용",
				Resource:       "/api/generate",
			},
		},
	}
}

func Generate(c *gin.Context) {
	payment := c.GetHeader("X-PAYMENT")
	// paymentSig 없으면 -> 402 반환
	if payment == "" {
		c.Header("PAYMENT-REQUIRED", "true")
		c.JSON(402, getPaymentRequirements())
		return
	}

	//hedaer X-PAYMENT 있으면 -> 파싱 -> 검증 후 JWT 생성
	decoded, err := base64.StdEncoding.DecodeString(payment)
	if err != nil {
		c.JSON(400, gin.H{"error": "잘못된 X-PAYMENT 형식"})
		return
	}
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

	//hold 확인 후 결제 데이터 저장 (Settle 시 사용)
	agentId := paymentData.Payload.From
	service.PendingPayments.Store(agentId, service.PaymentContext{
		Signature: decoded,
		Required:  requiredBytes,
		AgentID:   agentId,
	})

	/// JWT 생성 -> CRE 호출 (JWT 전달)
	token, err := service.CreateJWT(agentId)
	if err != nil {
		c.JSON(500, gin.H{"error": "JWT 생성 실패"})
		return
	}
	//CRE 호출
	go func() {
		err := service.CallCRE(token, agentId)
		if err != nil {
			fmt.Printf("CRE 호출 실패: %v\n", err)
		}
	}()
	c.JSON(202, gin.H{
		"message": "processing",
		"agentId": agentId,
	})
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
