// 대시보드 업데이트 및 관리

// 대시보드 업데이트
async function updateDashboard() {
    // WebSocket이 연결되어 있으면 자동으로 업데이트되므로 불필요
    // WebSocket이 연결되지 않았을 때만 HTTP API 사용
    if (!healthWebSocket || healthWebSocket.readyState !== WebSocket.OPEN) {
        await loadStreams();
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
    
    const days = Math.floor(uptime / 86400);
    const hours = Math.floor((uptime % 86400) / 3600);
    const minutes = Math.floor((uptime % 3600) / 60);
    
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
                <button class="btn btn-small ${stream.recording ? 'btn-warning' : 'btn-success'}" onclick="toggleRecording('${stream.streamID}', ${stream.recording})" id="recording-btn-recent-${stream.streamID}">
                    ${stream.recording ? 'Stop Recording' : 'Start Recording'}
                </button>
                <a href="#" class="btn btn-small btn-secondary tab-link" data-tab="recording" onclick="selectStreamForRecording('${stream.streamID}')">View</a>
            </div>
        </div>
    `).join('');
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

