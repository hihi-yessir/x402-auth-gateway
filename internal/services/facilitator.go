package service

import (
	"bytes"
	"encoding/json"
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
	reqBody := map[string]interface{}{
		"paymentSignature": string(paymentSignature),
		"paymentRequired":  string(paymentRequired),
	}
	body, _ := json.Marshal(reqBody)

	resp, err := http.Post(
		getFacilitatorURL()+"/verify",
		"application/json",
		bytes.NewBuffer(body),
	)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	return resp.StatusCode == 200, nil
}

// 정산
func Settle(paymentSignature []byte, paymentRequired []byte) (string, error) {
	reqBody := map[string]interface{}{
		"paymentSignature": paymentSignature,
		"paymentRequired":  paymentRequired,
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
	reqBody := map[string]interface{}{
		"paymentSignature": paymentSignature,
		"paymentREquired":  paymentRequired,
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
