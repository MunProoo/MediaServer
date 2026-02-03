// WebSocket 연결 및 관리
let healthWebSocket = null;
let reconnectAttempts = 0;
const MAX_RECONNECT_ATTEMPTS = 5;
const RECONNECT_DELAY = 3000; // 3초

// WebSocket 연결
function connectWebSocket() {
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const wsUrl = `${protocol}//${window.location.host}/ws/health`;
    
    try {
        healthWebSocket = new WebSocket(wsUrl);
        
        healthWebSocket.onopen = function(event) {
            console.log('WebSocket connected');
            reconnectAttempts = 0;
            updateConnectionStatus('connected');
            // WebSocket 연결 후 초기 스트림 목록 요청 (서버가 자동으로 보내지만, 즉시 받기 위해)
            // 서버가 주기적으로 보내므로 별도 요청 불필요
        };
        
        healthWebSocket.onmessage = function(event) {
            try {
                const data = JSON.parse(event.data);
                
                // DataManager에 데이터 저장
                const dataManager = DataManager.getInstance();
                dataManager.updateData(data);
                
                updateHealthStatus(data);
                // 스트림 목록이 있으면 업데이트
                if (data.streams && Array.isArray(data.streams)) {
                    updateStreamListFromWebSocket(data.streams);
                }
            } catch (error) {
                console.error('Failed to parse WebSocket message:', error);
            }
        };
        
        healthWebSocket.onerror = function(error) {
            console.error('WebSocket error:', error);
            updateConnectionStatus('error');
        };
        
        healthWebSocket.onclose = function(event) {
            console.log('WebSocket closed:', event.code, event.reason);
            updateConnectionStatus('disconnected');
            
            // 자동 재연결
            if (reconnectAttempts < MAX_RECONNECT_ATTEMPTS) {
                reconnectAttempts++;
                console.log(`Reconnecting... (${reconnectAttempts}/${MAX_RECONNECT_ATTEMPTS})`);
                setTimeout(connectWebSocket, RECONNECT_DELAY);
            } else {
                console.error('Max reconnection attempts reached');
                updateConnectionStatus('failed');
            }
        };
    } catch (error) {
        console.error('Failed to create WebSocket:', error);
        updateConnectionStatus('error');
    }
}

// Health 상태 업데이트
function updateHealthStatus(data) {
    // DataManager에서 최신 데이터 가져오기
    const dataManager = DataManager.getInstance();
    const healthData = dataManager.getData();
    
    // 서버 상태 업데이트
    const statusElement = document.getElementById('serverStatus');
    if (statusElement) {
        const status = healthData.status || data?.status;
        if (status === 'ok') {
            statusElement.textContent = 'Online';
            statusElement.style.color = '#4caf50';
        } else {
            statusElement.textContent = 'Offline';
            statusElement.style.color = '#f44336';
        }
    }
    
    // 서버 업타임 업데이트
    const uptime = healthData.uptime !== undefined ? healthData.uptime : data?.uptime;
    if (uptime !== undefined) {
        updateServerUptime(uptime);
    }
    
    // 스트림 통계 업데이트
    const streams = healthData.streams && healthData.streams.length > 0 ? healthData.streams : (data?.streams || []);
    if (streams.length > 0 || (data?.streams && Array.isArray(data.streams))) {
        updateStreamStats();
        updateRecentStreams();
    }
}

// 연결 상태 업데이트
function updateConnectionStatus(status) {
    const statusElement = document.getElementById('connectionStatus');
    const infoElement = document.getElementById('connectionInfo');
    
    if (!statusElement) return;
    
    switch(status) {
        case 'connecting':
            statusElement.textContent = 'Connecting';
            statusElement.style.color = '#ff9800';
            if (infoElement) infoElement.textContent = 'WebSocket';
            break;
        case 'connected':
            statusElement.textContent = 'Connected';
            statusElement.style.color = '#4caf50';
            if (infoElement) infoElement.textContent = 'WebSocket';
            break;
        case 'disconnected':
            statusElement.textContent = 'Reconnecting';
            statusElement.style.color = '#ff9800';
            if (infoElement) infoElement.textContent = 'WebSocket';
            break;
        case 'error':
        case 'failed':
            statusElement.textContent = 'Failed';
            statusElement.style.color = '#f44336';
            if (infoElement) infoElement.textContent = 'WebSocket';
            break;
    }
}

// WebSocket으로부터 스트림 목록 업데이트
function updateStreamListFromWebSocket(streamsData) {
    // DataManager에서 최신 스트림 목록 가져오기
    const dataManager = DataManager.getInstance();
    const streams = dataManager.getStreams();
    
    // streamsData가 있으면 사용, 없으면 DataManager에서 가져오기
    const streamsToUpdate = (streamsData && Array.isArray(streamsData) && streamsData.length > 0) 
        ? streamsData 
        : streams;
    
    if (streamsToUpdate.length > 0 || (streamsData && Array.isArray(streamsData))) {
        // updateStreamList는 DataManager에서 가져오므로 파라미터 불필요
        updateStreamList();
        
        // Recording 탭의 스트림 목록도 업데이트
        if (typeof updateRecordingStreamList === 'function') {
            updateRecordingStreamList(streamsToUpdate);
        }
    }
}

