package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
)

func getFacilitatorURL() string {
	url := os.Getenv("FACILITATOR_URL")
	if url == "" {
		return "https://x402.org/facilitator" // 기본값
	}
	return url
}

// 결제 검증 요청
func ValidatePayment(paymentSignature []byte, paymentRequired []byte) (bool, error) {
	var payloadObj map[string]interface{}
	var requirementsObj interface{}
	json.Unmarshal(paymentSignature, &payloadObj)
	json.Unmarshal(paymentRequired, &requirementsObj)

	// x402Version 추출 (paymentPayload에서)
	x402Version := 2 // 기본값
	if v, ok := payloadObj["x402Version"]; ok {
		if vFloat, ok := v.(float64); ok {
			x402Version = int(vFloat)
		}
	}

	reqBody := map[string]interface{}{
		"x402Version":         x402Version, // 필수!
		"paymentPayload":      payloadObj,
		"paymentRequirements": requirementsObj,
	}
	body, _ := json.Marshal(reqBody)
	fmt.Printf("Facilitator Verify 요청: %s\n", string(body))

	resp, err := http.Post(
		getFacilitatorURL()+"/verify",
		"application/json",
		bytes.NewBuffer(body),
	)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	fmt.Printf("Facilitator 응답 (%d): %s\n", resp.StatusCode, string(respBody))

	return resp.StatusCode == 200, nil
}

// 정산
func Settle(paymentSignature []byte, paymentRequired []byte) (string, error) {
	var payloadObj map[string]interface{}
	var requirementsObj interface{}

	if err := json.Unmarshal(paymentSignature, &payloadObj); err != nil {
		return "", fmt.Errorf("paymentPayload 파싱 실패: %v", err)
	}
	if err := json.Unmarshal(paymentRequired, &requirementsObj); err != nil {
		return "", fmt.Errorf("paymentRequirements 파싱 실패: %v", err)
	}

	// x402Version 추출
	x402Version := 2
	if v, ok := payloadObj["x402Version"]; ok {
		if vFloat, ok := v.(float64); ok {
			x402Version = int(vFloat)
		}
	}

	reqBody := map[string]interface{}{
		"x402Version":         x402Version, // 필수!
		"paymentPayload":      payloadObj,
		"paymentRequirements": requirementsObj,
	}
	body, _ := json.Marshal(reqBody)

	fmt.Printf("Settle 요청 바디: %s\n", string(body))

	resp, err := http.Post(
		getFacilitatorURL()+"/settle",
		"application/json",
		bytes.NewBuffer(body),
	)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	// 🆕 로그 추가!
	respBody, _ := io.ReadAll(resp.Body)
	fmt.Printf("Settle 응답 (%d): %s\n", resp.StatusCode, string(respBody))

	var result struct {
		Success     bool   `json:"success"`
		ErrorReason string `json:"errorReason"`
		Transaction string `json:"transaction"` // txHash가 아니라 transaction!
		Payer       string `json:"payer"`
	}
	json.Unmarshal(respBody, &result)

	if !result.Success {
		return "", fmt.Errorf("settle 실패: %s", result.ErrorReason)
	}
	return result.Transaction, nil
}

// 환불 (필요 시)
func Refund(paymentSignature []byte, paymentRequired []byte) (string, error) {
	var payloadObj map[string]interface{}
	var requirementsObj interface{}

	if err := json.Unmarshal(paymentSignature, &payloadObj); err != nil {
		return "", fmt.Errorf("paymentPayload 파싱 실패: %v", err)
	}
	if err := json.Unmarshal(paymentRequired, &requirementsObj); err != nil {
		return "", fmt.Errorf("paymentRequirements 파싱 실패: %v", err)
	}

	// x402Version 추출
	x402Version := 2
	if v, ok := payloadObj["x402Version"]; ok {
		if vFloat, ok := v.(float64); ok {
			x402Version = int(vFloat)
		}
	}

	reqBody := map[string]interface{}{
		"x402Version":         x402Version,
		"paymentPayload":      payloadObj,
		"paymentRequirements": requirementsObj,
	}
	body, _ := json.Marshal(reqBody)

	fmt.Printf("Refund 요청 바디: %s\n", string(body))

	resp, err := http.Post(
		getFacilitatorURL()+"/refund",
		"application/json",
		bytes.NewBuffer(body),
	)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	fmt.Printf("Refund 응답 (%d): %s\n", resp.StatusCode, string(respBody))

	var result struct {
		Success     bool   `json:"success"`
		ErrorReason string `json:"errorReason"`
		Transaction string `json:"transaction"`
	}
	json.Unmarshal(respBody, &result)

	if !result.Success {
		return "", fmt.Errorf("refund 실패: %s", result.ErrorReason)
	}
	return result.Transaction, nil
}
