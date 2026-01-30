//go:build windows
// +build windows

package serviceUtil

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"mjy/define"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/eventlog"
	"golang.org/x/sys/windows/svc/mgr"
)

// exePath ...
func exePath() (string, error) {
	prog := os.Args[0]
	p, err := filepath.Abs(prog)
	if err != nil {
		return "", err
	}
	fi, err := os.Stat(p)
	if err == nil {
		if !fi.Mode().IsDir() {
			return p, nil
		}
		err = fmt.Errorf("%s is directory", p)
	}
	if filepath.Ext(p) == "" {
		p += ".exe"
		fi, err := os.Stat(p)
		if err == nil {
			if !fi.Mode().IsDir() {
				return p, nil
			}
			err = fmt.Errorf("%s is directory", p)
		}
	}
	return "", err
}

// GetState ...
func GetState(name string) (svc.State, error) {
	m, err := mgr.Connect()
	if err != nil {
		return svc.Stopped, err
	}
	defer m.Disconnect()

	s, err := m.OpenService(name)
	if err != nil {
		return svc.Stopped, fmt.Errorf("could not access service: [%s] %v", name, err)
	}
	defer s.Close()

	state, err := s.Query()
	return state.State, err
}

// InstallService ...
func InstallService(name, desc string) error {
	exepath, err := exePath()
	if err != nil {
		return err
	}
	m, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer m.Disconnect()
	s, err := m.OpenService(name)
	if err == nil {
		s.Close()
		return fmt.Errorf("service %s already exists", name)
	}

	// 서버 재시작시 DB 구동전 서비스가 올라오면서 문제가 발생됨
	// 서비스 시작유형 [자동] -> [자동(지연된 시작)] 으로 변경
	switch name {
	case define.ServiceNameTurnServer, define.ServiceNameMediaServer:
		config := mgr.Config{
			ServiceType:      windows.SERVICE_WIN32_OWN_PROCESS,
			StartType:        mgr.StartAutomatic,
			ErrorControl:     mgr.ErrorIgnore,
			DisplayName:      desc,
			DelayedAutoStart: true,
		}

		s, err = m.CreateService(name, exepath, config, "is", "auto-started")
	default:
		config := mgr.Config{
			ServiceType:  windows.SERVICE_WIN32_OWN_PROCESS,
			StartType:    mgr.StartAutomatic,
			ErrorControl: mgr.ErrorIgnore,
			DisplayName:  desc,
		}

		s, err = m.CreateService(name, exepath, config, "is", "auto-started")
	}

	if err != nil {
		return err
	}
	defer s.Close()
	err = eventlog.InstallAsEventCreate(name, eventlog.Error|eventlog.Warning|eventlog.Info)
	if err != nil {
		s.Delete()
		return fmt.Errorf("SetupEventLogSource() failed: %s", err)
	}

	action := make([]mgr.RecoveryAction, 1)
	action[0].Type = mgr.ServiceRestart
	action[0].Delay = 60000
	// action[1].Type = mgr.ServiceRestart
	// action[1].Delay = 60000

	err = s.SetRecoveryActions(action, 3600*24)
	if err != nil {
		s.Delete()
		return fmt.Errorf("SetRecoveryActions() failed: %s", err)
	}

	return nil
}

// RemoveService ...
func RemoveService(name string) error {
	m, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer m.Disconnect()
	s, err := m.OpenService(name)
	if err != nil {
		return fmt.Errorf("service %s is not installed", name)
	}
	defer s.Close()
	err = s.Delete()
	if err != nil {
		return err
	}
	err = eventlog.Remove(name)
	if err != nil {
		return fmt.Errorf("RemoveEventLogSource() failed: %s", err)
	}
	return nil
}

// StartService ...
func StartService(name string) error {
	m, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer m.Disconnect()
	s, err := m.OpenService(name)
	if err != nil {
		return fmt.Errorf("could not access service: %v", err)
	}
	defer s.Close()
	err = s.Start("is", "manual-started")
	if err != nil {
		return fmt.Errorf("could not start service: %v", err)
	}
	return nil
}

// ControlService ...
func ControlService(name string, c svc.Cmd, to svc.State) error {
	m, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer m.Disconnect()
	s, err := m.OpenService(name)
	if err != nil {
		return fmt.Errorf("could not access service: %v", err)
	}
	defer s.Close()
	status, err := s.Control(c)
	if err != nil {
		return fmt.Errorf("could not send control=%d: %v", c, err)
	}
	timeout := time.Now().Add(10 * time.Second)
	for status.State != to {
		if timeout.Before(time.Now()) {
			return fmt.Errorf("timeout waiting for service to go to state=%d", to)
		}
		time.Sleep(300 * time.Millisecond)
		status, err = s.Query()
		if err != nil {
			return fmt.Errorf("could not retrieve service status: %v", err)
		}
	}
	return nil
}
