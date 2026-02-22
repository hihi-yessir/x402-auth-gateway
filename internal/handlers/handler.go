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

type PaymentOption struct {
	Scheme            string `json:"scheme"`
	Network           string `json:"network"`
	Asset             string `json:"asset"`
	PayTo             string `json:"payTo"`
	MaxAmountRequired string `json:"maxAmountRequired"`
	MaxTimeoutSeconds int    `json:"maxTimeoutSeconds"`
	Description       string `json:"description"`
	Resource          string `json:"resource"`
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
		Network:           os.Getenv("PAYMENT_NETWORK"),
		Asset:             os.Getenv("PAYMENT_ASSET"),
		PayTo:             os.Getenv("PAYMENT_PAY_TO"),
		MaxAmountRequired: os.Getenv("PAYMENT_AMOUNT"), // 0.5 USDC
		MaxTimeoutSeconds: 3600,
		Description:       "AI 에이전트 이미지 생성 비용",
		Resource:          "/api/generate",
		Extra: Extra{
			Name:    "USD Coin",
			Version: "2",
		},
	}
}

func Generate(c *gin.Context) {
	payment := c.GetHeader("X-PAYMENT")
	// paymentSig 없으면 -> 402 반환
	if payment == "" {
		option := getPaymentRequirements()
		requirements := map[string]interface{}{
			"x402Version": 1,
			"accepts":     []PaymentOption{option},
		}
		reqJSON, _ := json.Marshal(requirements)

		// 헤더 이름을 X-Payment-Requirements로 변경 (표준 규격)
		c.Header("X-Payment-Requirements", base64.StdEncoding.EncodeToString(reqJSON))

		// 기존 X-PAYMENT 헤더도 유지 (하위 호환)
		c.Header("X-PAYMENT", base64.StdEncoding.EncodeToString(reqJSON))

		c.JSON(402, requirements)
		return
	}
	agentId := c.GetHeader("X-AGENT-ID")
	if agentId == "" {
		c.JSON(400, gin.H{"error": "X-AGENT-ID 헤더 필요"})
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

	//hold 확인 후 결제 데이터 저장 (Settle 시 사용)
	service.PendingPayments.Store(agentId, service.PaymentContext{
		Signature: decoded,
		Required:  requiredBytes,
	})

	//CRE 호출
	go func() {
		err := service.CallCRE(agentId)
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
