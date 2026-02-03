// 스트림 관리 기능

// HTML 이스케이프 유틸리티
function escapeHtml(text) {
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
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
    if (!streamList) return;
    
    streamList.innerHTML = '';
    
    if (streamsToUse.length === 0) {
        streamList.innerHTML = '<p style="text-align: center; color: #999; padding: 20px;">No streams configured. Click "Add Stream" to create one.</p>';
        return;
    }
    
    streamsToUse.forEach(stream => {
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
        Swal.fire({
            icon: 'error',
            title: '오류',
            text: '스트림 데이터를 불러오는데 실패했습니다.',
            confirmButtonText: '확인'
        });
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
            
            // RecentStreams의 버튼도 업데이트
            const recentBtnElement = document.getElementById(`recording-btn-recent-${streamID}`);
            if (recentBtnElement) {
                recentBtnElement.textContent = newRecording ? 'Stop Recording' : 'Start Recording';
                recentBtnElement.className = newRecording ? 'btn btn-small btn-warning' : 'btn btn-small btn-success';
                recentBtnElement.setAttribute('onclick', `toggleRecording('${streamID}', ${newRecording})`);
            }
            
            // RecentStreams의 배지도 업데이트
            updateRecentStreams();
            
            Swal.fire({
                icon: 'success',
                title: '성공',
                text: newRecording ? '녹화가 시작되었습니다.' : '녹화가 중지되었습니다.',
                timer: 2000,
                showConfirmButton: false
            });
        } else {
            const data = await response.json();
            Swal.fire({
                icon: 'error',
                title: '오류',
                text: data.message || '녹화 상태 업데이트에 실패했습니다.',
                confirmButtonText: '확인'
            });
        }
    } catch (error) {
        console.error('Failed to toggle recording:', error);
        Swal.fire({
            icon: 'error',
            title: '오류',
            text: '녹화 상태를 변경하는데 실패했습니다.',
            confirmButtonText: '확인'
        });
    }
}

// 스트림 삭제
async function deleteStream(streamId) {
    const result = await Swal.fire({
        title: '스트림 삭제',
        text: '이 스트림을 삭제하시겠습니까?',
        icon: 'warning',
        showCancelButton: true,
        confirmButtonColor: '#d33',
        cancelButtonColor: '#3085d6',
        confirmButtonText: '삭제',
        cancelButtonText: '취소'
    });
    
    if (!result.isConfirmed) {
        return;
    }
    
    try {
        const response = await fetch(`/api/streams/${streamId}`, {
            method: 'DELETE'
        });
        
        if (response.ok) {
            Swal.fire({
                icon: 'success',
                title: '삭제 완료',
                text: '스트림이 성공적으로 삭제되었습니다.',
                timer: 2000,
                showConfirmButton: false
            });
            loadStreams();
        } else {
            const data = await response.json();
            Swal.fire({
                icon: 'error',
                title: '오류',
                text: data.message || '스트림 삭제에 실패했습니다.',
                confirmButtonText: '확인'
            });
        }
    } catch (error) {
        console.error('Failed to delete stream:', error);
        Swal.fire({
            icon: 'error',
            title: '오류',
            text: '스트림 삭제에 실패했습니다.',
            confirmButtonText: '확인'
        });
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
                Swal.fire({
                    icon: 'success',
                    title: '성공',
                    text: isEdit ? '스트림이 성공적으로 수정되었습니다.' : '스트림이 성공적으로 추가되었습니다.',
                    timer: 2000,
                    showConfirmButton: false
                });
                closeStreamModal();
                loadStreams();
            } else {
                const data = await response.json();
                Swal.fire({
                    icon: 'error',
                    title: '오류',
                    text: data.message || '스트림 저장에 실패했습니다.',
                    confirmButtonText: '확인'
                });
            }
        } catch (error) {
            console.error('Failed to save stream:', error);
            Swal.fire({
                icon: 'error',
                title: '오류',
                text: '스트림 저장에 실패했습니다.',
                confirmButtonText: '확인'
            });
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

