package main

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync"
	"time"
)

// MediaServerClient MediaServer API 클라이언트
type MediaServerClient struct {
	baseURL    string
	httpClient *http.Client
}

var mediaServerClient *MediaServerClient
var clientOnce sync.Once
var core2UrlMap map[string]string // hls 녹화 경로 토큰화
var core2UrlMapMutex sync.Mutex

type WRRequestBody struct {
	Name     string             `json:"name"`
	Channels map[string]Channel `json:"channels"`
}

type Channel struct {
	Name               string `json:"name"`
	URL                string `json:"url"`
	OnDemand           bool   `json:"on_demand"`
	Status             int    `json:"status"`
	InsecureSkipVerify bool   `json:"insecure_skip_verify"`
	Audio              bool   `json:"audio"`
	OnRecording        bool   `json:"on_recording"`
}

var globalClientPool = sync.Pool{
	New: func() interface{} {
		transport := &http.Transport{
			MaxIdleConns:        200,                                   // 최대 유휴 연결 수
			IdleConnTimeout:     60 * time.Second,                      // 유휴 연결 타임아웃
			TLSHandshakeTimeout: 10 * time.Second,                      // TLS 핸드셰이크 타임아웃
			TLSClientConfig:     &tls.Config{InsecureSkipVerify: true}, // MediaServer가 자체인증서라 필요....
		}
		return &http.Client{
			Transport: transport,
			Timeout:   10 * time.Second, // 요청 타임아웃
		}
	},
}

// getMediaServerClient MediaServer 클라이언트 싱글톤 반환
func getMediaServerClient() *MediaServerClient {
	clientOnce.Do(func() {
		baseURL := fmt.Sprintf("https://%s:%d", appConfig.Server.MediaServer.Address, appConfig.Server.MediaServer.Port)
		mediaServerClient = &MediaServerClient{
			baseURL:    baseURL,
			httpClient: globalClientPool.Get().(*http.Client),
		}
		core2UrlMap = make(map[string]string)
	})
	return mediaServerClient
}

// request 공통 HTTP 요청 함수
// queryParams가 nil이 아니면 쿼리 파라미터를 URL에 추가합니다
func (client *MediaServerClient) request(method, path string, body interface{}, queryParams ...url.Values) (*http.Response, error) {
	var reqBody io.Reader
	if body != nil {
		jsonData, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		reqBody = bytes.NewBuffer(jsonData)
	}

	requestURL := client.baseURL + path

	// 쿼리 파라미터가 있으면 추가
	if len(queryParams) > 0 && queryParams[0] != nil {
		parsedURL, err := url.Parse(requestURL)
		if err != nil {
			return nil, fmt.Errorf("failed to parse URL: %w", err)
		}
		parsedURL.RawQuery = queryParams[0].Encode()
		requestURL = parsedURL.String()
	}

	req, err := http.NewRequest(method, requestURL, reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	apiClient := globalClientPool.Get().(*http.Client)
	defer globalClientPool.Put(apiClient)

	resp, err := apiClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}

	return resp, nil
}

// streamEditReq 스트림 추가/수정/삭제 요청
func streamEditReq(stream Stream, methodFlag int) (result bool) {
	client := getMediaServerClient()

	body := WRRequestBody{
		Name: stream.Name,
		Channels: map[string]Channel{
			"0": {
				Name:        "ch1",
				URL:         stream.RtspURL,
				OnDemand:    true,
				OnRecording: stream.Recording,
			},
		},
	}

	var path string
	var method string
	var reqBody interface{}

	switch methodFlag {
	case METHOD_ADD:
		path = fmt.Sprintf("/stream/%s/add", stream.StreamID)
		method = "POST"
		reqBody = body
	case METHOD_EDIT:
		path = fmt.Sprintf("/stream/%s/edit", stream.StreamID)
		method = "POST"
		reqBody = body
	case METHOD_DELETE:
		path = fmt.Sprintf("/stream/%s/delete", stream.StreamID)
		method = "GET"
		reqBody = nil
	default:
		log.Printf("Invalid method flag: %d", methodFlag)
		return false
	}

	resp, err := client.request(method, path, reqBody)
	if err != nil {
		log.Printf("Request failed: %v", err)
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		dump2, _ := httputil.DumpResponse(resp, true)
		log.Printf("Response error (status %d): %s", resp.StatusCode, string(dump2))
		return false
	}

	responseData, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Failed to read response body: %v", err)
		return false
	}

	var objmap map[string]interface{}
	if err := json.Unmarshal(responseData, &objmap); err != nil {
		log.Printf("Failed to unmarshal response: %v", err)
		return false
	}

	if status, exist := objmap["payload"]; exist {
		if statusStr, ok := status.(string); ok && statusStr == "success" {
			return true
		}
	}

	return false
}

// recordingReq Recording 시작/중지 요청
func recordingReq(streamIds []string, actionFlag bool) (result bool) {
	client := getMediaServerClient()

	var action string
	if actionFlag {
		action = "start"
	} else {
		action = "stop"
	}

	body := map[string]interface{}{
		"streamIds": streamIds,
	}

	path := fmt.Sprintf("/streams/recording/%s", action)
	resp, err := client.request("POST", path, body)
	if err != nil {
		log.Printf("Recording request failed: %v", err)
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		dump2, _ := httputil.DumpResponse(resp, true)
		log.Printf("Response error (status %d): %s", resp.StatusCode, string(dump2))
		return false
	}

	responseData, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Failed to read response body: %v", err)
		return false
	}

	var objmap map[string]interface{}
	if err := json.Unmarshal(responseData, &objmap); err != nil {
		log.Printf("Failed to unmarshal response: %v", err)
		return false
	}

	payload, ok := objmap["payload"].(map[string]interface{})
	if !ok {
		return false
	}

	successCnt, exist := payload["success"]
	if !exist {
		return false
	}

	// JSON 숫자는 float64로 파싱됨
	if cnt, ok := successCnt.(float64); ok && int(cnt) == len(streamIds) {
		return true
	}

	return false
}
