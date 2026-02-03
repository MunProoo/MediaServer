// WebSocket 연결
let healthWebSocket = null;
let reconnectAttempts = 0;
const MAX_RECONNECT_ATTEMPTS = 5;
const RECONNECT_DELAY = 3000; // 3초

let dataManager = DataManager.getInstance();

// 초기화
document.addEventListener('DOMContentLoaded', function() {
    initializeApp();
});

// 앱 초기화
function initializeApp() {
    connectWebSocket();
    loadSettings();
    setupCoreLiveViewLink();
    setupTabs();
    loadStreams();
    setupStreamForm();
    // 주기적으로 상태 업데이트 (5초마다)
    setInterval(updateDashboard, 5000);
}

// 탭 설정
function setupTabs() {
    // 탭 링크 클릭 이벤트
    document.querySelectorAll('.tab-link').forEach(link => {
        link.addEventListener('click', function(e) {
            e.preventDefault();
            const tabName = this.getAttribute('data-tab');
            switchTab(tabName);
        });
    });
    
    // URL 파라미터 확인 (recording 접근 시)
    const urlParams = new URLSearchParams(window.location.search);
    const tab = urlParams.get('tab');
    if (tab) {
        switchTab(tab);
    } else if (window.location.pathname === '/v1/recording') {
        switchTab('recording');
    }
}

// 탭 전환
function switchTab(tabName) {
    // 현재 활성화된 탭 확인
    const currentActiveTab = document.querySelector('.tab-content.active');
    const currentTabName = currentActiveTab ? currentActiveTab.id : null;
    
    // Recording 탭을 떠날 때 정리
    if (currentTabName === 'recording' && tabName !== 'recording') {
        if (typeof cleanupRecording !== 'undefined') {
            cleanupRecording();
        }
    }
    
    // 모든 탭 컨텐츠 숨기기
    document.querySelectorAll('.tab-content').forEach(content => {
        content.classList.remove('active');
    });
    
    // 모든 탭 링크 비활성화
    document.querySelectorAll('.tab-link').forEach(link => {
        link.classList.remove('active');
    });
    
    // 선택된 탭 컨텐츠 표시
    const targetTab = document.getElementById(tabName);
    if (targetTab) {
        targetTab.classList.add('active');
    }
    
    // 선택된 탭 링크 활성화
    const targetLink = document.querySelector(`.tab-link[data-tab="${tabName}"]`);
    if (targetLink) {
        targetLink.classList.add('active');
    }
    
    // URL 업데이트 (히스토리 추가)
    const newUrl = tabName === 'dashboard' ? '/' : `/?tab=${tabName}`;
    window.history.pushState({ tab: tabName }, '', newUrl);
    
    // Recording 탭인 경우 초기화
    if (tabName === 'recording') {
        initializeRecordingTab();
    }
    
    // Monitoring 탭인 경우 초기화
    if (tabName === 'monitoring') {
        initializeMonitoringTab();
    }
}

// Recording 탭 초기화
function initializeRecordingTab() {
    // recording.js의 초기화 함수가 있으면 호출
    if (typeof initializeRecording !== 'undefined') {
        initializeRecording();
    }
}

// Monitoring 탭 초기화
function initializeMonitoringTab() {
    if(!dataManager.getMediaServer()) {
        return;
    }
    
    const monitoringContainer = document.querySelector('#monitoring .monitoring-container');
    if (!monitoringContainer) return;
    
    // 이미 iframe이 있으면 제거 (중복 방지)
    const existingFrame = monitoringContainer.querySelector('#monitoringFrame');
    if (existingFrame) {
        existingFrame.remove();
    }
    
    // iframe 동적 생성
    const iframe = document.createElement('iframe');
    iframe.id = 'monitoringFrame';
    iframe.src = dataManager.getMediaServer() + '/monitoring/dashboard';
    iframe.setAttribute('frameborder', '0');
    iframe.setAttribute('scrolling', 'yes');
    iframe.style.width = '100%';
    iframe.style.border = 'none';
    iframe.style.background = 'white';
    iframe.title = 'MediaServer Monitoring';
    
    monitoringContainer.appendChild(iframe);
}

// Monitoring 탭 정리 (탭을 떠날 때 호출)
function cleanupMonitoringTab() {
    const monitoringFrame = document.getElementById('monitoringFrame');
    if (monitoringFrame) {
        monitoringFrame.remove();
    }
}

// Settings 탭 초기화
function initializeSettingsTab() {
    if(!dataManager.getMediaServer()) {
        return;
    }
    
    const settingsContainer = document.querySelector('#settings .monitoring-container');
    if (!settingsContainer) return;
    
    // 이미 iframe이 있으면 제거 (중복 방지)
    const existingFrame = settingsContainer.querySelector('#settingsFrame');
    if (existingFrame) {
        existingFrame.remove();
    }
    
    // iframe 동적 생성
    const iframe = document.createElement('iframe');
    iframe.id = 'settingsFrame';
    iframe.src = dataManager.getMediaServer() + '/pages/settings';
    iframe.setAttribute('frameborder', '0');
    iframe.setAttribute('scrolling', 'yes');
    iframe.style.width = '100%';
    iframe.style.border = 'none';
    iframe.style.background = 'white';
    iframe.title = 'MediaServer Settings';
    
    settingsContainer.appendChild(iframe);
}

// Settings 탭 정리 (탭을 떠날 때 호출)
function cleanupSettingsTab() {
    const settingsFrame = document.getElementById('settingsFrame');
    if (settingsFrame) {
        settingsFrame.remove();
    }
}

// LiveView 링크 클릭 이벤트 설정 (모든 LiveView 링크에 적용)
function setupCoreLiveViewLink() {
    const coreLiveViewLinks = document.querySelectorAll('.core-live-view-link');
    coreLiveViewLinks.forEach(link => {
        link.addEventListener('click', function(e) {
            e.preventDefault();
            const address = window.location.origin + "/v1/coreLiveView?controls=1";
            window.open(address, "_blank");
        });
    });
}

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
    if (data.streams && Array.isArray(data.streams)) {
        updateStreamStats(data.streams);
        updateRecentStreams(data.streams);
    }
}

// 서버 업타임 표시
function updateServerUptime(seconds) {
    // DataManager에서 업타임 가져오기 (파라미터가 없으면)
    const dataManager = DataManager.getInstance();
    const uptime = seconds !== undefined ? seconds : dataManager.getUptime();
    
    if (uptime === null || uptime === undefined) return;
    
    const uptimeElement = document.getElementById('serverUptime');
    if (!uptimeElement) return;
    
    const days = Math.floor(seconds / 86400);
    const hours = Math.floor((seconds % 86400) / 3600);
    const minutes = Math.floor((seconds % 3600) / 60);
    
    let uptimeText = '';
    if (days > 0) {
        uptimeText = `${days}d ${hours}h`;
    } else if (hours > 0) {
        uptimeText = `${hours}h ${minutes}m`;
    } else {
        uptimeText = `${minutes}m`;
    }
    
    uptimeElement.textContent = uptimeText;
}

// 스트림 통계 업데이트
function updateStreamStats() {
    // DataManager에서 스트림 목록 가져오기
    const dataManager = DataManager.getInstance();
    const streams = dataManager.getStreams();
    
    // 총 스트림 수
    const totalStreamsElement = document.getElementById('totalStreams');
    if (totalStreamsElement) {
        totalStreamsElement.textContent = streams.length;
    }
    
    // 활성 스트림 수 (녹화 중이거나 활성 상태인 스트림)
    const activeStreamsInfo = document.getElementById('activeStreamsInfo');
    if (activeStreamsInfo) {
        const activeCount = streams.filter(s => s.recording).length;
        activeStreamsInfo.textContent = `${activeCount} active`;
    }
    
    // 녹화 중인 스트림 수
    const recordingStreamsElement = document.getElementById('recordingStreams');
    if (recordingStreamsElement) {
        const recordingCount = streams.filter(s => s.recording).length;
        recordingStreamsElement.textContent = recordingCount;
    }
}

// 최근 스트림 목록 업데이트
function updateRecentStreams() {
    // DataManager에서 스트림 목록 가져오기
    const dataManager = DataManager.getInstance();
    const streams = dataManager.getStreams();
    
    const recentStreamsContainer = document.getElementById('recentStreams');
    if (!recentStreamsContainer) return;
    
    if (streams.length === 0) {
        recentStreamsContainer.innerHTML = `
            <div class="empty-state">
                <div class="empty-icon">📡</div>
                <p>No streams configured</p>
                <button class="btn btn-primary btn-small" onclick="showAddStreamModal()">Add Your First Stream</button>
            </div>
        `;
        return;
    }
    
    // 최대 6개만 표시
    const recentStreams = streams.slice(0, 6);
    
    recentStreamsContainer.innerHTML = recentStreams.map(stream => `
        <div class="recent-stream-card">
            <div class="stream-card-header">
                <h4>${escapeHtml(stream.name)}</h4>
                <span class="stream-badge ${stream.recording ? 'badge-recording' : 'badge-inactive'}">
                    ${stream.recording ? '🔴 Recording' : '⚪ Inactive'}
                </span>
            </div>
            <div class="stream-card-body">
                <p class="stream-info">
                    <span class="info-label">IP:</span>
                    <span class="info-value">${escapeHtml(stream.ip)}</span>
                </p>
                <p class="stream-info">
                    <span class="info-label">Stream ID:</span>
                    <span class="info-value stream-id">${escapeHtml(stream.streamID.substring(0, 8))}...</span>
                </p>
            </div>
            <div class="stream-card-actions">
                <button class="btn btn-small btn-primary" onclick="editStream('${stream.streamID}')">Edit</button>
                <a href="#" class="btn btn-small btn-secondary tab-link" data-tab="recording" onclick="selectStreamForRecording('${stream.streamID}')">View</a>
            </div>
        </div>
    `).join('');
}

// HTML 이스케이프
function escapeHtml(text) {
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
}

// 스트림 선택 (Recording 탭으로 이동)
function selectStreamForRecording(streamID) {
    // Recording 탭으로 전환
    switchTab('recording');
    
    // 스트림 선택은 recording.js에서 처리
    setTimeout(() => {
        if (typeof selectStreamById !== 'undefined') {
            selectStreamById(streamID);
        }
    }, 300);
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

// 대시보드 업데이트
async function updateDashboard() {
    // WebSocket이 연결되어 있으면 자동으로 업데이트되므로 불필요
    // WebSocket이 연결되지 않았을 때만 HTTP API 사용
    if (!healthWebSocket || healthWebSocket.readyState !== WebSocket.OPEN) {
        await loadStreams();
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
            updateRecordingStreamList(streamsData);
        }
    }
}

// 스트림 목록 로드 (초기 로드용, 이후는 WebSocket으로 업데이트)
async function loadStreams() {
    // WebSocket이 연결되어 있으면 WebSocket으로부터 받아오고,
    // 연결되지 않았을 때만 HTTP API 사용
    if (healthWebSocket && healthWebSocket.readyState === WebSocket.OPEN) {
        // WebSocket이 연결되어 있으면 WebSocket 메시지를 기다림
        // (서버가 주기적으로 보내므로 별도 요청 불필요)
        return;
    }
    
    // WebSocket이 연결되지 않았을 때만 HTTP API 사용
    try {
        const response = await fetch('/api/streams');
        const data = await response.json();
        if (data.streams) {
            // DataManager에 데이터 저장
            const dataManager = DataManager.getInstance();
            dataManager.updateData(data);
            
            updateHealthStatus(data);
            // updateStreamList는 DataManager에서 가져오므로 파라미터 불필요
            updateStreamList();
        }
    } catch (error) {
        console.error('Failed to load streams:', error);
    }
}

// 스트림 목록 업데이트
function updateStreamList(streams) {
    // DataManager에서 스트림 목록 가져오기 (파라미터가 없거나 비어있으면)
    const dataManager = DataManager.getInstance();
    const streamsToUse = (streams && streams.length > 0) ? streams : dataManager.getStreams();
    
    const streamList = document.getElementById('streamList');
    streamList.innerHTML = '';
    
    if (streams.length === 0) {
        streamList.innerHTML = '<p style="text-align: center; color: #999; padding: 20px;">No streams configured. Click "Add Stream" to create one.</p>';
        return;
    }
    
    streams.forEach(stream => {
        const streamCard = document.createElement('div');
        streamCard.className = 'stream-card';
        streamCard.innerHTML = `
            <h3>${escapeHtml(stream.name)}</h3>
            <p><strong>IP:</strong> ${escapeHtml(stream.ip)}</p>
            <p><strong>RTSP URL:</strong> ${escapeHtml(stream.rtspURL)}</p>
            <p><strong>Stream ID:</strong> ${escapeHtml(stream.streamID)}</p>
            <p><strong>Recording:</strong> <span id="recording-status-${stream.streamID}">${stream.recording ? 'Yes' : 'No'}</span></p>
            <div class="stream-actions">
                <button class="btn btn-edit btn-small" onclick="editStream('${stream.streamID}')">Edit</button>
                <button class="btn btn-danger btn-small" onclick="deleteStream('${stream.streamID}')">Delete</button>
                <button class="btn ${stream.recording ? 'btn-warning' : 'btn-success'} btn-small" onclick="toggleRecording('${stream.streamID}', ${stream.recording})" id="recording-btn-${stream.streamID}">
                    ${stream.recording ? 'Stop Recording' : 'Start Recording'}
                </button>
            </div>
        `;
        streamList.appendChild(streamCard);
    });
}

// HTML 이스케이프
function escapeHtml(text) {
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
}

// 스트림 추가 모달 표시
function showAddStreamModal() {
    document.getElementById('modalTitle').textContent = 'Add Stream';
    document.getElementById('streamForm').reset();
    document.getElementById('currentStreamID').value = '';
    document.getElementById('streamModal').style.display = 'block';
}

// 스트림 수정
async function editStream(streamId) {
    try {
        const response = await fetch(`/api/streams/${streamId}`);
        const data = await response.json();
        if (data.stream) {
            const stream = data.stream;
            document.getElementById('modalTitle').textContent = 'Edit Stream';
            document.getElementById('currentStreamID').value = stream.streamID;
            document.getElementById('streamName').value = stream.name;
            document.getElementById('streamIP').value = stream.ip;
            document.getElementById('streamRtspURL').value = stream.rtspURL;
            document.getElementById('streamModal').style.display = 'block';
        }
    } catch (error) {
        console.error('Failed to load stream:', error);
        alert('Failed to load stream data');
    }
}

// Recording 토글
async function toggleRecording(streamID, currentRecording) {
    const newRecording = !currentRecording;
    
    try {
        const response = await fetch(`/api/streams/${streamID}/recording`, {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json'
            },
            body: JSON.stringify({ recording: newRecording })
        });
        
        if (response.ok) {
            // UI 업데이트
            const statusElement = document.getElementById(`recording-status-${streamID}`);
            const btnElement = document.getElementById(`recording-btn-${streamID}`);
            
            if (statusElement) {
                statusElement.textContent = newRecording ? 'Yes' : 'No';
            }
            
            if (btnElement) {
                btnElement.textContent = newRecording ? 'Stop Recording' : 'Start Recording';
                btnElement.className = newRecording ? 'btn btn-warning btn-small' : 'btn btn-success btn-small';
                btnElement.setAttribute('onclick', `toggleRecording('${streamID}', ${newRecording})`);
            }
            
            alert(newRecording ? 'Recording started successfully' : 'Recording stopped successfully');
        } else {
            const data = await response.json();
            alert(data.message || 'Failed to update recording');
        }
    } catch (error) {
        console.error('Failed to toggle recording:', error);
        alert('Failed to toggle recording');
    }
}

// 스트림 삭제
async function deleteStream(streamId) {
    if (!confirm('Are you sure you want to delete this stream?')) {
        return;
    }
    
    try {
        const response = await fetch(`/api/streams/${streamId}`, {
            method: 'DELETE'
        });
        
        if (response.ok) {
            alert('Stream deleted successfully');
            loadStreams();
        } else {
            const data = await response.json();
            alert(data.message || 'Failed to delete stream');
        }
    } catch (error) {
        console.error('Failed to delete stream:', error);
        alert('Failed to delete stream');
    }
}

// 모달 닫기
function closeStreamModal() {
    document.getElementById('streamModal').style.display = 'none';
}

// 스트림 폼 설정
function setupStreamForm() {
    document.getElementById('streamForm').addEventListener('submit', async function(e) {
        e.preventDefault();
        
        const currentStreamID = document.getElementById('currentStreamID').value;
        const isEdit = currentStreamID !== '';
        
        const formData = {
            name: document.getElementById('streamName').value,
            ip: document.getElementById('streamIP').value,
            rtspURL: document.getElementById('streamRtspURL').value
        };
        
        try {
            // 스트림 추가/수정 (recording 제외)
            const url = isEdit ? `/api/streams/${currentStreamID}` : '/api/streams';
            const method = isEdit ? 'PUT' : 'POST';
            
            const response = await fetch(url, {
                method: method,
                headers: {
                    'Content-Type': 'application/json'
                },
                body: JSON.stringify(formData)
            });
            
            if (response.ok) {
                alert(isEdit ? 'Stream updated successfully' : 'Stream added successfully');
                closeStreamModal();
                loadStreams();
            } else {
                const data = await response.json();
                alert(data.message || 'Failed to save stream');
            }
        } catch (error) {
            console.error('Failed to save stream:', error);
            alert('Failed to save stream');
        }
    });
}

// 모달 외부 클릭 시 닫기
window.onclick = function(event) {
    const modal = document.getElementById('streamModal');
    if (event.target == modal) {
        closeStreamModal();
    }
}

// 설정 로드
function loadSettings() {
    const savedSettings = localStorage.getItem('clientSettings');
    if (savedSettings) {
        const settings = JSON.parse(savedSettings);
        document.getElementById('serverUrl').value = settings.serverUrl || '';
        document.getElementById('apiKey').value = settings.apiKey || '';
    }
}

// 설정 저장
document.getElementById('settingsForm').addEventListener('submit', function(e) {
    e.preventDefault();
    
    const settings = {
        serverUrl: document.getElementById('serverUrl').value,
        apiKey: document.getElementById('apiKey').value
    };
    
    localStorage.setItem('clientSettings', JSON.stringify(settings));
    alert('Settings saved successfully!');
});

