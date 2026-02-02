package main

import (
	"crypto/rand"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

func CreateUUID() (uuid string) {
	u := new([16]byte)
	_, err := rand.Read(u[:])
	if err != nil {
		//TODO log 기록
		fmt.Println("Cannot generate UUID : ", err)
	}

	// 0x40 is reserved variant from RFC 4122
	u[8] = (u[8] | 0x40) & 0x7F
	// Set the four most significant bits (bits 12 through 15) of the
	// time_hi_and_version field to the 4-bit version number.
	u[6] = (u[6] & 0xF) | (0x4 << 4)
	uuid = fmt.Sprintf("%x-%x-%x-%x-%x", u[0:4], u[4:6], u[6:8], u[8:10], u[10:])
	return
}

/*
보안을 위해, m3u8 재작성 및 토큰 매핑 방식 구현

m3u8안에는 짧은 안전한 경로만.
프록시 서버가 알아서 외부 URL 매핑 및 전달
외부 서버 정보/쿼리 스트링 노출 되지않음
*/
func replaceM3U8(queryParams url.Values, body string) string {
	streamID := queryParams.Get("streamID")
	channelID := queryParams.Get("channel")
	fileName := queryParams.Get("m3u8File")

	date := ""
	dateStr := fileName[:8] // YYYYMMDD
	if len(dateStr) == 8 {
		date = dateStr[:4] + "-" + dateStr[4:6] + "-" + dateStr[6:8]
	}

	// base := getMediaServerClient().baseURL

	lines := strings.Split(body, "\n")
	var newLines []string

	for _, line := range lines {
		if strings.HasSuffix(line, ".ts") {
			// 절대경로/상대경로 처리
			segURLStr := fmt.Sprintf("/recordings/%s/%s/%s/%s", date, streamID, channelID, line)

			// 토큰 생성 및 저장
			token := makeToken(segURLStr)
			core2UrlMapMutex.Lock()
			core2UrlMap[token] = segURLStr
			core2UrlMapMutex.Unlock()

			// 프록시 경로로 교체
			proxyLine := fmt.Sprintf("/proxy/core/ts?token=%s", token)
			newLines = append(newLines, proxyLine)
		} else if strings.Contains(line, "#EXT-X-KEY:METHOD=AES-128,URI=") {

			// 원본: #EXT-X-KEY:METHOD=AES-128,URI="/stream/1a481721-c99a-4fec-5d6d-93bec9a642dc/channel/0/recording/key",IV=0x2be3eb71313e490b3df724b34be3bd57
			re := regexp.MustCompile(`URI="([^"]*)"`)
			matches := re.FindStringSubmatch(line)

			if len(matches) > 1 {
				keyFileUri := matches[1] // 그룹1만 뽑으면 /stream/1a481721-c99a-4fec-5d6d-93bec9a642dc/channel/0/recording/key
				segURLStr := fmt.Sprintf("%s", keyFileUri)
				token := makeToken(segURLStr)

				core2UrlMapMutex.Lock()
				core2UrlMap[token] = segURLStr
				core2UrlMapMutex.Unlock()

				// 프록시 경로로 교체
				proxyLine := fmt.Sprintf("/proxy/core/key?token=%s", token)
				newLine := re.ReplaceAllString(line, `URI="`+proxyLine+`"`)
				newLines = append(newLines, newLine)
			}

		} else {
			newLines = append(newLines, line)
		}
	}

	return strings.Join(newLines, "\n")
}

// 원본 URL 토큰 변환
func makeToken(rawURL string) string {
	h := sha1.Sum([]byte(rawURL))
	return hex.EncodeToString(h[:8])
}
