package main

import (
	"bytes"
	"crypto/rand"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
	"golang.org/x/net/html"
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

// HTML에 프록시 스크립트 주입 및 리소스 경로 변경
func injectProxyScriptAndRewriteHTML(htmlContent string, c *gin.Context) (string, error) {
	doc, err := html.Parse(strings.NewReader(htmlContent))
	if err != nil {
		return "", err
	}

	proxyBaseURL := c.Request.URL.Scheme + "://" + c.Request.Host
	targetOrigin := getMediaServerClient().baseURL

	// <head> 태그 찾아서 스크립트 주입
	var injectScript func(*html.Node) bool
	injectScript = func(n *html.Node) bool {
		if n.Type == html.ElementNode && n.Data == "head" {
			// <script> 노드를 직접 생성
			scriptNode := &html.Node{
				Type: html.ElementNode,
				Data: "script",
			}

			// 스크립트 내용을 텍스트 노드로 추가
			scriptText := &html.Node{
				Type: html.TextNode,
				Data: fmt.Sprintf(`
(function() {
    'use strict';
    
    const PROXY_BASE = '%s';
    const TARGET_ORIGIN = '%s';
    
    // URL을 프록시 URL로 변환하는 헬퍼 함수
    function toProxyURL(url) {
        if (!url) return url;
        
        // 이미 프록시 URL인 경우
        if (url.includes('/proxy/resource?url=') || url.includes('/proxy/html?url=')) {
            return url;
        }
        
        // data:, blob: URL은 그대로
        if (url.startsWith('data:') || url.startsWith('blob:')) {
            return url;
        }
        
        // 절대 URL로 변환
        let absoluteURL;
        try {
            if (url.startsWith('http://') || url.startsWith('https://')) {
                absoluteURL = url;
            } else if (url.startsWith('//')) {
                absoluteURL = TARGET_ORIGIN.split(':')[0] + ':' + url;
            } else if (url.startsWith('/')) {
                absoluteURL = TARGET_ORIGIN + url;
            } else {
                absoluteURL = new URL(url, TARGET_ORIGIN).href;
            }
        } catch (e) {
            console.warn('[Proxy] Failed to parse URL:', url, e);
            return url;
        }
        
        // 프록시 URL로 변환
        return PROXY_BASE + '/proxy/resource?url=' + encodeURIComponent(absoluteURL);
    }
    
    // 1. fetch API 오버라이드
    const originalFetch = window.fetch;
    window.fetch = function(url, options) {
        const proxyURL = toProxyURL(url);
        return originalFetch(proxyURL, options);
    };
    
    // 2. XMLHttpRequest 오버라이드
    const originalOpen = XMLHttpRequest.prototype.open;
    XMLHttpRequest.prototype.open = function(method, url, ...rest) {
        const proxyURL = toProxyURL(url);
        return originalOpen.call(this, method, proxyURL, ...rest);
    };
    
    // 3. WebSocket 오버라이드
    const originalWebSocket = window.WebSocket;
    window.WebSocket = function(url, protocols) {
        let wsURL = url;
        if (wsURL.startsWith('ws://') || wsURL.startsWith('wss://')) {
            const httpURL = wsURL.replace(/^ws/, 'http');
            wsURL = PROXY_BASE + '/proxy/websocket?url=' + encodeURIComponent(httpURL);
        }
        return new originalWebSocket(wsURL, protocols);
    };
    
    // 4. 동적 이미지 로드 오버라이드
    const originalImageSrc = Object.getOwnPropertyDescriptor(HTMLImageElement.prototype, 'src');
    if (originalImageSrc && originalImageSrc.set) {
        Object.defineProperty(HTMLImageElement.prototype, 'src', {
            set: function(value) {
                const proxyURL = toProxyURL(value);
                originalImageSrc.set.call(this, proxyURL);
            },
            get: function() {
                return originalImageSrc.get.call(this);
            }
        });
    }
    
    // 5. 동적 스크립트 로드 오버라이드
    const originalScriptSrc = Object.getOwnPropertyDescriptor(HTMLScriptElement.prototype, 'src');
    if (originalScriptSrc && originalScriptSrc.set) {
        Object.defineProperty(HTMLScriptElement.prototype, 'src', {
            set: function(value) {
                const proxyURL = toProxyURL(value);
                originalScriptSrc.set.call(this, proxyURL);
            },
            get: function() {
                return originalScriptSrc.get.call(this);
            }
        });
    }
    
    // 6. 동적 링크(CSS) 로드 오버라이드
    const originalLinkHref = Object.getOwnPropertyDescriptor(HTMLLinkElement.prototype, 'href');
    if (originalLinkHref && originalLinkHref.set) {
        Object.defineProperty(HTMLLinkElement.prototype, 'href', {
            set: function(value) {
                const proxyURL = toProxyURL(value);
                originalLinkHref.set.call(this, proxyURL);
            },
            get: function() {
                return originalLinkHref.get.call(this);
            }
        });
    }
    
    // 7. window.open 오버라이드
    const originalWindowOpen = window.open;
    window.open = function(url, ...rest) {
        if (url) {
            const proxyURL = PROXY_BASE + '/proxy/html?url=' + encodeURIComponent(toProxyURL(url));
            return originalWindowOpen(proxyURL, ...rest);
        }
        return originalWindowOpen(url, ...rest);
    };


	// ★★★ 8. Form Submit 이벤트 가로채기 (NEW!) ★★★
    document.addEventListener('submit', function(e) {
        const form = e.target;
        if (form && form.action) {
            const originalAction = form.action;
            
            // 이미 프록시 URL인 경우 스킵
            if (originalAction.includes('/proxy/resource?url=')) {
                return;
            }
            
            // 프록시 URL로 변경
            const proxyAction = toProxyURL(originalAction);
            form.action = proxyAction;
        }
    }, true); // capture phase에서 처리
    
    // ★★★ 9. 동적으로 생성되는 form의 action 속성 감시 (NEW!) ★★★
    const originalFormAction = Object.getOwnPropertyDescriptor(HTMLFormElement.prototype, 'action');
    if (originalFormAction && originalFormAction.set) {
        Object.defineProperty(HTMLFormElement.prototype, 'action', {
            set: function(value) {
                const proxyURL = toProxyURL(value);
                originalFormAction.set.call(this, proxyURL);
            },
            get: function() {
                return originalFormAction.get.call(this);
            }
        });
    }
    
    // ★★★ 10. 페이지 로드 시 기존 form들의 action 변경 (NEW!) ★★★
    window.addEventListener('DOMContentLoaded', function() {
        const forms = document.querySelectorAll('form[action]');
        forms.forEach(function(form) {
            const originalAction = form.getAttribute('action');
            if (originalAction && !originalAction.includes('/proxy/resource?url=')) {
                const proxyAction = toProxyURL(originalAction);
                form.setAttribute('action', proxyAction);
            }
        });
    });
    
})();
`, proxyBaseURL, targetOrigin),
			}

			scriptNode.AppendChild(scriptText)

			// <head>의 가장 앞에 삽입 (다른 스크립트보다 먼저 실행되도록)
			if n.FirstChild != nil {
				n.InsertBefore(scriptNode, n.FirstChild)
			} else {
				n.AppendChild(scriptNode)
			}

			return true
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			if injectScript(child) {
				return true
			}
		}
		return false
	}

	injectScript(doc)

	// 기존 HTML 태그의 리소스 경로 변경
	var rewrite func(*html.Node)
	rewrite = func(n *html.Node) {
		if n.Type == html.ElementNode {
			// 다양한 태그의 리소스 경로 변경
			attrs := map[string][]string{
				"link":   {"href"},
				"script": {"src"},
				"img":    {"src", "srcset"},
				"iframe": {"src"},
				"video":  {"src", "poster"},
				"audio":  {"src"},
				"source": {"src", "srcset"},
				"embed":  {"src"},
				"object": {"data"},
				"form":   {"action"}, // form 추가
			}

			if attrNames, exists := attrs[n.Data]; exists {
				for i, attr := range n.Attr {
					for _, attrName := range attrNames {
						if attr.Key == attrName && attr.Val != "" {
							// 절대 URL로 변환
							absoluteURL := resolveURL(targetOrigin, attr.Val)
							// 프록시 URL로 변경
							if absoluteURL != "" && !strings.HasPrefix(attr.Val, "data:") {
								n.Attr[i].Val = fmt.Sprintf("%s/proxy/resource?url=%s",
									proxyBaseURL,
									url.QueryEscape(absoluteURL))
							}
						}
					}
				}
			}

			// style 속성의 url() 처리
			for i, attr := range n.Attr {
				if attr.Key == "style" {
					n.Attr[i].Val = rewriteCSSURLs(attr.Val, c)
				}
			}
		}

		// <style> 태그 내용 처리
		if n.Type == html.ElementNode && n.Data == "style" {
			if n.FirstChild != nil && n.FirstChild.Type == html.TextNode {
				n.FirstChild.Data = rewriteCSSURLs(n.FirstChild.Data, c)
			}
		}

		for child := n.FirstChild; child != nil; child = child.NextSibling {
			rewrite(child)
		}
	}

	rewrite(doc)

	// HTML을 문자열로 변환
	var buf bytes.Buffer
	err = html.Render(&buf, doc)
	if err != nil {
		return "", err
	}

	return buf.String(), nil
}

func rewriteCSSURLs(cssContent string, c *gin.Context) string {
	proxyBaseURL := c.Request.URL.Scheme + "://" + c.Request.Host

	baseURL := getMediaServerClient().baseURL

	// 세 가지 패턴을 각각 처리
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`url\(\s*"([^"]+)"\s*\)`),      // url("...")
		regexp.MustCompile(`url\(\s*'([^']+)'\s*\)`),      // url('...')
		regexp.MustCompile(`url\(\s*([^'")][^\)]*)\s*\)`), // url(...)
	}

	result := cssContent
	for _, pattern := range patterns {
		result = pattern.ReplaceAllStringFunc(result, func(match string) string {
			// URL 추출
			submatches := pattern.FindStringSubmatch(match)
			if len(submatches) < 2 {
				return match
			}

			originalURL := strings.TrimSpace(submatches[1])

			// data: URL이나 http로 시작하는 절대 경로는 그대로
			if strings.HasPrefix(originalURL, "data:") ||
				strings.HasPrefix(originalURL, "http://") ||
				strings.HasPrefix(originalURL, "https://") {
				return match
			}

			// 상대 URL을 절대 URL로 변환
			absoluteURL := resolveURL(baseURL, originalURL)
			if absoluteURL == "" {
				return match
			}

			// 프록시 URL로 변경
			proxyURL := fmt.Sprintf("%s/proxy/resource?url=%s",
				proxyBaseURL,
				url.QueryEscape(absoluteURL))

			// 원본과 동일한 인용부호 스타일 유지
			if strings.Contains(match, `"`) {
				return fmt.Sprintf(`url("%s")`, proxyURL)
			} else if strings.Contains(match, `'`) {
				return fmt.Sprintf(`url('%s')`, proxyURL)
			} else {
				return fmt.Sprintf(`url(%s)`, proxyURL)
			}
		})
	}

	return result
}

// 상대 URL을 절대 URL로 변환
func resolveURL(baseURLStr, ref string) string {
	// 이미 절대 URL인 경우
	if strings.HasPrefix(ref, "http://") || strings.HasPrefix(ref, "https://") {
		return ref
	}

	// baseURL을 파싱 (https://IP:port 형식 지원)
	baseURL, err := url.Parse(baseURLStr)
	if err != nil {
		return ""
	}

	// 프로토콜 상대 URL (//example.com/path)
	if strings.HasPrefix(ref, "//") {
		return baseURL.Scheme + ":" + ref
	}

	// 상대 URL 처리
	refURL, err := url.Parse(ref)
	if err != nil {
		return ""
	}

	// baseURL을 기준으로 상대 URL을 절대 URL로 변환
	return baseURL.ResolveReference(refURL).String()
}
