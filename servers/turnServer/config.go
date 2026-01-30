package main

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
)

func loadConfig() {
	exePath, _ := os.Executable()
	exeDir := filepath.Dir(exePath)
	configFile := filepath.Join(exeDir, "tnConfig.json")

	data, err := os.ReadFile(configFile)
	if err != nil {
		if os.IsNotExist(err) {
			log.Printf("[ERROR] [main] [loadConfig] make default config... file: %s", configFile)

			config = createDefaultConfig()
			if err := saveConfigToFile(configFile, &config); err != nil {
				log.Printf("[ERROR] [main] [loadConfig] [saveConfigToFile] %v", err)
				os.Exit(1)
			}
		} else {
			log.Printf("[ERROR] [main] [loadConfig] cannot open config file: %v", err)
			os.Exit(1)
		}
	} else {
		err = json.Unmarshal(data, &config)
		if err != nil {
			log.Printf("[ERROR] [main] [loadConfig] [Unmarshal] %v", err)
			os.Exit(1)
		}
	}

}

// createDefaultConfig 기본 설정 생성
func createDefaultConfig() TurnServerConfig {
	return TurnServerConfig{
		PublicIP:           "14.14.14.2",
		Users:              "mjy=mjy", // TURN 서버는 많은 자원이 소모되므로 검증된 사용자만 사용하도록 한다.
		Port:               8888,
		Realm:              "pion.ly",
		MonitorIntervalMin: 5, // 5분마다 모니터링 로그 출력
	}
}

// saveConfigToFile config를 파일로 저장
func saveConfigToFile(filePath string, config *TurnServerConfig) error {
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filePath, data, 0644)
}
