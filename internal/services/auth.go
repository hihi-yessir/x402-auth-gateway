// JWT 생성 -> CRE 호출
package service

import (
	"bytes"
	"crypto/ecdsa"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/golang-jwt/jwt/v5"
)

// CRE 설정
var (
	creGatewayURL string
	workflowID    string
	privateKey    *ecdsa.PrivateKey
)

// 초기화 (서버 시작 시 호출)
func InitCREConfig() error {
	creGatewayURL = os.Getenv("CRE_GATEWAY_URL")
	workflowID = os.Getenv("CRE_WORKFLOW_ID")
	keyHex := os.Getenv("GATEWAY_PRIVATE_KEY")

	if creGatewayURL == "" || workflowID == "" || keyHex == "" {
		return fmt.Errorf("CRE 환경변수 누락")
	}

	var err error
	privateKey, err = crypto.HexToECDSA(keyHex)
	if err != nil {
		return fmt.Errorf("Private Key 파싱 실패: %v", err)
	}

	return nil
}

// ECDSA JWT 생성 (CRE 공식 스펙)
func CreateJWT(agentId string) (string, error) {
	claims := jwt.MapClaims{
		"agentId": agentId,
		"iat":     time.Now().Unix(),
		"exp":     time.Now().Add(time.Hour).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodES256, claims)
	return token.SignedString(privateKey)
}

// JSON-RPC 요청 구조체
type JSONRPCRequest struct {
	JSONRPC string    `json:"jsonrpc"`
	Method  string    `json:"method"`
	Params  CREParams `json:"params"`
}

type CREParams struct {
	WorkflowID string   `json:"workflowID"`
	Input      CREInput `json:"input"`
}

type CREInput struct {
	AgentID         string `json:"agentId"`
	PaymentVerified bool   `json:"paymentVerified"`
}

// CRE 호출 (JSON-RPC 2.0)
func CallCRE(token string, agentId string) error {
	reqBody := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      fmt.Sprintf("req-%d", time.Now().Unix()),
		"method":  "workflows.execute",
		"params": map[string]interface{}{
			"workflow": map[string]string{
				"workflowID": workflowID,
			},
			"input": map[string]string{
				"agentId": agentId,
			},
		},
	}
	body, _ := json.Marshal(reqBody)

	req, err := http.NewRequest("POST", creGatewayURL, bytes.NewBuffer(body))
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 && resp.StatusCode != 202 {
		return fmt.Errorf("CRE 호출 실패: %d", resp.StatusCode)
	}

	fmt.Printf("CRE 호출 성공: agentId=%s\n", agentId)
	return nil
}
