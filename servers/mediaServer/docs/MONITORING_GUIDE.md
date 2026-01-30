# MediaServer 모니터링 시스템 가이드

## 📊 개요

MediaServer에 실시간 모니터링 시스템이 추가되었습니다. 시스템 리소스, 스트림 상태, 클라이언트 연결 등을 실시간으로 모니터링할 수 있습니다.

## 🚀 주요 기능

### 1. 시스템 모니터링
- **CPU 사용률**: 실시간 CPU 사용률 및 코어 수
- **메모리 사용률**: 사용 중인 메모리 및 전체 메모리
- **디스크 사용률**: 디스크 여유 공간 및 사용률
- **네트워크 속도**: 초당 송수신 데이터량 (MB/s)
- **GoRoutines**: 실행 중인 고루틴 수

#### 네트워크 측정 방법
- 5초마다 시스템의 **모든 네트워크 인터페이스** 통계 수집
- 이전 측정값과 현재 측정값의 차이를 시간 간격으로 나누어 **초당 전송 속도** 계산
- 단위: MB/s (Megabytes per second)
- **모든 물리적 네트워크 인터페이스 합산** (Ethernet, WiFi, VPN 등)
- **loopback 인터페이스 제외** (localhost 통신은 제외)

**측정 대상:**
- ✅ 이더넷 (eth0, ens33 등)
- ✅ WiFi (wlan0, WiFi 등)
- ✅ VPN (tun0, vpn 등)
- ❌ Loopback (lo, lo0) - 제외

### 2. 스트림 모니터링
- 각 스트림의 온라인/오프라인 상태
- 채널별 클라이언트 연결 수
- 녹화 상태 실시간 표시
- 스트림 업타임

### 3. 경고 시스템
- CPU 사용률 75% 이상: 경고
- CPU 사용률 90% 이상: 위험
- 메모리 사용률 80% 이상: 경고
- 메모리 사용률 90% 이상: 위험
- 디스크 사용률 80% 이상: 경고
- 디스크 사용률 90% 이상: 위험
- GoRoutines 5000개 이상: 경고
- GoRoutines 10000개 이상: 위험

### 4. 히스토리 차트
- 최근 1시간의 CPU 사용률 그래프
- 최근 1시간의 메모리 사용률 그래프
- 최근 1시간의 네트워크 사용량 그래프

## 🌐 접속 방법

### 웹 대시보드
```
http://your-server-ip:8083/monitoring/dashboard
```

예시:
```
http://localhost:8083/monitoring/dashboard
```

## 📡 API 엔드포인트

### 1. 전체 모니터링 데이터
```
GET /api/monitoring/data
GET /api/monitoring/data?history=true  (히스토리 포함)
```

**응답 예시:**
```json
{
  "system": {
    "cpu_usage_percent": 45.2,
    "cpu_cores": 8,
    "memory_used_mb": 4096,
    "memory_total_mb": 16384,
    "memory_usage_percent": 25.0,
    "network_sent_mbps": 125.5,
    "network_recv_mbps": 89.3,
    "goroutines": 156,
    "uptime": "2일 5시간 30분",
    "uptime_seconds": 192600
  },
  "streams": [...],
  "summary": {
    "total_streams": 10,
    "online_streams": 8,
    "offline_streams": 2,
    "total_channels": 10,
    "total_clients": 15,
    "recording_channels": 3
  },
  "alerts": [...]
}
```

### 2. 시스템 메트릭만 조회
```
GET /api/monitoring/system
```

### 3. 스트림 메트릭만 조회
```
GET /api/monitoring/streams
```

### 4. 특정 스트림 조회
```
GET /api/monitoring/stream/:uuid
```

### 5. 경고 목록 조회
```
GET /api/monitoring/alerts
```

### 6. 히스토리 데이터 조회
```
GET /api/monitoring/history
```

### 7. 요약 통계
```
GET /api/monitoring/stats
```

## 🔧 설정

### 자동 업데이트 주기
모니터링 데이터는 다음 주기로 자동 수집됩니다:

- **메트릭 수집**: 5초마다
- **히스토리 저장**: 1분마다
- **웹 대시보드 업데이트**: 5초마다 (자동)

### 히스토리 데이터 보관
- 최대 60개 데이터 포인트 (1시간)
- 1분마다 1개 포인트 저장
- 메모리에만 저장 (재시작 시 초기화)

## 📈 사용 예시

### cURL을 사용한 API 호출
```bash
# 전체 모니터링 데이터 조회
curl http://localhost:8083/api/monitoring/data

# 히스토리 포함 조회
curl http://localhost:8083/api/monitoring/data?history=true

# 시스템 메트릭만 조회
curl http://localhost:8083/api/monitoring/system

# 경고 목록 조회
curl http://localhost:8083/api/monitoring/alerts
```

### JavaScript에서 사용
```javascript
// 모니터링 데이터 가져오기
async function getMonitoringData() {
    const response = await fetch('/api/monitoring/data?history=true');
    const data = await response.json();
    
    console.log('CPU 사용률:', data.system.cpu_usage_percent);
    console.log('온라인 스트림:', data.summary.online_streams);
    console.log('경고:', data.alerts);
}

// 5초마다 자동 업데이트
setInterval(getMonitoringData, 5000);
```

### Python에서 사용
```python
import requests
import time

def monitor_server():
    url = "http://localhost:8083/api/monitoring/data"
    
    while True:
        response = requests.get(url)
        data = response.json()
        
        cpu = data['system']['cpu_usage_percent']
        memory = data['system']['memory_usage_percent']
        
        print(f"CPU: {cpu}%, Memory: {memory}%")
        
        # 경고 확인
        if data['alerts']:
            for alert in data['alerts']:
                print(f"[{alert['level']}] {alert['message']}")
        
        time.sleep(5)

monitor_server()
```

## 🎨 대시보드 기능

### 실시간 메트릭 카드
- CPU, 메모리, 디스크, 네트워크 사용률을 시각적으로 표시
- 프로그레스 바로 사용률 표시
- 색상 코드:
  - 녹색: 0~50%
  - 파란색: 50~75%
  - 주황색: 75~90%
  - 빨간색: 90% 이상

### 스트림 상태 테이블
- 모든 스트림의 실시간 상태 표시
- 온라인/오프라인 배지
- 녹화 중인 채널 깜빡이는 표시
- 클라이언트 연결 수
- 업타임 표시

### 차트
- Chart.js를 사용한 실시간 라인 차트
- 최근 1시간의 데이터 표시
- 부드러운 애니메이션

## 🔔 경고 알림

경고는 다음 3가지 레벨로 분류됩니다:

### Info (정보)
- 일반적인 정보성 메시지
- 파란색 배경

### Warning (경고)
- 주의가 필요한 상황
- 주황색 배경
- 예: CPU 75% 이상, 메모리 80% 이상

### Critical (위험)
- 즉각적인 조치가 필요한 상황
- 빨간색 배경
- 예: CPU 90% 이상, 메모리 90% 이상

## 🛠️ 문제 해결

### 대시보드가 로딩되지 않는 경우
1. MediaServer가 정상 실행 중인지 확인
2. 포트 8083이 열려있는지 확인
3. 브라우저 콘솔에서 에러 확인

### 데이터가 업데이트되지 않는 경우
1. 브라우저 개발자 도구에서 네트워크 탭 확인
2. API 엔드포인트가 정상 응답하는지 확인
3. 서버 로그 확인

### CPU/메모리 사용률이 0%로 표시되는 경우
- gopsutil 라이브러리가 정상 설치되었는지 확인
- 운영체제 권한 문제일 수 있음 (특히 Linux)

## 📊 성능 영향

모니터링 시스템의 리소스 사용량:

- **CPU**: 약 1~2% 추가 사용
- **메모리**: 약 10~20MB 추가 사용
- **네트워크**: 무시할 수준 (API 호출 시에만)

## 🔐 보안

- 현재 버전은 인증 없이 접근 가능
- 프로덕션 환경에서는 다음 조치 권장:
  - 방화벽으로 모니터링 포트 제한
  - VPN을 통한 접근만 허용
  - 또는 Basic Auth 추가

## 📝 향후 계획

- [ ] 알림 이메일/Slack 통합
- [ ] 히스토리 데이터 영구 저장 (DB)
- [ ] 더 많은 메트릭 추가 (GPU 사용률 등)
- [ ] 커스텀 경고 임계값 설정
- [ ] 모바일 반응형 디자인 개선
- [ ] 다크/라이트 테마 전환

## 📞 지원

문제가 발생하거나 기능 요청이 있으시면 이슈를 등록해주세요.

---

**버전:** 1.0.0  
**최종 업데이트:** 2026-01-19


