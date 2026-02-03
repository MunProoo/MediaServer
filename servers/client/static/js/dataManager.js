// ============================================
// DataManager (싱글톤 패턴)
// healthCheck로 받는 정보들을 저장하고 관리
// 
// 사용 예시:
//   const dataManager = DataManager.getInstance();
//   const allData = dataManager.getData();
//   const streams = dataManager.getStreams();
//   const stream = dataManager.getStream('stream-id');
//   const status = dataManager.getStatus();
//   const uptime = dataManager.getUptime();
// ============================================
const DataManager = (function() {
    let instance = null;
    
    class DataManager {
        constructor() {
            if (instance) {
                return instance;
            }
            
            this.data = {
                status: null,
                message: null,
                timestamp: null,
                uptime: null,
                streams: [],
                mediaServer: null,
                lastUpdate: null
            };
            
            instance = this;
        }
        
        // 데이터 업데이트
        updateData(healthData) {
            if (!healthData) return;
            
            this.data.status = healthData.status || this.data.status;
            this.data.message = healthData.message || this.data.message;
            this.data.timestamp = healthData.timestamp || this.data.timestamp;
            this.data.uptime = healthData.uptime !== undefined ? healthData.uptime : this.data.uptime;
            this.data.mediaServer = healthData.mediaServer || this.data.mediaServer;
            this.data.lastUpdate = new Date();
            
            if (healthData.streams && Array.isArray(healthData.streams)) {
                this.data.streams = healthData.streams;
            }
        }
        
        // 전체 데이터 가져오기
        getData() {
            return { ...this.data };
        }
        
        // 상태 가져오기
        getStatus() {
            return this.data.status;
        }
        
        // 업타임 가져오기
        getUptime() {
            return this.data.uptime;
        }
        
        // 스트림 목록 가져오기
        getStreams() {
            return [...this.data.streams];
        }
        
        // 특정 스트림 가져오기
        getStream(streamID) {
            return this.data.streams.find(stream => stream.streamID === streamID);
        }
        
        // 녹화 중인 스트림 목록 가져오기
        getRecordingStreams() {
            return this.data.streams.filter(stream => stream.recording);
        }
        
        // 온라인 스트림 개수 가져오기
        getOnlineStreamCount() {
            // status가 'ok'이면 모든 스트림이 온라인으로 간주
            // 또는 streams 배열에서 상태를 확인할 수 있다면 그걸 사용
            return this.data.streams.length;
        }
        
        // 마지막 업데이트 시간 가져오기
        getLastUpdate() {
            return this.data.lastUpdate;
        }

        getMediaServer() {
            return this.data.mediaServer;
        }
        
        // 데이터 초기화
        clear() {
            this.data = {
                status: null,
                message: null,
                timestamp: null,
                uptime: null,
                streams: [],
                mediaServer: null,
                lastUpdate: null
            };
        }
    }
    
    return {
        getInstance: function() {
            if (!instance) {
                instance = new DataManager();
            }
            return instance;
        }
    };
})();

