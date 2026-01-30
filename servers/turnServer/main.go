// SPDX-FileCopyrightText: 2023 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

// Package main implements a simple TURN server
package main

import (
	"fmt"
	"net"
	"regexp"
	"strconv"
	"sync"
	"time"

	"log"

	"github.com/pion/turn/v3"
)

// var log = logrus.New()

type TurnServerConfig struct {
	PublicIP           string
	Port               int
	Users              string
	Realm              string
	MonitorIntervalMin int // 모니터링 로그 출력 간격 (분 단위, 기본값: 5분)
	MinPort            int // 릴레이 포트 범위 최소값 (기본값: 49152)
	MaxPort            int // 릴레이 포트 범위 최대값 (기본값: 65535)
}

var config TurnServerConfig
var server *turn.Server

func StartServer() {
	// WireShark를 통해서 요청이 왔는지 확인 (BINDING REQUEST)

	// 클라이언트 쌍 통계를 위한 변수들
	var (
		tcpClients    int
		udpClients    int
		totalClients  int
		activeClients = make(map[string]string) // 클라이언트별 프로토콜 추적 (클라이언트ID -> 프로토콜)
		clientsMutex  sync.RWMutex              // activeClients 맵 동시성 제어
	)

	loadConfig()

	publicIP := config.PublicIP
	port := config.Port
	users := config.Users
	realm := config.Realm
	minPort := config.MinPort
	maxPort := config.MaxPort

	// 기본 포트 범위 설정
	if minPort == 0 {
		minPort = 49152 // IANA 권장 동적 포트 시작
	}
	if maxPort == 0 {
		maxPort = 49220 // 최대 포트
	}

	if len(publicIP) == 0 {
		log.Printf("[ERROR] [main] [StartServer] 'public ip' is required")
		return
	} else if len(users) == 0 {
		log.Printf("[ERROR] [main] [StartServer] 'users' is required")
		return
	}

	log.Printf("[INFO] [main] [StartServer] Start Turn Server")

	// Create a UDP listener to pass into pion/turn
	// pion/turn itself doesn't allocate any UDP sockets, but lets the user pass them in
	// this allows us to add logging, storage or modify inbound/outbound traffic
	// ㅕ예 fltmsj tjfwjd
	udpListener, err := net.ListenPacket("udp4", "0.0.0.0:"+strconv.Itoa(port))
	if err != nil {
		log.Printf("[ERROR] [main] [StartServer] Failed to create udp listener: %v", err)
	}

	tcpListener, err := net.Listen("tcp4", "0.0.0.0:"+strconv.Itoa(port))
	if err != nil {
		log.Printf("[ERROR] [main] [StartServer] Failed to create tcp listener: %v", err)
	}

	// Cache -users flag for easy lookup later
	// If passwords are stored they should be saved to your DB hashed using turn.GenerateAuthKey
	usersMap := map[string][]byte{}
	for _, kv := range regexp.MustCompile(`(\w+)=(\w+)`).FindAllStringSubmatch(users, -1) {
		// 사용자이름, 영역, 비밀번호를 사용하여 사용자 인증키 생성
		usersMap[kv[1]] = turn.GenerateAuthKey(kv[1], realm, kv[2])
	}

	// RelayAddressGenerator 생성 - TCP와 UDP에서 공유
	// 포트 범위를 지정하여 방화벽 규칙 설정을 쉽게 함
	relayGenerator := &turn.RelayAddressGeneratorPortRange{
		RelayAddress: net.ParseIP(publicIP), // 클라이언트에게 알려줄 IP (공인 IP)
		Address:      "0.0.0.0",             // 실제 서버가 바인딩할 주소 (모든 인터페이스)
		MinPort:      uint16(minPort),       // 릴레이 포트 범위 시작
		MaxPort:      uint16(maxPort),       // 릴레이 포트 범위 끝
	}

	// TURN 서버 인스턴스 생성
	server, err = turn.NewServer(turn.ServerConfig{
		Realm: realm,
		// Set AuthHandler callback
		// This is called every time a user tries to authenticate with the TURN server
		// Return the key for that user, or false when no user is found
		AuthHandler: func(username string, realm string, srcAddr net.Addr) ([]byte, bool) { // nolint: revive
			// 연결 프로토콜 확인
			protocol := "unknown"
			clientID := srcAddr.String()

			if tcpAddr, ok := srcAddr.(*net.TCPAddr); ok {
				protocol = "TCP"
				msg := fmt.Sprintf("Client connected - TCP %s:%d", tcpAddr.IP, tcpAddr.Port)
				log.Printf("[INFO] [main] [AuthHandler] %s", msg)
			} else if udpAddr, ok := srcAddr.(*net.UDPAddr); ok {
				protocol = "UDP"
				msg := fmt.Sprintf("Client connected - UDP %s:%d", udpAddr.IP, udpAddr.Port)
				log.Printf("[INFO] [main] [AuthHandler] %s", msg)
			}

			// 클라이언트 추적 정보 저장
			clientsMutex.Lock()
			_, exists := activeClients[clientID]
			if !exists {
				activeClients[clientID] = protocol
				totalClients++

				if protocol == "TCP" {
					tcpClients++
				} else if protocol == "UDP" {
					udpClients++
				}
			}
			clientsMutex.Unlock()

			return usersMap[username], true
		},
		// PacketConnConfigs is a list of UDP Listeners and the configuration around them
		PacketConnConfigs: []turn.PacketConnConfig{
			{
				PacketConn:            udpListener,
				RelayAddressGenerator: relayGenerator, // UDP와 TCP 모두 동일한 generator 사용
			},
		},
		ListenerConfigs: []turn.ListenerConfig{
			{
				Listener:              tcpListener,
				RelayAddressGenerator: relayGenerator, // UDP와 TCP 모두 동일한 generator 사용
			},
		},
	})
	if err != nil {
		log.Printf("[ERROR] [main] [StartServer] failed to turn.NewServer: %v", err)
	}

	// Allocation 모니터링 고루틴 시작 - 현재 연결 상태 표시
	go func() {
		log.Printf("[INFO] [main] [AllocationMonitor] Allocation monitor started")

		// 모니터링 함수
		monitorAllocations := func() {
			if server == nil {
				return
			}

			// 현재 Allocation 개수 확인
			currentCount := server.AllocationCount()

			// activeClients 정리: 실제 allocation 수와 맞추기
			clientsMutex.Lock()
			activeCount := len(activeClients)

			// allocation이 감소했으면 activeClients에서도 제거
			if currentCount < activeCount {
				removeCount := activeCount - currentCount
				var toRemove []string

				for clientID := range activeClients {
					toRemove = append(toRemove, clientID)
					if len(toRemove) >= removeCount {
						break
					}
				}

				for _, clientID := range toRemove {
					protocol := activeClients[clientID]
					delete(activeClients, clientID)

					if protocol == "TCP" {
						tcpClients--
					} else if protocol == "UDP" {
						udpClients--
					}
					totalClients--
				}
			}

			// 현재 TCP/UDP 개수 계산 (Lock 안에서 직접 계산)
			currentTCP := 0
			currentUDP := 0
			for _, protocol := range activeClients {
				if protocol == "TCP" {
					currentTCP++
				} else if protocol == "UDP" {
					currentUDP++
				}
			}
			clientsMutex.Unlock()

			// 현재 연결 상태 출력
			msg := fmt.Sprintf("Current connections - Total: %d (TCP: %d, UDP: %d)",
				currentCount, currentTCP, currentUDP)
			log.Printf("[INFO] [main] [AllocationMonitor] %s", msg)
		}

		// 첫 실행 (즉시)
		monitorAllocations()

		// 설정된 간격으로 실행 (기본값: 5분)
		intervalMin := config.MonitorIntervalMin
		if intervalMin <= 0 {
			intervalMin = 5 // 기본값
		}
		ticker := time.NewTicker(time.Duration(intervalMin) * time.Minute)
		defer ticker.Stop()

		msg := fmt.Sprintf("Monitor interval set to %d minutes", intervalMin)
		log.Printf("[INFO] [main] [AllocationMonitor] %s", msg)

		for range ticker.C {
			monitorAllocations()
		}

		log.Printf("[INFO] [main] [AllocationMonitor] Monitor loop exited")
	}()

	// Block until user sends SIGINT or SIGTERM
	// SIGINT : 터미널의 Ctrl + C
	// SIGTERM : 프로세스 종료 신호 (kill process)
	// sigs := make(chan os.Signal, 1)
	// signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	// <-sigs

	// 시그널 대기 코드 주석
	// <-sigs
}

func CloseServer() {
	log.Printf("[INFO] [main] [CloseServer] Closing server...")
	if err := server.Close(); err != nil {
		log.Printf("[ERROR] [main] [CloseServer] failed to close server: %v", err)
	}
	log.Printf("[INFO] [main] [CloseServer] Server closed, function returning")
}
