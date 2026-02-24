package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
)

type GenerationRequest struct {
	Prompt         string  `json:"prompt"`
	NegativePrompt string  `json:"negative_prompt,omitempty"`
	Width          int     `json:"width"`
	Height         int     `json:"height"`
	Steps          int     `json:"steps"`
	Cfg            float64 `json:"cfg"`
	Seed           int     `json:"seed,omitempty"`
	Frames         int     `json:"frames,omitempty"`
	Fps            int     `json:"fps,omitempty"`
}

type GenerationResponse struct {
	JobID   string `json:"job_id"`
	Status  string `json:"status"`
	PollURL string `json:"poll_url"`
}

func getBaseURL() string {
	url := os.Getenv("GENERATION_BASE_URL")
	if url == "" {
		return "https://demo-api.whitewall.network/v1"
	}
	return url
}

// GetBaseDomain - /v1 제거한 도메인 반환
func GetBaseDomain() string {
	baseURL := getBaseURL()
	// "https://demo-api.whitewall.network/v1" → "https://demo-api.whitewall.network"
	return baseURL[:len(baseURL)-3]
}

func GenerateContent(contentType string, prompt string) (*GenerationResponse, error) {
	apiKey := os.Getenv("GENERATION_API_KEY")
	baseURL := getBaseURL()

	var endpoint string
	var reqBody GenerationRequest

	if contentType == "image" {
		endpoint = baseURL + "/gen/image"
		reqBody = GenerationRequest{
			Prompt:         prompt,
			NegativePrompt: "blurry, low quality, distorted",
			Width:          1024,
			Height:         1024,
			Steps:          30,
			Cfg:            7.5,
		}
	} else if contentType == "video" {
		endpoint = baseURL + "/gen/video"
		reqBody = GenerationRequest{
			Prompt:         prompt,
			NegativePrompt: "blurry, distorted, jittery, low quality",
			Width:          512,
			Height:         320,
			Frames:         25,
			Steps:          20,
			Cfg:            4.0,
			Fps:            25,
		}
	} else {
		return nil, fmt.Errorf("지원하지 않는 타입: %s", contentType)
	}

	body, _ := json.Marshal(reqBody)

	req, _ := http.NewRequest("POST", endpoint, bytes.NewBuffer(body))
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result GenerationResponse
	json.NewDecoder(resp.Body).Decode(&result)

	return &result, nil
}

// Job 상태 응답 (WhiteWall API 스펙)
type JobStatusResponse struct {
	JobID       string `json:"job_id"`
	Type        string `json:"type"`                   // "image" or "video"
	Status      string `json:"status"`                 // queued, processing, completed, failed
	Params      any    `json:"params,omitempty"`
	CreatedAt   string `json:"created_at,omitempty"`
	StartedAt   string `json:"started_at,omitempty"`
	CompletedAt string `json:"completed_at,omitempty"`
	ArtifactURL string `json:"artifact_url,omitempty"` // 완료 시 결과물 경로
	Error       string `json:"error,omitempty"`        // 실패 시 에러 메시지
}

// Job 상태 폴링: GET /jobs/:id
func PollJobStatus(jobID string) (*JobStatusResponse, error) {
	apiKey := os.Getenv("GENERATION_API_KEY")
	pollURL := fmt.Sprintf("%s/jobs/%s", getBaseURL(), jobID)

	req, _ := http.NewRequest("GET", pollURL, nil)
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result JobStatusResponse
	json.NewDecoder(resp.Body).Decode(&result)

	return &result, nil
}

// Artifact 응답
type ArtifactResponse struct {
	Body        io.ReadCloser
	ContentType string
	StatusCode  int
}

// FetchArtifact - WhiteWall에서 artifact 가져오기 (프록시용)
func FetchArtifact(jobID string) (*ArtifactResponse, error) {
	apiKey := os.Getenv("GENERATION_API_KEY")
	artifactURL := fmt.Sprintf("%s/jobs/%s/artifact", getBaseURL(), jobID)

	req, _ := http.NewRequest("GET", artifactURL, nil)
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	// 주의: Body는 호출자가 닫아야 함!

	return &ArtifactResponse{
		Body:        resp.Body,
		ContentType: resp.Header.Get("Content-Type"),
		StatusCode:  resp.StatusCode,
	}, nil
}
