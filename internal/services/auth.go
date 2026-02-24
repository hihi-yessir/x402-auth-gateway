// JWT 생성 -> CRE 호출
package service

import (
	"bytes"
	"crypto/ecdsa"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/ethereum/go-ethereum/crypto"
)

// CRE 설정
var (
	creGatewayURL string
	workflowID    string
	privateKey    *ecdsa.PrivateKey
	signerAddress string
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

	//주소 추출
	publicKey := privateKey.Public().(*ecdsa.PublicKey)
	signerAddress = crypto.PubkeyToAddress(*publicKey).Hex()

	return nil
}

// 요청 구조체
type CRERequest struct {
	JSONRPC string    `json:"jsonrpc"`
	Method  string    `json:"method"`
	Params  CREParams `json:"params"`
	ID      int       `json:"id"`
}

type CREParams struct {
	WorkflowID string   `json:"workflow_id"` ////배포된 CRE workflow 이름
	Input      CREInput `json:"input"`
	Signature  string   `json:"signature"`
	Signer     string   `json:"signer"`
}

type CREInput struct {
	AgentID uint64 `json:"agentId"`
}

// 서명 생성 함수
func signCRERequest(agentId uint64) (string, error) {
	// 1. CRE가 검증하는 것과 똑같이 input을 JSON으로 직렬화
	input := CREInput{AgentID: agentId}
	message, err := json.Marshal(input)
	if err != nil {
		return "", fmt.Errorf("input 직렬화 실패: %v", err)
	}

	// 2. Ethereum prefix 추가 (CRE와 동일하게!)
	prefix := fmt.Sprintf("\x19Ethereum Signed Message:\n%d", len(message))
	hash := crypto.Keccak256Hash([]byte(prefix + string(message)))

	// 3. 서명
	sig, err := crypto.Sign(hash.Bytes(), privateKey)
	if err != nil {
		return "", err
	}

	// 4. v 값 조정 (0,1 → 27,28)
	sig[64] += 27

	return "0x" + fmt.Sprintf("%x", sig), nil
}

// CRE 호출 (JSON-RPC 2.0)
func CallCRE(agentId string) error {
	//agentId string -> uint64 형변환
	agentIdNum, err := strconv.ParseUint(agentId, 10, 64)
	if err != nil {
		return fmt.Errorf("agentId 변환 실패: %v", err)
	}

	//서명 생성
	signature, err := signCRERequest(agentIdNum)
	if err != nil {
		return fmt.Errorf("서명 생성 실패: %v", err)
	}

	req := CRERequest{
		JSONRPC: "2.0",
		Method:  "workflow_execute",
		Params: CREParams{
			WorkflowID: workflowID,
			Input: CREInput{
				AgentID: agentIdNum,
			},
			Signature: signature,
			Signer:    signerAddress,
		},
		ID: 1,
	}

	body, _ := json.Marshal(req)

	httpReq, err := http.NewRequest("POST", creGatewayURL+"/trigger", bytes.NewBuffer(body))
	if err != nil {
		return err
	}

	httpReq.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(httpReq)
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
