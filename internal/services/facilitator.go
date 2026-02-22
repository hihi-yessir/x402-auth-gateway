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
	var payloadObj interface{}
	var requirementsObj interface{}
	json.Unmarshal(paymentSignature, &payloadObj)
	json.Unmarshal(paymentRequired, &requirementsObj)

	reqBody := map[string]interface{}{
		"paymentPayload":      payloadObj,
		"paymentRequirements": requirementsObj,
	}
	body, _ := json.Marshal(reqBody)
	fmt.Printf("Facilitator 요청: %s\n", string(body))

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
	var payloadObj interface{}
	var requirementsObj interface{}

	if err := json.Unmarshal(paymentSignature, &payloadObj); err != nil {
		return "", fmt.Errorf("paymentPayload 파싱 실패: %v", err)
	}
	if err := json.Unmarshal(paymentRequired, &requirementsObj); err != nil {
		return "", fmt.Errorf("paymentRequirements 파싱 실패: %v", err)
	}

	reqBody := map[string]interface{}{
		"paymentPayload":      payloadObj,
		"paymentRequirements": requirementsObj,
	}
	body, _ := json.Marshal(reqBody)

	resp, err := http.Post(
		getFacilitatorURL()+"/settle",
		"application/json",
		bytes.NewBuffer(body),
	)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var result struct {
		TxHash string `json:"txHash"`
	}
	json.NewDecoder(resp.Body).Decode(&result)
	return result.TxHash, nil
}

// 환불 (필요 시)
func Refund(paymentSignature []byte, paymentRequired []byte) (string, error) {
	var payloadObj interface{}
	var requirementsObj interface{}

	if err := json.Unmarshal(paymentSignature, &payloadObj); err != nil {
		return "", fmt.Errorf("paymentPayload 파싱 실패: %v", err)
	}
	if err := json.Unmarshal(paymentRequired, &requirementsObj); err != nil {
		return "", fmt.Errorf("paymentRequirements 파싱 실패: %v", err)
	}

	reqBody := map[string]interface{}{
		"paymentPayload":      payloadObj,
		"paymentRequirements": requirementsObj,
	}
	body, _ := json.Marshal(reqBody)

	resp, err := http.Post(
		getFacilitatorURL()+"/refund",
		"application/json",
		bytes.NewBuffer(body),
	)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var result struct {
		TxHash string `json:"txHash"`
	}
	json.NewDecoder(resp.Body).Decode(&result)
	return result.TxHash, nil
}
