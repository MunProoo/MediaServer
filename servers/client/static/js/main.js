// WebSocket 연결
let healthWebSocket = null;
let reconnectAttempts = 0;
const MAX_RECONNECT_ATTEMPTS = 5;
const RECONNECT_DELAY = 3000; // 3초

// 초기화
document.addEventListener('DOMContentLoaded', function() {
    initializeApp();
});

// 앱 초기화
function initializeApp() {
    connectWebSocket();
    loadSettings();
    setupCoreLiveViewLink();
    loadStreams();
    setupStreamForm();
    // 주기적으로 상태 업데이트 (5초마다)
    setInterval(updateDashboard, 5000);
}

// Streams 링크 클릭 이벤트 설정
function setupCoreLiveViewLink() {
    const coreLiveViewLink = document.getElementById('coreLiveViewLink');
    if (coreLiveViewLink) {
        coreLiveViewLink.addEventListener('click', function(e) {
            e.preventDefault();
            const address = window.location.origin + "/v1/coreLiveView?controls=1";
            window.open(address, "_blank");
        });
    }
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
        };
        
        healthWebSocket.onmessage = function(event) {
            try {
                const data = JSON.parse(event.data);
                updateHealthStatus(data);
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
    const statusElement = document.getElementById('serverStatus');
    
    if (data.status === 'ok') {
        statusElement.textContent = 'Online';
        statusElement.style.color = '#4caf50';
        
        // Uptime 표시 (선택사항)
        if (data.uptime !== undefined) {
            const hours = Math.floor(data.uptime / 3600);
            const minutes = Math.floor((data.uptime % 3600) / 60);
            const seconds = data.uptime % 60;
            // 필요시 uptime을 표시할 수 있음
        }
    } else {
        statusElement.textContent = 'Offline';
        statusElement.style.color = '#f44336';
    }
}

// 연결 상태 업데이트
function updateConnectionStatus(status) {
    const statusElement = document.getElementById('serverStatus');
    
    switch(status) {
        case 'connected':
            statusElement.textContent = 'Connecting...';
            statusElement.style.color = '#ff9800';
            break;
        case 'disconnected':
            statusElement.textContent = 'Reconnecting...';
            statusElement.style.color = '#ff9800';
            break;
        case 'error':
        case 'failed':
            statusElement.textContent = 'Connection Failed';
            statusElement.style.color = '#f44336';
            break;
    }
}

// 대시보드 업데이트
async function updateDashboard() {
    await loadStreams();
}

// 스트림 목록 로드
async function loadStreams() {
    try {
        const response = await fetch('/api/streams');
        const data = await response.json();
        if (data.streams) {
            updateStreamList(data.streams);
            document.getElementById('activeStreams').textContent = data.streams.length;
        }
    } catch (error) {
        console.error('Failed to load streams:', error);
    }
}

// 스트림 목록 업데이트
function updateStreamList(streams) {
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

