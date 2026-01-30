//go:build windows
// +build windows

package main

import (
	"flag"
	"fmt"
	"log"
	"mjy/define"
	"mjy/logUtil"
	"mjy/serviceUtil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/debug"
)

var isIntSess bool

func main() {
	os.Chdir(filepath.Dir(os.Args[0]))
	const svcName = define.ServiceNameTurnServer

	var err error
	isIntSess, err = svc.IsWindowsService()
	if err != nil {
		log.Println("failed to determine if we are running in an interactive session")
	}
	if isIntSess {
		runService(svcName, false)
		return
	}

	cmd := flag.String("rtype", "debug", "run type")
	flag.Parse()
	*cmd = strings.ToLower(*cmd)

	switch *cmd {
	case "debug":
		runService(svcName, true)
		return
	case "install":
		err = serviceUtil.InstallService(svcName, svcName)
	case "remove":
		err = serviceUtil.RemoveService(svcName)
	case "start":
		err = serviceUtil.StartService(svcName)
	case "stop":
		err = serviceUtil.ControlService(svcName, svc.Stop, svc.Stopped)
	case "pause":
		err = serviceUtil.ControlService(svcName, svc.Pause, svc.Paused)
	case "continue":
		err = serviceUtil.ControlService(svcName, svc.Continue, svc.Running)
	default:
		fmt.Printf("invalid command %s", *cmd)
	}

	if err != nil {
		log.Printf("failed to start service. err : %v", err)
	}
	return
}

// 서비스 구조체 정의
type turnService struct{}

func (m *turnService) Execute(args []string, req <-chan svc.ChangeRequest, status chan<- svc.Status) (svcSpecificExitCode bool, exitCode uint32) {
	const cmdsAccepted = svc.AcceptStop | svc.AcceptShutdown | svc.AcceptPauseAndContinue

	// 서비스가 시작되었음을 Windows에 알림
	status <- svc.Status{State: svc.StartPending}
	log.Println("Service is starting...")

	go StartServer()

	// 서비스가 실행 중임을 알림
	status <- svc.Status{State: svc.Running, Accepts: cmdsAccepted}

loop:
	for {
		select {
		case c := <-req:
			switch c.Cmd {
			case svc.Interrogate:
				status <- c.CurrentStatus
				time.Sleep(100 * time.Millisecond)
				status <- c.CurrentStatus
			case svc.Stop, svc.Shutdown:
				// 종료 작업 시작을 Windows에 알림
				status <- svc.Status{State: svc.StopPending}
				CloseServer()
				time.Sleep(1 * time.Second)
				break loop
			case svc.Pause:
				status <- svc.Status{State: svc.Paused, Accepts: cmdsAccepted}
			case svc.Continue:
				status <- svc.Status{State: svc.Running, Accepts: cmdsAccepted}
			default:
			}
		}
	}
	// 최종 종료 상태 전송
	status <- svc.Status{State: svc.Stopped}
	return
}

func runService(name string, isDebug bool) {
	// 서비스 모드 여부 확인 후 로그 초기화
	if err := logUtil.InitLogging("turnServer", isIntSess); err != nil {
		log.Printf("[ERROR] [main] Failed to initialize logging: %v", err)
		// 로그 초기화 실패해도 서비스는 계속 실행
	}
	defer logUtil.CloseLogging()

	log.Println("start turn service")

	var err error
	run := svc.Run
	if isDebug {
		run = debug.Run
	}
	err = run(name, &turnService{})
	if err != nil {
		return
	}

}
