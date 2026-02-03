// Recording 탭 전용 전역 변수 (탭 진입 시에만 초기화)
let recordingGlobals = {
    // DOM 요소
    timelineViewport: null,
    timelineCanvas: null,
    ruler: null,
    segmentsContainer: null,
    player: null,
    playheadFixed: null,
    playheadOnTimeline: null,
    currentTimeInfo: null,
    visibleRangeTitle: null,
    visibleRangeTime: null,
    status: null,
    
    // 타임라인 관련
    hls: null,
    DAY_SECONDS: 24 * 60 * 60,
    MAX_SEGMENT_TIME: 24 * 60 * 60 + 1800, // 88200초
    segments: [],
    currentSegmentIndex: -1,
    currentSegmentUrl: null,
    GAP_THRESHOLD_SECONDS: 1,
    MEDIA_TIME_TOLERANCE: 0.05,
    pixelsPerSecond: 2,
    timelineOffset: 0,
    currentTimeSec: 0,
    segmentElements: [],
    
    // 줌 레벨
    zoomLevels: [
        { seconds: 3600 * 24, label: '24시간' },
        { seconds: 3600 * 12, label: '12시간' },
        { seconds: 3600 * 8, label: '8시간' },
        { seconds: 3600 * 4, label: '4시간' },
        { seconds: 3600 * 2, label: '2시간' },
        { seconds: 3600, label: '1시간' },
        { seconds: 1800, label: '30분' },
        { seconds: 900, label: '15분' }
    ],
    currentZoomIndex: 4, // 기본: 2시간
    
    // 드래그 관련
    isDragging: false,
    isMouseDown: false,
    dragStartX: 0,
    dragStartOffset: 0,
    hasMoved: false,
    dragStartSegmentIndex: -1,
    
    // 초기화 여부
    initialized: false
};

// Stream 관리 전역 변수 (기존)
let streams = [];
let selectedStream = null;

// 이벤트 핸들러들
let mouseMoveHandler = null;
let timelineMouseDownHandler = null;
let mouseUpHandler = null;
let playerTimeupdateHandler = null;
let timelineViewportWheelHandler = null;

// Recording 탭 초기화 함수
function initializeRecording() {
    const g = recordingGlobals;
    
    if (g.initialized) {
        return; // 이미 초기화됨
    }
    
    // DOM 요소 할당
    g.player = document.getElementById('videoPlayer');
    if (!g.player) {
        return; // Recording 탭이 아직 로드되지 않음
    }
    
    g.timelineViewport = document.getElementById('timelineViewport');
    g.timelineCanvas = document.getElementById('timelineCanvas');
    g.ruler = document.getElementById('ruler');
    g.segmentsContainer = document.getElementById('segmentsContainer');
    g.playheadFixed = document.getElementById('playheadFixed');
    g.playheadOnTimeline = document.getElementById('playheadOnTimeline');
    g.currentTimeInfo = document.getElementById('currentTimeInfo');
    g.visibleRangeTitle = document.getElementById('visibleRangeTitle');
    g.visibleRangeTime = document.getElementById('visibleRangeTime');
    g.status = document.getElementById('status');
    
    if (!g.timelineViewport || !g.timelineCanvas || !g.ruler || !g.segmentsContainer) {
        return; // 필수 DOM 요소가 없음
    }
    
    // 초기값 설정
    if (g.currentTimeInfo) {
        g.currentTimeInfo.textContent = '00:00:00';
    }
    if (g.visibleRangeTitle) {
        g.visibleRangeTitle.textContent = '표시 구간';
    }
    if (g.visibleRangeTime) {
        g.visibleRangeTime.textContent = '| 00:00:00';
    }
    if (g.status) {
        g.status.textContent = '대기중';
        g.status.classList.remove('status--alert');
    }
    
    // 날짜 입력 초기화 (오늘 날짜)
    const dateInput = document.getElementById('dateInput');
    if (dateInput) {
        const today = new Date().toISOString().split('T')[0];
        dateInput.value = today;
        dateInput.max = today;
        
        dateInput.addEventListener('change', function() {
            if (selectedStream) {
                loadRecordingList();
            }
        });
    }
    
    // 이벤트 리스너 등록
    const refreshBtn = document.getElementById('refreshBtn');
    if (refreshBtn) {
        refreshBtn.addEventListener('click', loadRecordingList);
    }
    
    // 이벤트 핸들러 생성
    createEventHandlers();
    
    // Stream 목록 로드
    loadStreamList();
    
    g.initialized = true;
}

// Recording 탭 정리 함수 (탭을 떠날 때 호출)
function cleanupRecording() {
    const g = recordingGlobals;
    
    if (!g.initialized) {
        return;
    }
    
    // 이벤트 리스너 제거
    if (g.timelineCanvas && timelineMouseDownHandler) {
        g.timelineCanvas.removeEventListener('mousedown', timelineMouseDownHandler);
    }
    if (mouseMoveHandler) {
        document.removeEventListener('mousemove', mouseMoveHandler);
    }
    if (mouseUpHandler) {
        document.removeEventListener('mouseup', mouseUpHandler);
    }
    if (g.player && playerTimeupdateHandler) {
        g.player.removeEventListener('timeupdate', playerTimeupdateHandler);
    }
    if (g.timelineViewport && timelineViewportWheelHandler) {
        g.timelineViewport.removeEventListener('wheel', timelineViewportWheelHandler);
    }
    
    // 타임라인 정리
    clearTimeline();
    
    // 전역 변수 초기화
    g.initialized = false;
    selectedStream = null;
}

// 이벤트 핸들러 생성
function createEventHandlers() {
    const g = recordingGlobals;
    
    // 마우스 이동
    mouseMoveHandler = function(e) {
        if (!g.isMouseDown) return;
        
        var deltaX = e.clientX - g.dragStartX;
        
        // 5픽셀 이상 움직이면 드래그로 간주
        if (Math.abs(deltaX) > 5 && !g.isDragging) {
            g.isDragging = true;
            g.timelineCanvas.classList.add('dragging');
        }
        
        if (g.isDragging) {
            g.hasMoved = true;
            
            // 드래그 중에는 비디오 재생 멈춤
            if (!g.player.paused) {
                g.player.pause();
            }
            
            g.timelineOffset = g.dragStartOffset + deltaX;
            
            // 범위 제한 및 시간 계산
            var viewportWidth = g.timelineViewport.clientWidth;
            var zoomSeconds = g.zoomLevels[g.currentZoomIndex].seconds;
            
            g.pixelsPerSecond = viewportWidth / zoomSeconds;
            var currentPixelsPerSecond = g.pixelsPerSecond;
            var paddingSeconds = viewportWidth / currentPixelsPerSecond / 2;
            var maxSegmentTime = g.MAX_SEGMENT_TIME;
            var totalSeconds = maxSegmentTime + paddingSeconds * 2;
            
            var canvasWidth = totalSeconds * currentPixelsPerSecond;
            var minOffset = -(canvasWidth - viewportWidth);
            var maxOffset = 0;
            g.timelineOffset = Math.max(minOffset, Math.min(maxOffset, g.timelineOffset));
            
            // ruler와 canvas 함께 스크롤
            g.ruler.style.transform = 'translateX(' + g.timelineOffset + 'px)';
            g.timelineCanvas.style.transform = 'translateX(' + g.timelineOffset + 'px)';
            var isWideView = zoomSeconds >= 3600*12;
            
            // 중앙의 플레이헤드가 가리키는 시간 계산
            var centerOffset = -g.timelineOffset + viewportWidth / 2;
            var centerTimeSec = centerOffset / currentPixelsPerSecond - paddingSeconds;
            g.currentTimeSec = Math.max(0, Math.min(maxSegmentTime, centerTimeSec));
            
            if(isWideView) {
                var playheadX = (g.currentTimeSec + paddingSeconds) * currentPixelsPerSecond;
                g.playheadOnTimeline.style.left = playheadX + 'px';
            }
            
            updateCurrentTime();
            updateInfo();
            
            // 드래그 중에는 seek하지 않음 (빈 공간에서는 재생할 수 없으므로)
            var foundSegment = false;
            for (var i = 0; i < g.segments.length; i++) {
                var seg = g.segments[i];
                if (isGapSegment(seg)) continue;
                if (g.currentTimeSec >= seg.startSec && g.currentTimeSec < seg.endSec) {
                    foundSegment = true;
                    break;
                }
            }
            
            if (foundSegment) {
                seekToTime(g.currentTimeSec, true);
            }
        }
    };
    
    // 타임라인에 마우스 클릭 (드래그 이벤트)
    timelineMouseDownHandler = function(e) {
        g.isMouseDown = true;
        g.dragStartX = e.clientX;
        g.dragStartOffset = g.timelineOffset;
        g.hasMoved = false;
        g.dragStartSegmentIndex = g.currentSegmentIndex;
        e.preventDefault();
    };
    
    mouseUpHandler = function(e) {
        if (!g.isMouseDown) return;
        
        if (g.hasMoved && g.isDragging) {
            // 드래그 종료 시: 현재 시간이 세그먼트 범위에 있는지 확인
            var foundSegment = false;
            var targetSegmentIndex = -1;
            
            for (var i = 0; i < g.segments.length; i++) {
                var seg = g.segments[i];
                if (isGapSegment(seg)) continue;
                if (g.currentTimeSec >= seg.startSec && g.currentTimeSec < seg.endSec) {
                    foundSegment = true;
                    targetSegmentIndex = i;
                    break;
                }
            }
            
            if (!foundSegment) {
                var adjustedTime = skipForwardOverGap(g.currentTimeSec);
                if (adjustedTime !== g.currentTimeSec) {
                    g.currentTimeSec = adjustedTime;
                    var resolved = findSegmentContainingTime(g.currentTimeSec);
                    if (resolved) {
                        foundSegment = true;
                        targetSegmentIndex = resolved.index;
                    }
                }
            }
            
            // 세그먼트 범위에 없으면 드래그 시작 세그먼트의 시작 또는 끝으로 이동
            if (!foundSegment && g.dragStartSegmentIndex >= 0 && g.dragStartSegmentIndex < g.segments.length) {
                var startSeg = g.segments[g.dragStartSegmentIndex];
                if (g.currentTimeSec < startSeg.startSec) {
                    g.currentTimeSec = startSeg.startSec + 0.5;
                } else if (g.currentTimeSec > startSeg.endSec) {
                    g.currentTimeSec = startSeg.endSec - 0.5;
                }
                targetSegmentIndex = g.dragStartSegmentIndex;
            }
            
            // 세그먼트 재생
            if (targetSegmentIndex >= 0 && targetSegmentIndex < g.segments.length) {
                var seg = g.segments[targetSegmentIndex];
                var relativeTime = (seg.mediaOffset || 0) + (g.currentTimeSec - seg.startSec);
                if (relativeTime < 0) {
                    relativeTime = 0;
                }
                g.currentSegmentIndex = targetSegmentIndex;
                
                if (g.currentSegmentUrl !== seg.url) {
                    playHLS(seg.url, relativeTime, seg.isLive);
                } else {
                    g.player.currentTime = relativeTime;
                }
                
                updateTimeline();
            }
        } else {
            // 클릭 (드래그 없음): 클릭한 위치의 세그먼트 찾기
            var clickedElement = e.target;
            
            if (clickedElement.classList.contains('segment') && !clickedElement.classList.contains('segment-gap')) {
                var index = parseInt(clickedElement.getAttribute('data-index'));
                var seg = g.segments[index];
                seekToTime(seg.startSec, false);
            }
        }
        
        g.isMouseDown = false;
        g.isDragging = false;
        g.hasMoved = false;
        g.dragStartSegmentIndex = -1;
        g.timelineCanvas.classList.remove('dragging');
        
        if (g.player.paused) {
            g.player.play();
        }
    };
    
    playerTimeupdateHandler = function() {
        if (g.currentSegmentIndex < 0) return;
        
        var seg = g.segments[g.currentSegmentIndex];
        if (!seg || isGapSegment(seg)) return;
        
        var mediaTime = g.player.currentTime;
        var baseOffset = seg.mediaOffset || 0;
        var relativeElapsed = mediaTime - baseOffset;
        
        // relativeElapsed가 세그먼트 범위를 벗어난 경우 처리
        if (relativeElapsed < -g.MEDIA_TIME_TOLERANCE || relativeElapsed > seg.duration + g.MEDIA_TIME_TOLERANCE) {
            var resolved = findSegmentByMediaPosition(seg.url, mediaTime);
            if (resolved) {
                g.currentSegmentIndex = resolved.index;
                seg = resolved.segment;
                baseOffset = resolved.mediaOffset;
                relativeElapsed = mediaTime - baseOffset;
            } else if (relativeElapsed < -g.MEDIA_TIME_TOLERANCE) {
                relativeElapsed = 0;
            }
        }
        
        g.currentTimeSec = seg.startSec + Math.min(relativeElapsed, seg.duration);
        
        // 드래그 중이 아닐 때만 타임라인 자동 스크롤
        if (!g.isDragging && !g.isMouseDown) {
            updateTimeline();
        } else {
            updateCurrentTime();
            updateInfo();
            var viewportWidth = g.timelineViewport.clientWidth;
            var paddingSeconds = viewportWidth / g.pixelsPerSecond / 2;
            updateSegmentPositions(paddingSeconds);
        }
    };
    
    // 마우스 휠로 줌
    timelineViewportWheelHandler = function(e) {
        e.preventDefault();
        
        if (e.deltaY < 0) {
            // 줌 인
            if (g.currentZoomIndex < g.zoomLevels.length - 1) {
                g.currentZoomIndex++;
            } else {
                return;
            }
        } else {
            // 줌 아웃
            if (g.currentZoomIndex > 0) {
                g.currentZoomIndex--;
            } else {
                return;
            }
        }
        
        updateTimeline();
    };
}

// 이벤트 리스너 등록
function registEventListeners() {
    const g = recordingGlobals;
    
    if (g.timelineCanvas && timelineMouseDownHandler) {
        g.timelineCanvas.removeEventListener('mousedown', timelineMouseDownHandler);
        g.timelineCanvas.addEventListener('mousedown', timelineMouseDownHandler);
    }
    
    if (mouseMoveHandler) {
        document.removeEventListener('mousemove', mouseMoveHandler);
        document.addEventListener('mousemove', mouseMoveHandler);
    }
    
    if (mouseUpHandler) {
        document.removeEventListener('mouseup', mouseUpHandler);
        document.addEventListener('mouseup', mouseUpHandler);
    }
    
    if (g.player && playerTimeupdateHandler) {
        g.player.removeEventListener('timeupdate', playerTimeupdateHandler);
        g.player.addEventListener('timeupdate', playerTimeupdateHandler);
    }
    
    if (g.timelineViewport && timelineViewportWheelHandler) {
        g.timelineViewport.removeEventListener('wheel', timelineViewportWheelHandler);
        try {
            g.timelineViewport.addEventListener('wheel', timelineViewportWheelHandler, { passive: false });
        } catch(e) {
            g.timelineViewport.addEventListener('wheel', timelineViewportWheelHandler);
        }
    }
}

// WebSocket으로부터 받은 스트림 목록 업데이트 (main.js에서 호출)
function updateRecordingStreamList(streamsData) {
    if (streamsData && Array.isArray(streamsData)) {
        streams = streamsData;
        renderStreamList();
    }
}

// Stream 목록 가져오기
async function loadStreamList() {
    // WebSocket이 연결되어 있고 이미 스트림 목록을 받았다면 그대로 사용
    if (typeof healthWebSocket !== 'undefined' && 
        healthWebSocket && 
        healthWebSocket.readyState === WebSocket.OPEN && 
        streams.length > 0) {
        renderStreamList();
        return;
    }
    
    // WebSocket이 연결되지 않았거나 스트림 목록이 없을 때만 HTTP API 사용
    try {
        const response = await fetch('/api/streams');
        if (!response.ok) {
            throw new Error('Failed to load streams');
        }
        
        const data = await response.json();
        streams = data.streams || [];
        renderStreamList();
    } catch (error) {
        console.error('Error loading streams:', error);
        updateStatus('스트림 목록을 불러오는데 실패했습니다.', true);
    }
}

// Stream 목록 렌더링
function renderStreamList() {
    const streamList = document.getElementById('recordingStreamList');
    if (!streamList) {
        return;
    }
    streamList.innerHTML = '';
    
    if (!Array.isArray(streams)) {
        console.error('streams is not an array:', streams);
        streams = [];
    }
    
    if (streams.length === 0) {
        streamList.innerHTML = '<div class="stream-item" style="text-align: center; color: #999; padding: 20px;">스트림이 없습니다</div>';
        return;
    }
    
    streams.forEach(stream => {
        const item = document.createElement('div');
        item.className = 'stream-item';
        if (selectedStream && selectedStream.streamID === stream.streamID) {
            item.classList.add('selected');
        }
        item.textContent = stream.name;
        item.addEventListener('click', function(e) {
            selectStream(stream, e.currentTarget);
        });
        streamList.appendChild(item);
    });
}

// Stream 선택
function selectStream(stream, element) {
    selectedStream = stream;
    
    document.querySelectorAll('.stream-item').forEach(item => {
        item.classList.remove('selected');
    });
    if (element) {
        element.classList.add('selected');
    }
    
    updateStatus('대기중', false);
    loadRecordingList();
}

// 녹화 목록 가져오기
async function loadRecordingList() {
    if (!selectedStream) {
        updateStatus('대기중', false);
        return;
    }
    
    const dateInput = document.getElementById('dateInput');
    const date = dateInput.value;
    
    if (!date) {
        updateStatus('대기중', false);
        return;
    }
    
    updateStatus('녹화 파일을 불러오는 중...', false);
    
    try {
        const url = `/proxy/core/m3u8List?streamID=${encodeURIComponent(selectedStream.streamID)}&date=${encodeURIComponent(date)}`;
        const response = await fetch(url);
        
        if (!response.ok) {
            throw new Error('Failed to load recording list');
        }
        
        const data = await response.json();
        
        // 응답 데이터 처리
        initTimeline(data);
        
    } catch (error) {
        console.error('Error loading recording list:', error);
        updateStatus('녹화 파일이 없습니다.', true);
    }
}

// 상태 메시지 업데이트
function updateStatus(message, isError) {
    const g = recordingGlobals;
    if (!g.status) {
        return;
    }
    g.status.textContent = message;
    if (isError) {
        g.status.classList.add('status--alert');
    } else {
        g.status.classList.remove('status--alert');
    }
}

// 타임라인 초기화
function initTimeline(data) {
    const g = recordingGlobals;
    clearTimeline();
    
    // API 응답 데이터를 segments로 변환
    if (data) {
        initSegments(data);
    }
    
    g.currentZoomIndex = 4; // 2시간으로 리셋
    registEventListeners();
}

// 기존 세그먼트 및 상태 정리
function clearTimeline() {
    const g = recordingGlobals;
    
    // HLS 인스턴스 정리
    if (g.hls) {
        g.hls.destroy();
        g.hls = null;
    }
    
    // 비디오 정지 및 초기화
    if (g.player) {
        g.player.pause();
        g.player.removeAttribute('src');
        g.player.load();
    }
    
    // 세그먼트 데이터 초기화
    g.segments = [];
    g.segmentElements = [];
    g.currentSegmentIndex = -1;
    g.currentSegmentUrl = null;
    g.currentTimeSec = 0;
    
    // DOM 초기화
    if (g.segmentsContainer) {
        g.segmentsContainer.innerHTML = '';
    }
    if (g.ruler) {
        g.ruler.innerHTML = '';
    }
    
    // 타임라인 상태 초기화
    g.timelineOffset = 0;
    g.isDragging = false;
    g.isMouseDown = false;
    g.hasMoved = false;
    
    // 표시 시간 초기화
    if (g.currentTimeInfo) {
        g.currentTimeInfo.textContent = '00:00:00';
    }
    if (g.visibleRangeTime) {
        g.visibleRangeTime.textContent = '00:00:00';
    }
    
    // 플레이헤드 숨김
    if (g.playheadFixed) {
        g.playheadFixed.style.display = 'none';
    }
    if (g.playheadOnTimeline) {
        g.playheadOnTimeline.style.display = 'none';
    }
}

// 세그먼트 초기화 (API 응답 데이터 사용)
function initSegments(data) {
    const g = recordingGlobals;
    
    // API 응답 구조 파싱
    // 응답 구조: { payload: { recordings: [...] }, status: 1 }
    let recordingData = [];
    
    if (data && data.payload && Array.isArray(data.payload.recordings)) {
        recordingData = data.payload.recordings;
    } else if (data && Array.isArray(data.recordings)) {
        recordingData = data.recordings;
    } else if (Array.isArray(data)) {
        recordingData = data;
    } else if (data && Array.isArray(data.data)) {
        recordingData = data.data;
    }
    
    if (recordingData.length === 0) {
        updateStatus('녹화 파일이 없습니다.', true);
        return;
    }
    
    var recordingBlocks = [];
    var streamID = selectedStream ? selectedStream.streamID : '';
    
    for (var i = 0; i < recordingData.length; i++) {
        var row = recordingData[i];
        var content = row.m3u8Content || row.content || '';
        var startTime = row.startTime || '00:00:00';
        var fileName = row.fileName || row.filename || '';
        var channelId = row.channelId || '0';
        
        if (!content) continue;
        
        var urlAddress = window.location.origin + '/proxy/core/m3u8?';
        urlAddress += "StreamID=" + streamID;
        urlAddress += "&ChannelID=" + channelId;
        urlAddress += "&Filename=" + encodeURIComponent(fileName);
        
        var inferredStartSec = getFirstTsStartSec(content);
        var defaultStartSec = inferredStartSec !== null ? inferredStartSec : timeToSeconds(startTime);
        var blocks = createRecordingBlocksFromContent(content, defaultStartSec, urlAddress);
        recordingBlocks = recordingBlocks.concat(blocks);
    }
    
    g.segments = mergeRecordingBlocksWithGaps(recordingBlocks);
    
    var recordingCount = 0;
    for (var j = 0; j < g.segments.length; j++) {
        if (!isGapSegment(g.segments[j])) {
            recordingCount++;
        }
    }
    
    if(recordingCount == 0) {
        updateStatus('녹화 파일이 없습니다.', true);
    } else {
        updateStatus(recordingCount + '개의 녹화 구간이 발견되었습니다.', false);
    }
    
    createSegments();
    
    var firstPlayableIndex = findNextRecordingSegment(-1);
    if (firstPlayableIndex >= 0) {
        g.currentSegmentIndex = firstPlayableIndex;
        g.currentTimeSec = g.segments[firstPlayableIndex].startSec;
        updateTimeline();
        var seg = g.segments[firstPlayableIndex];
        playHLS(seg.url, seg.mediaOffset || 0, seg.isLive);
    }
}

// 세그먼트 DOM 요소 생성
function createSegments() {
    const g = recordingGlobals;
    g.segmentElements = [];
    
    for (var i = 0; i < g.segments.length; i++) {
        var seg = g.segments[i];
        var segDiv = document.createElement('div');
        if (isGapSegment(seg)) {
            segDiv.className = 'segment segment-gap';
            segDiv.title = 'No Recording';
        } else {
            segDiv.className = 'segment';
            segDiv.title = seg.startTime + ' (' + Math.floor(seg.duration) + ' Sec)';
            segDiv.setAttribute('data-index', i);
        }
        
        g.segmentsContainer.appendChild(segDiv);
        g.segmentElements.push(segDiv);
    }
}

// 세그먼트 위치만 업데이트
function updateSegmentPositions(paddingSeconds) {
    const g = recordingGlobals;
    
    if (typeof paddingSeconds === 'undefined') {
        var viewportWidth = g.timelineViewport.clientWidth;
        var currentPixelsPerSecond = g.pixelsPerSecond;
        paddingSeconds = viewportWidth / currentPixelsPerSecond / 2;
    }
    
    var currentPixelsPerSecond = g.pixelsPerSecond;
    
    for (var i = 0; i < g.segments.length; i++) {
        if (g.segmentElements[i]) {
            var seg = g.segments[i];
            var left = (seg.startSec + paddingSeconds) * currentPixelsPerSecond;
            var width = seg.duration * currentPixelsPerSecond;
            
            g.segmentElements[i].style.left = left + 'px';
            g.segmentElements[i].style.width = width + 'px';
            
            // playhead 위치 기준으로 이전/이후
            g.segmentElements[i].classList.remove('segment-past', 'segment-future', 'segment-current');
            g.segmentElements[i].removeAttribute('data-playhead-ratio');
            
            if(seg.endSec <= g.currentTimeSec) {
                g.segmentElements[i].classList.add('segment-past');
            } else if (seg.startSec > g.currentTimeSec) {
                g.segmentElements[i].classList.add('segment-future');
            } else if (seg.startSec <= g.currentTimeSec && g.currentTimeSec < seg.endSec) {
                g.segmentElements[i].classList.add('segment-current');
                var playheadRatio = (g.currentTimeSec - seg.startSec) / seg.duration;
                g.segmentElements[i].style.setProperty('--playhead-ratio', (playheadRatio * 100) + '%');
            }
        }
    }
}

// 타임라인 렌더링
function updateTimeline() {
    const g = recordingGlobals;
    var viewportWidth = g.timelineViewport.clientWidth;
    
    var zoomSeconds = g.zoomLevels[g.currentZoomIndex].seconds;
    g.pixelsPerSecond = viewportWidth / zoomSeconds;
    
    var maxSegmentTime = g.MAX_SEGMENT_TIME;
    var paddingSeconds = viewportWidth / g.pixelsPerSecond / 2;
    var totalSeconds = maxSegmentTime + paddingSeconds * 2;
    var canvasWidth = totalSeconds * g.pixelsPerSecond;
    g.timelineCanvas.style.width = canvasWidth + 'px';
    
    g.ruler.style.width = canvasWidth + 'px';
    
    try {
        var viewportRect = g.timelineViewport.getBoundingClientRect();
        var rulerParent = g.ruler.parentElement;
        
        if (rulerParent) {
            var parentRect = rulerParent.getBoundingClientRect();
            var rulerPosition = window.getComputedStyle(g.ruler).position;
            
            if (rulerPosition === 'absolute' || rulerPosition === 'relative') {
                var leftOffset = viewportRect.left - parentRect.left;
                g.ruler.style.left = leftOffset + 'px';
            }
        }
    } catch (e) {
        console.log('Ruler positioning error:', e);
    }
    
    var isWideView = zoomSeconds >= 3600 * 12;
    
    if (isWideView) {
        g.playheadFixed.style.display = 'none';
        g.playheadOnTimeline.style.display = 'block';
        
        var playheadX = (g.currentTimeSec + paddingSeconds) * g.pixelsPerSecond;
        g.playheadOnTimeline.style.left = playheadX + 'px';
        
        var desiredOffset = -(playheadX - viewportWidth / 2);
        g.timelineOffset = desiredOffset;
    } else {
        g.playheadFixed.style.display = 'block';
        g.playheadOnTimeline.style.display = 'none';
        
        var centerOffset = (g.currentTimeSec + paddingSeconds) * g.pixelsPerSecond;
        g.timelineOffset = -(centerOffset - viewportWidth / 2);
    }
    
    var minOffset = -(canvasWidth - viewportWidth);
    var maxOffset = 0;
    g.timelineOffset = Math.max(minOffset, Math.min(maxOffset, g.timelineOffset));
    
    g.ruler.style.transform = 'translateX(' + g.timelineOffset + 'px)';
    g.timelineCanvas.style.transform = 'translateX(' + g.timelineOffset + 'px)';
    
    renderTimeScale(paddingSeconds, maxSegmentTime);
    updateSegmentPositions(paddingSeconds);
    updateCurrentTime();
    updateInfo();
}

// 시간 눈금 렌더링
function renderTimeScale(paddingSeconds, maxTime) {
    const g = recordingGlobals;
    g.ruler.innerHTML = '';
    
    var zoomSeconds = g.zoomLevels[g.currentZoomIndex].seconds;
    var currentPixelsPerSecond = g.pixelsPerSecond;
    
    if (typeof paddingSeconds === 'undefined') {
        var viewportWidth = g.timelineViewport.clientWidth;
        paddingSeconds = viewportWidth / currentPixelsPerSecond / 2;
    }
    
    if (typeof maxTime === 'undefined') {
        maxTime = g.DAY_SECONDS;
    }
    
    var tickInterval = 3600;
    var majorInterval = 3600;
    
    if (zoomSeconds >= 3600 * 12) {
        tickInterval = 3600;
        majorInterval = 7200;
    } else if (zoomSeconds >= 3600 * 8) {
        tickInterval = 1800;
        majorInterval = 3600;
    } else if (zoomSeconds >= 3600 * 4) {
        tickInterval = 900;
        majorInterval = 1800;
    } else if (zoomSeconds >= 3600 * 2) {
        tickInterval = 300;
        majorInterval = 900;
    } else if (zoomSeconds >= 3600) {
        tickInterval = 60;
        majorInterval = 600;
    } else if (zoomSeconds >= 900) {
        tickInterval = 60;
        majorInterval = 300;
    }
    
    for (var sec = 0; sec <= maxTime; sec += tickInterval) {
        var x = (sec + paddingSeconds) * currentPixelsPerSecond;
        var isMajor = sec % majorInterval === 0;
        
        var tick = document.createElement('div');
        tick.className = isMajor ? 'time-tick major' : 'time-tick';
        tick.style.left = x + 'px';
        g.ruler.appendChild(tick);
        
        if (isMajor) {
            var label = document.createElement('div');
            label.className = 'time-label';
            label.style.left = x + 'px';
            label.textContent = secondsToTime(sec);
            g.ruler.appendChild(label);
        }
    }
}

// 정보 업데이트
function updateInfo() {
    const g = recordingGlobals;
    var viewportWidth = g.timelineViewport.clientWidth;
    var currentPixelsPerSecond = g.pixelsPerSecond;
    var paddingSeconds = viewportWidth / currentPixelsPerSecond / 2;
    
    var canvasCenterX = -g.timelineOffset + viewportWidth / 2;
    var viewCenterSec = canvasCenterX / currentPixelsPerSecond - paddingSeconds;
    var viewStartSec = viewCenterSec - (viewportWidth / 2 / currentPixelsPerSecond);
    var viewEndSec = viewCenterSec + (viewportWidth / 2 / currentPixelsPerSecond);
    
    if (g.visibleRangeTime) {
        g.visibleRangeTime.textContent = secondsToTime(Math.max(0, viewStartSec)) + 
            ' ~ ' + secondsToTime(Math.min(g.MAX_SEGMENT_TIME, viewEndSec));
    }
}

// 현재 시간 정보 업데이트
function updateCurrentTime() {
    const g = recordingGlobals;
    var timeStr = secondsToTime(Math.max(0, Math.min(g.MAX_SEGMENT_TIME, g.currentTimeSec)));
    if (g.currentTimeInfo) {
        g.currentTimeInfo.textContent = timeStr;
    }
}

// 타임라인의 특정 시간으로 이동
function seekToTime(targetSec, skipTimelineUpdate) {
    const g = recordingGlobals;
    if (!g.segments.length) return;
    
    var adjustedTarget = skipForwardOverGap(targetSec);
    var resolved = findSegmentContainingTime(adjustedTarget);
    
    if (!resolved) {
        for (var i = g.segments.length - 1; i >= 0; i--) {
            var candidate = g.segments[i];
            if (isGapSegment(candidate)) continue;
            if (adjustedTarget >= candidate.endSec) {
                adjustedTarget = candidate.endSec - 0.05;
                if (adjustedTarget < candidate.startSec) {
                    adjustedTarget = candidate.startSec;
                }
                resolved = { index: i, segment: candidate };
                break;
            }
        }
    }
    
    if (!resolved) {
        var firstIdx = findNextRecordingSegment(-1);
        if (firstIdx === -1) return;
        resolved = { index: firstIdx, segment: g.segments[firstIdx] };
        adjustedTarget = resolved.segment.startSec;
    }
    
    g.currentSegmentIndex = resolved.index;
    g.currentTimeSec = adjustedTarget;
    
    if(!skipTimelineUpdate) {
        updateTimeline();
    } else {
        var viewportWidth = g.timelineViewport.clientWidth;
        var paddingSeconds = viewportWidth / g.pixelsPerSecond / 2;
        updateSegmentPositions(paddingSeconds);
    }
    
    var seg = resolved.segment;
    var relativeTime = (seg.mediaOffset || 0) + (adjustedTarget - seg.startSec);
    if (relativeTime < 0) {
        relativeTime = 0;
    }
    
    if (g.currentSegmentUrl === seg.url && g.hls) {
        g.player.currentTime = relativeTime;
    } else {
        playHLS(seg.url, relativeTime, seg.isLive);
    }
}

// HLS 재생
function playHLS(url, seekTime, isLive) {
    const g = recordingGlobals;
    if (typeof seekTime === 'undefined') seekTime = 0;
    
    if (g.currentSegmentUrl === url && g.hls) {
        try {
            if (typeof seekTime === 'number' && seekTime >= 0) {
                g.player.currentTime = seekTime;
            }
        } catch(e) {
            console.log('동일 URL seek 실패:', e);
        }
        if (g.player.paused) {
            g.player.play().catch(function(err) {
                console.log('자동 재생 실패:', err);
            });
        }
        return;
    }
    
    g.currentSegmentUrl = url;
    
    if (g.hls) {
        g.hls.destroy();
    }
    
    if (typeof Hls !== 'undefined' && Hls.isSupported()) {
        g.hls = new Hls({
            enableWorker: true,
            lowLatencyMode: false
        });
        
        g.hls.loadSource(url);
        g.hls.attachMedia(g.player);
        
        g.hls.on(Hls.Events.MANIFEST_PARSED, function() {
            if (seekTime > 0) {
                g.player.currentTime = seekTime;
            } else if(!isLive){
                g.player.currentTime = 1;
            }
            g.player.play().catch(function(err) {
                console.log('자동 재생 실패:', err);
            });
        });
        
        g.hls.on(Hls.Events.ERROR, function(event, data) {
            console.error('HLS Error:', data);
            if (data.fatal) {
                updateStatus('오류: ' + data.type, true);
            }
        });
    } else if (g.player.canPlayType('application/vnd.apple.mpegurl')) {
        g.player.src = url;
        g.player.currentTime = 0.2;
        if (seekTime > 0) {
            g.player.currentTime = seekTime;
        }
        g.player.play();
    } else {
        Swal.fire({
            icon: 'warning',
            title: '브라우저 호환성',
            text: 'HLS 재생을 지원하지 않는 브라우저입니다.',
            confirmButtonText: '확인'
        });
    }
}

// 유틸리티 함수들
function timeToSeconds(timeStr) {
    var parts = timeStr.split(':');
    var h = parseInt(parts[0], 10);
    var m = parseInt(parts[1], 10);
    var s = parseInt(parts[2], 10);
    return h * 3600 + m * 60 + s;
}

function secondsToTime(seconds) {
    var h = Math.floor(seconds / 3600);
    var m = Math.floor((seconds % 3600) / 60);
    var s = Math.floor(seconds % 60);
    
    var hh = String(h).length < 2 ? '0' + h : h;
    var mm = String(m).length < 2 ? '0' + m : m;
    var ss = String(s).length < 2 ? '0' + s : s;
    
    return hh + ':' + mm + ':' + ss;
}

function parseDuration(text) {
    var matches = text.match(/#EXTINF:([\d.]+)/g);
    if (!matches) return 0;
    
    var sum = 0;
    for (var i = 0; i < matches.length; i++) {
        var value = parseFloat(matches[i].split(':')[1]);
        sum += value;
    }
    return sum;
}

function extractSecondsFromTsFilename(name, baseDate) {
    if (!name) return null;
    var trimmed = name.trim();
    var match = trimmed.match(/_(\d{8})_(\d{6})\.ts$/i);
    if (!match) return null;
    
    var fileDate = match[1];
    var timeStr = match[2];
    var h = parseInt(timeStr.substring(0, 2), 10);
    var m = parseInt(timeStr.substring(2, 4), 10);
    var s = parseInt(timeStr.substring(4, 6), 10);
    if (isNaN(h) || isNaN(m) || isNaN(s)) return null;
    
    var baseSeconds = h * 3600 + m * 60 + s;
    
    if (h === 0 && baseSeconds <= 1800 && baseDate && fileDate !== baseDate) {
        return 86400 + baseSeconds;
    }
    
    return baseSeconds;
}

function parseM3u8ToEntries(content, defaultStartSec, urlAddress) {
    if (!content) return [];
    
    var lines = content.split(/\r?\n/);
    var pendingDuration = null;
    var cumulativeOffset = 0;
    var fallbackStartSec = typeof defaultStartSec === 'number' ? defaultStartSec : 0;
    var entries = [];
    var baseDate = null;
    
    for (var i = 0; i < lines.length; i++) {
        var line = lines[i].trim();
        if (!line) continue;
        
        if (line.indexOf('#EXTINF:') === 0) {
            var value = line.split(':')[1];
            pendingDuration = parseFloat(value);
            continue;
        }
        
        if (line.charAt(0) === '#') {
            continue;
        }
        
        if (pendingDuration === null) {
            continue;
        }
        
        if (baseDate === null) {
            var dateMatch = line.match(/_(\d{8})_/);
            if (dateMatch) {
                baseDate = dateMatch[1];
            }
        }
        
        var startSec = extractSecondsFromTsFilename(line, baseDate);
        if (startSec === null) {
            startSec = fallbackStartSec;
        }
        
        var entry = {
            url: urlAddress,
            startSec: startSec,
            endSec: startSec + pendingDuration,
            duration: pendingDuration,
            mediaOffset: cumulativeOffset,
            mediaEndOffset: cumulativeOffset + pendingDuration
        };
        
        entries.push(entry);
        cumulativeOffset += pendingDuration;
        fallbackStartSec = startSec + pendingDuration;
        pendingDuration = null;
    }
    
    entries.sort(function(a, b) {
        return a.startSec - b.startSec;
    });
    
    return entries;
}

function mergeEntriesToBlocks(entries, urlAddress) {
    if (!entries || !entries.length) {
        return [];
    }
    
    const g = recordingGlobals;
    var blocks = [];
    var currentBlock = null;
    
    for (var j = 0; j < entries.length; j++) {
        var entryItem = entries[j];
        
        if (!currentBlock) {
            currentBlock = {
                url: urlAddress,
                startSec: entryItem.startSec,
                endSec: entryItem.endSec,
                mediaOffset: entryItem.mediaOffset,
                mediaEndOffset: entryItem.mediaEndOffset
            };
            continue;
        }
        
        var diff = entryItem.startSec - currentBlock.endSec;
        
        if (diff > g.GAP_THRESHOLD_SECONDS) {
            blocks.push(currentBlock);
            currentBlock = {
                url: urlAddress,
                startSec: entryItem.startSec,
                endSec: entryItem.endSec,
                mediaOffset: entryItem.mediaOffset,
                mediaEndOffset: entryItem.mediaEndOffset
            };
        } else {
            currentBlock.endSec = entryItem.endSec;
            currentBlock.mediaEndOffset = entryItem.mediaEndOffset;
        }
    }
    
    if (currentBlock) {
        blocks.push(currentBlock);
    }
    
    return blocks;
}

function formatBlocks(blocks, isLive) {
    if (!blocks || !blocks.length) {
        return [];
    }
    
    var formattedBlocks = [];
    for (var k = 0; k < blocks.length; k++) {
        var block = blocks[k];
        formattedBlocks.push({
            url: block.url,
            startSec: block.startSec,
            endSec: block.endSec,
            duration: block.endSec - block.startSec,
            startTime: secondsToTime(block.startSec),
            mediaOffset: block.mediaOffset,
            mediaEndOffset: block.mediaEndOffset,
            isGap: false,
            isLive: typeof isLive === 'boolean' ? isLive : false
        });
    }
    
    return formattedBlocks;
}

function createRecordingBlocksFromContent(content, defaultStartSec, urlAddress) {
    var entries = parseM3u8ToEntries(content, defaultStartSec, urlAddress);
    if (!entries.length) {
        return [];
    }
    
    var blocks = mergeEntriesToBlocks(entries, urlAddress);
    var isLive = !hasEndList(content);
    var formattedBlocks = formatBlocks(blocks, isLive);
    
    return formattedBlocks;
}

function hasEndList(content) {
    if (!content) return false;
    var lines = content.split(/\r?\n/);
    for (var i = 0; i < lines.length; i++) {
        var line = lines[i].trim();
        if (line === '#EXT-X-ENDLIST') {
            return true;
        }
    }
    return false;
}

function mergeRecordingBlocksWithGaps(blocks) {
    const g = recordingGlobals;
    
    if (!blocks || !blocks.length) {
        return [];
    }
    
    blocks.sort(function(a, b) {
        return a.startSec - b.startSec;
    });
    
    var merged = [];
    var prevEnd = null;
    
    for (var i = 0; i < blocks.length; i++) {
        var block = blocks[i];
        
        if (prevEnd !== null && block.startSec - prevEnd > g.GAP_THRESHOLD_SECONDS) {
            merged.push({
                isGap: true,
                startSec: prevEnd,
                endSec: block.startSec,
                duration: block.startSec - prevEnd
            });
        }
        
        merged.push(block);
        prevEnd = block.endSec;
    }
    
    return merged;
}

function isGapSegment(segment) {
    return segment && segment.isGap;
}

function findSegmentContainingTime(timeSec) {
    const g = recordingGlobals;
    if (!g.segments || !g.segments.length) return null;
    for (var i = 0; i < g.segments.length; i++) {
        var seg = g.segments[i];
        if (isGapSegment(seg)) continue;
        if (timeSec >= seg.startSec && timeSec < seg.endSec) {
            return { index: i, segment: seg };
        }
    }
    return null;
}

function findNextRecordingSegment(fromIndex) {
    const g = recordingGlobals;
    if (!g.segments || !g.segments.length) return -1;
    var start = typeof fromIndex === 'number' ? fromIndex + 1 : 0;
    for (var i = start; i < g.segments.length; i++) {
        if (!isGapSegment(g.segments[i])) {
            return i;
        }
    }
    return -1;
}

function skipForwardOverGap(timeSec) {
    const g = recordingGlobals;
    if (!g.segments || !g.segments.length) return timeSec;
    for (var i = 0; i < g.segments.length; i++) {
        var seg = g.segments[i];
        if (!isGapSegment(seg)) continue;
        if (timeSec >= seg.startSec && timeSec < seg.endSec) {
            return seg.endSec;
        }
    }
    return timeSec;
}

function findSegmentByMediaPosition(url, mediaTime) {
    const g = recordingGlobals;
    if (!g.segments || !g.segments.length) return null;
    if (!url && typeof mediaTime !== 'number') return null;
    
    for (var i = 0; i < g.segments.length; i++) {
        var seg = g.segments[i];
        if (isGapSegment(seg)) continue;
        if (seg.url !== url) continue;
        
        var startOffset = seg.mediaOffset || 0;
        var endOffset = typeof seg.mediaEndOffset === 'number' ? seg.mediaEndOffset : (startOffset + seg.duration);
        
        if (mediaTime >= startOffset - g.MEDIA_TIME_TOLERANCE && mediaTime < endOffset + g.MEDIA_TIME_TOLERANCE) {
            return {
                index: i,
                segment: seg,
                mediaOffset: startOffset
            };
        }
    }
    return null;
}

function getFirstTsStartSec(content) {
    if (!content) return null;
    var lines = content.split(/\r?\n/);
    for (var i = 0; i < lines.length; i++) {
        var line = lines[i].trim();
        if (!line || line.charAt(0) === '#') continue;
        var startSec = extractSecondsFromTsFilename(line);
        if (startSec !== null) {
            return startSec;
        }
    }
    return null;
}

// dialogAlert 주석처리 (사용하지 않음)
// function dialogAlert(app, title, message) {
//     alert(title + ': ' + message);
// }
