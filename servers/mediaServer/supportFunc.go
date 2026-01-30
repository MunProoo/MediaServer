package main

import (
	"crypto/rand"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
)

// Default streams signals
const (
	SignalStreamRestart = iota ///< Y   Restart
	SignalStreamStop
	SignalStreamClient
)

// generateUUID function make random uuid for clients and stream
func generateUUID() (string, error) {
	b := make([]byte, 16)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%X-%X-%X-%X-%X", b[0:4], b[4:6], b[6:8], b[8:10], b[10:]), nil
}

// stringToInt convert string to int if err to zero
func stringToInt(val string) int {
	i, err := strconv.Atoi(val)
	if err != nil {
		return 0
	}
	return i
}

// stringInBetween fin char to char sub string
func stringInBetween(str string, start string, end string) (result string) {
	s := strings.Index(str, start)
	if s == -1 {
		return
	}
	str = str[s+len(start):]
	e := strings.Index(str, end)
	if e == -1 {
		return
	}
	str = str[:e]
	return str
}

// extractIPFromICEServer ICE 서버 URL에서 IP 주소 추출
// 예: "turn:14.14.14.2:8888" -> "14.14.14.2"
// 예: "turn:14.14.14.2:8888?transport=tcp" -> "14.14.14.2"
func extractIPFromICEServer(iceServer string) string {
	// URL 파싱 시도 (turn: 스킴을 http:로 임시 변경)
	iceServer = strings.Replace(iceServer, "turn:", "http://", 1)
	iceServer = strings.Replace(iceServer, "stun:", "http://", 1)

	parsedURL, err := url.Parse(iceServer)
	if err != nil {
		return ""
	}

	// 호스트에서 포트 제거
	host, _, err := net.SplitHostPort(parsedURL.Host)
	if err != nil {
		// 포트가 없는 경우 전체 호스트 반환
		return parsedURL.Host
	}

	return host
}

// extractFirstIPFromICEServers ICE 서버 목록에서 첫 번째 IP 주소 추출
func extractFirstIPFromICEServers(iceServers []string) string {
	if len(iceServers) == 0 {
		return ""
	}

	for _, server := range iceServers {
		if ip := extractIPFromICEServer(server); ip != "" {
			return ip
		}
	}

	return ""
}
