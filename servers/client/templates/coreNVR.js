/************************************************
 * terminalRecordingEntireVideo.js
 * Created at Nov 10, 2025 9:34:17 AM.
 *
 * @author SW2Team
 ************************************************/
var dataManager;

// DOM요소
var timelineViewport;
var timelineCanvas;
var ruler;

var segmentsContainer;
var player;
var playheadFixed;
var playheadOnTimeline;

var currentTimeInfo;
var visibleRangeTitle;
var visibleRangeTime;


var status;

// 전역 변수
var hls = null;
var DAY_SECONDS = 24 * 60 * 60;

// 타임라인 최대 시간: 24시간 30분 (88200초)
var MAX_SEGMENT_TIME = DAY_SECONDS + 1800; // 86400 + 1800 = 88200

var segments = [];
var currentSegmentIndex = -1;
var currentSegmentUrl = null;
var GAP_THRESHOLD_SECONDS = 1; // 실제 타임라인 상에서 녹화 공백 판단 기준 (초)
var MEDIA_TIME_TOLERANCE = 0.05; // 플레이어 currentTime과 세그먼트의 미디어 위치 비교 시 허용 오차

// 타임라인 설정
var pixelsPerSecond = 2; // 1초당 픽셀 수 (동적 계산)
var timelineOffset = 0; // 타임라인 스크롤 오프셋
var currentTimeSec = 0; // 현재 재생 위치 (초)

// 세그먼트 DOM 캐시 (깜빡임 방지)
var segmentElements = [];

// 줌 레벨 설정
var zoomLevels = [
  { seconds: 3600 * 24, label: '24시간' },
  { seconds: 3600 * 12, label: '12시간' },
  { seconds: 3600 * 8, label: '8시간' },
  { seconds: 3600 * 4, label: '4시간' },
  { seconds: 3600 * 2, label: '2시간' },
  { seconds: 3600, label: '1시간' },
  { seconds: 1800, label: '30분' },
  { seconds: 900, label: '15분' }
];
var currentZoomIndex = 4; // 기본: 2시간

// 드래그 관련
var isDragging = false;
var isMouseDown = false;
var dragStartX = 0;
var dragStartOffset = 0;
var hasMoved = false;
var dragStartSegmentIndex = -1; // 드래그 시작 시점의 세그먼트 인덱스

// 이벤트 핸들러 -----------------------------------------------------------------------------------------------
// 마우스 이동
var mouseMoveHandler = function(e) {
  if (!isMouseDown) return;

  var deltaX = e.clientX - dragStartX;

    // 5픽셀 이상 움직이면 드래그로 간주
  if (Math.abs(deltaX) > 5 && !isDragging) {
    isDragging = true;
    timelineCanvas.classList.add('dragging');
  }

  if (isDragging) {
    hasMoved = true;

    // 드래그 중에는 비디오 재생 멈춤
    if (!player.paused) {
      player.pause();
    }

    timelineOffset = dragStartOffset + deltaX;
  
    // 범위 제한 및 시간 계산 updateTimeline()과 동일한 방식
    var viewportWidth = timelineViewport.clientWidth;
    var zoomSeconds = zoomLevels[currentZoomIndex].seconds;
    
    // pixelsPersecond 다시 계산 (updateTimeline과 동일하게)
    // 전역 변수도 업데이트하여 다른 함수들과 일관성 유지
    pixelsPerSecond = viewportWidth / zoomSeconds;
    var currentPixelsPerSecond = pixelsPerSecond;
    var paddingSeconds = viewportWidth / currentPixelsPerSecond / 2;
    // var totalSeconds = DAY_SECONDS + paddingSeconds * 2;

    var maxSegmentTime = MAX_SEGMENT_TIME;
    var totalSeconds = maxSegmentTime + paddingSeconds * 2;
    
    var canvasWidth = totalSeconds * currentPixelsPerSecond;
    var minOffset = -(canvasWidth - viewportWidth);
    var maxOffset = 0;
    timelineOffset = Math.max(minOffset, Math.min(maxOffset, timelineOffset));
    
    // ruler와 canvas 함께 스크롤
    ruler.style.transform = 'translateX(' + timelineOffset + 'px)';
    timelineCanvas.style.transform = 'translateX(' + timelineOffset + 'px)';
    var isWideView = zoomSeconds >= 3600*12;
    
    // 중앙의 플레이헤드가 가리키는 시간 계산
    var centerOffset = -timelineOffset + viewportWidth / 2;
    var centerTimeSec = centerOffset / currentPixelsPerSecond - paddingSeconds;
    // currentTimeSec = Math.max(0, Math.min(DAY_SECONDS, centerTimeSec));
    currentTimeSec = Math.max(0, Math.min(maxSegmentTime, centerTimeSec));
    
    if(isWideView) {
      var playheadX = (currentTimeSec + paddingSeconds) * currentPixelsPerSecond;
      playheadOnTimeline.style.left = playheadX + 'px';
    }
	

    updateCurrentTime();
    updateInfo();

    // 실시간으로 비디오 seek
    // seekToTime(currentTimeSec);

    // 드래그 중에는 seek하지 않음 (빈 공간에서는 재생할 수 없으므로)
    // 현재 시간이 세그먼트 범위에 있을 때만 seek
    var foundSegment = false;
    for (var i = 0; i < segments.length; i++) {
      var seg = segments[i];
      if (isGapSegment(seg)) continue;
      if (currentTimeSec >= seg.startSec && currentTimeSec < seg.endSec) {
        foundSegment = true;
        break;
      }
    }
    
    if (foundSegment) {
      seekToTime(currentTimeSec, true);
    }
  }
}

  // 타임라인에 마우스 클릭 (드래그 이벤트)
  var timelineMouseDownHandler = function(e) {
    isMouseDown = true;
    dragStartX = e.clientX;
    dragStartOffset = timelineOffset;
    hasMoved = false;

    // 드래그 시작 시점의 세그먼트 인덱스 저장
    dragStartSegmentIndex = currentSegmentIndex;

    e.preventDefault();
  }

  var mouseUpHandler = function(e) {
    if (!isMouseDown) return;

    if (hasMoved && isDragging) {
        // 드래그 종료 시: 현재 시간이 세그먼트 범위에 있는지 확인
        var foundSegment = false;
        var targetSegmentIndex = -1;
      
        for (var i = 0; i < segments.length; i++) {
          var seg = segments[i];
          if (isGapSegment(seg)) continue;
          if (currentTimeSec >= seg.startSec && currentTimeSec < seg.endSec) {
              foundSegment = true;
              targetSegmentIndex = i;
              break;
          }
        }

        if (!foundSegment) {
          var adjustedTime = skipForwardOverGap(currentTimeSec);
          if (adjustedTime !== currentTimeSec) {
            currentTimeSec = adjustedTime;
            var resolved = findSegmentContainingTime(currentTimeSec);
            if (resolved) {
              foundSegment = true;
              targetSegmentIndex = resolved.index;
            }
          }
        }

        // 세그먼트 범위에 없으면 드래그 시작 세그먼트의 시작 또는 끝으로 이동
        if (!foundSegment && dragStartSegmentIndex >= 0 && dragStartSegmentIndex < segments.length) {
          var startSeg = segments[dragStartSegmentIndex];
          // 현재 시간이 세그먼트 시작보다 앞이면 시작으로, 끝보다 뒤면 끝으로
          if (currentTimeSec < startSeg.startSec) {
            currentTimeSec = startSeg.startSec + 0.5;
          } else if (currentTimeSec > startSeg.endSec) {
            currentTimeSec = startSeg.endSec - 0.5;
          }
          targetSegmentIndex = dragStartSegmentIndex;
        }

        // 세그먼트 재생
        if (targetSegmentIndex >= 0 && targetSegmentIndex < segments.length) {
          var seg = segments[targetSegmentIndex];
          var relativeTime = (seg.mediaOffset || 0) + (currentTimeSec - seg.startSec);
          if (relativeTime < 0) {
            relativeTime = 0;
          }
          currentSegmentIndex = targetSegmentIndex;
          
          if (currentSegmentUrl !== seg.url) {
            playHLS(seg.url, relativeTime, seg.isLive);
          } else {
            player.currentTime = relativeTime;
          }
          
          // 타임라인 업데이트
          updateTimeline();
        }

    } else {
        // 클릭 (드래그 없음): 클릭한 위치의 세그먼트 찾기
        var clickedElement = e.target;
    
        // 세그먼트를 클릭했는지 확인
        if (clickedElement.classList.contains('segment') && !clickedElement.classList.contains('segment-gap')) {
          var index = parseInt(clickedElement.getAttribute('data-index'));
          var seg = segments[index];

          // seekToTime 사용하여 일관된 로직으로 처리
          seekToTime(seg.startSec, false);
        }
    }

    isMouseDown = false;
    isDragging = false;
    hasMoved = false;
    dragStartSegmentIndex = -1; // 드래그 종료 시 초기화
    timelineCanvas.classList.remove('dragging');

    if (player.paused) {
      player.play();
    }
  }

var playerTimeupdateHandler = function() {
  if (currentSegmentIndex < 0) return;

  var seg = segments[currentSegmentIndex];
  if (!seg || isGapSegment(seg)) return;

  var mediaTime = player.currentTime;
  var baseOffset = seg.mediaOffset || 0;
  var relativeElapsed = mediaTime - baseOffset;

  // relativeElapsed가 세그먼트 범위를 벗어난 경우 처리
  if (relativeElapsed < -MEDIA_TIME_TOLERANCE || relativeElapsed > seg.duration + MEDIA_TIME_TOLERANCE) {
    // 같은 m3u8 URL 내에서 mediaTime을 가지는 세그먼트 찾기 (녹화 공백으로 인해 세그먼트 나눠질 수 있음)
    var resolved = findSegmentByMediaPosition(seg.url, mediaTime);
    if (resolved) {
      // 다른 세그먼트로 전환
      currentSegmentIndex = resolved.index;
      seg = resolved.segment;
      baseOffset = resolved.mediaOffset;
      relativeElapsed = mediaTime - baseOffset;
    } else if (relativeElapsed < -MEDIA_TIME_TOLERANCE) {
      // 음수 보정
      relativeElapsed = 0;
    } 
  }

  // if (relativeElapsed >= seg.duration - MEDIA_TIME_TOLERANCE) {
  //   var nextIdx = findNextRecordingSegment(currentSegmentIndex);
  //   if (nextIdx !== -1) {
  //     var nextSeg = segments[nextIdx];
  //     if (nextSeg.url === seg.url) {
  //       currentSegmentIndex = nextIdx;
  //       seg = nextSeg;
  //       baseOffset = nextSeg.mediaOffset || 0;
  //       relativeElapsed = mediaTime - baseOffset;
  //       if (relativeElapsed < 0) {
  //         relativeElapsed = 0;
  //       }
  //     } else {
  //       currentSegmentIndex = nextIdx;
  //       seg = nextSeg;
  //       currentTimeSec = seg.startSec;
  //       updateTimeline();
  //       playHLS(seg.url, nextSeg.mediaOffset || 0);
  //       return;
  //     }
  //   } else {
  //     relativeElapsed = Math.min(relativeElapsed, seg.duration);
  //   }
  // }

  // if (relativeElapsed < 0) {
  //   relativeElapsed = 0;
  // }

  currentTimeSec = seg.startSec + Math.min(relativeElapsed, seg.duration);
  
  // 드래그 중이 아닐 때만 타임라인 자동 스크롤
  if (!isDragging && !isMouseDown) {
      updateTimeline();
  } else {
      // 드래그 중에도 시간 표시는 업데이트
      updateCurrentTime();
      updateInfo();
      
      // 세그먼트 색상 업데이트
      var viewportWidth = timelineViewport.clientWidth;
      var paddingSeconds = viewportWidth / pixelsPerSecond / 2;
      updateSegmentPositions(paddingSeconds);
  }
  
}

// 마우스 휠로 줌
var timelineViewportWheelHandler = function(e) {
  e.preventDefault();

  if (e.deltaY < 0) {
    // 줌 인
    if (currentZoomIndex < zoomLevels.length - 1) {
      currentZoomIndex++;
    } else {
      return;
    }
  } else {
    // 줌 아웃
    if (currentZoomIndex > 0) {
      currentZoomIndex--;
    } else {
      return;
    }
  }
  
  updateTimeline();
}
// ---------------------------------------------------------------------------------------------------------------

/*
 * 루트 컨테이너에서 load 이벤트 발생 시 호출.
 * 앱이 최초 구성된후 최초 랜더링 직후에 발생하는 이벤트 입니다.
 */
function onBodyLoad(/* cpr.events.CEvent */ e){
	dataManager = getDataManager();
	
	var dateInput = app.lookup("dti_cctv");
	dateInput.dateValue = new Date();
	dateInput.maxDate = new Date();
	
	initElement(); // DOM 요소 할당
	var sms = app.lookup("sms_get_streamList");
	sms.action = "/v1/core/getStreamList";
	sms.addParameter("Category", "");
	sms.send();
}

function initElement() {
	var videoApp = app.lookup("entireVideo");
	player = document.getElementById('uuid-'+videoApp.uuid).querySelector('video');
	player.style.objectFit = "contain";
	
	timelineViewport = document.getElementById('timelineViewport');
	timelineCanvas = document.getElementById('timelineCanvas');
	ruler = document.getElementById('ruler');

	segmentsContainer = document.getElementById('segmentsContainer');
	
	playheadFixed = document.getElementById('playheadFixed');
	playheadOnTimeline = document.getElementById('playheadOnTimeline');

	currentTimeInfo = document.getElementById('currentTimeInfo');
	currentTimeInfo.textContent = '00:00:00';
	
	visibleRangeTitle = document.getElementById('visibleRangeTitle');
	visibleRangeTitle.textContent = dataManager.getString("Str_TimeRange") + '  I    '
	visibleRangeTime = document.getElementById("visibleRangeTime");
	visibleRangeTime.textContent = '00:00:00';
	
	
	status = document.getElementById('status');
	status.textContent = dataManager.getString("Str_NVRSegmentsReady");
}


/*
 * 그리드에서 selection-change 이벤트 발생 시 호출.
 * detail의 cell 클릭하여 설정된 selectionunit에 해당되는 단위가 선택될 때 발생하는 이벤트.
 */
function onGrd_StreamListSelectionChange(/* cpr.events.CSelectionEvent */ e){
	/** 
	 * @type cpr.controls.Grid
	 */
	var grd_StreamList = e.control;
	sendRecordingListReq();
}



/*
 * 서브미션에서 submit-done 이벤트 발생 시 호출.
 * 응답처리가 모두 종료되면 발생합니다.
 */
function onSms_get_streamListSubmitDone(/* cpr.events.CSubmissionEvent */ e){
	/** 
	 * @type cpr.protocols.Submission
	 */
	var sms_get_streamList = e.control;
	var result = app.lookup("Result").getValue("ResultCode");
	if(result == COMERROR_NONE) {

	} else {
		dialogAlert(app, dataManager.getString("Str_Fail") , dataManager.getString(getErrorString(result)));
	}
}

/*
 * 서브미션에서 submit-error 이벤트 발생 시 호출.
 * 통신 중 문제가 생기면 발생합니다.
 */
function onSms_getTerminalListSubmitError(/* cpr.events.CSubmissionEvent */ e){
//	var result = app.lookup("Result");
//	result.setValue("ResultCode",COMERROR_NET_ERROR);
}


/*
 * 서브미션에서 submit-timeout 이벤트 발생 시 호출.
 * 통신 중 Timeout이 발생했을 때 호출되는 이벤트입니다.
 */
function onSms_getTerminalListSubmitTimeout(/* cpr.events.CSubmissionEvent */ e){
	var result = app.lookup("Result");
	result.setValue("ResultCode",COMERROR_NET_TIMEOUT);
}

/*
 * 서브미션에서 submit-done 이벤트 발생 시 호출.
 * 응답처리가 모두 종료되면 발생합니다.
 */
function onSms_get_m3u8FilesSubmitDone(/* cpr.events.CSubmissionEvent */ e){
	/** 
	 * @type cpr.protocols.Submission
	 */
	var sms_get_m3u8Files = e.control;
	var result = app.lookup("Result").getValue("ResultCode");
	if(result == COMERROR_NONE) {
		var dsRecordingDatas = app.lookup("RecordingDatas");
		
		initTimeline();
		
	} else {
		dialogAlert(app, dataManager.getString("Str_Fail") , dataManager.getString(getErrorString(result)));
	}
}

/*
 * 데이트 인풋에서 value-change 이벤트 발생 시 호출.
 * Dateinput의 value를 변경하여 변경된 값이 저장된 후에 발생하는 이벤트.
 */
function onDti_cctvValueChange(/* cpr.events.CValueChangeEvent */ e){
	/** 
	 * @type cpr.controls.DateInput
	 */
	var dti_cctv = e.control;
	var grd_StreamList = app.lookup("grd_StreamList");
	if(grd_StreamList.getSelectedRow()) {
		var streamID = grd_StreamList.getSelectedRow().getValue("StreamID");
		var date = app.lookup("dti_cctv").value;
		
		var dmRecordingInfo = app.lookup("RecordingInfo");
		dmRecordingInfo.setValue("StreamID", streamID);
		dmRecordingInfo.setValue("Date", date);
		app.lookup("sms_get_m3u8Files").send();
	} 
}

// utils ------------------------------------------------------------
// 시간 문자열 → 초 변환
function timeToSeconds(timeStr) {
  var parts = timeStr.split(':');
  var h = parseInt(parts[0], 10);
  var m = parseInt(parts[1], 10);
  var s = parseInt(parts[2], 10);
  return h * 3600 + m * 60 + s;
}

// 초 → 시간 문자열 변환
function secondsToTime(seconds) {
  var h = Math.floor(seconds / 3600);
  var m = Math.floor((seconds % 3600) / 60);
  var s = Math.floor(seconds % 60);
  
  var hh = String(h).length < 2 ? '0' + h : h;
  var mm = String(m).length < 2 ? '0' + m : m;
  var ss = String(s).length < 2 ? '0' + s : s;
  
  return hh + ':' + mm + ':' + ss;
}

// m3u8 텍스트에서 총 재생 시간 계산
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

// TS 파일명에서 날짜와 시간 추출하여 초 단위로 변환
// 파일명 패턴: 최신프로_20251204_235833.ts 또는 최신프로_20251205_000005.ts
// baseDate: 기준 날짜 (YYYYMMDD 형식의 문자열, 첫 번째 파일의 날짜)
// 시간이 0시대(00:00:00 ~ 00:30:00)일 때만 날짜를 확인하여 24시 이후로 계산
function extractSecondsFromTsFilename(name, baseDate) {
  if (!name) return null;
  var trimmed = name.trim();
  // 날짜(YYYYMMDD)와 시간(HHMMSS) 모두 추출
  var match = trimmed.match(/_(\d{8})_(\d{6})\.ts$/i);
  var fileDate = match[1]; // YYYYMMDD
  var timeStr = match[2];  // HHMMSS
  var h = parseInt(timeStr.substring(0, 2), 10);
  var m = parseInt(timeStr.substring(2, 4), 10);
  var s = parseInt(timeStr.substring(4, 6), 10);
  if (isNaN(h) || isNaN(m) || isNaN(s)) return null;
  
  var baseSeconds = h * 3600 + m * 60 + s;

  // 시간이 0시대(00:00:00 ~ 00:30:00)이고 기준 날짜가 있고 날짜가 다르면 24시 이후로 계산
  // 그 외의 경우는 그대로 반환
  if (h === 0 && baseSeconds <= 1800 && baseDate && fileDate !== baseDate) {
    // 날짜가 바뀐 경우: 기준 날짜의 24시 이후로 계산
    // 필요한 경우에만 정규식으로 날짜 파싱하는건 미세 최적화라 수십만건이 아닌 이상 굳이 필요없음.
    return 86400 + baseSeconds; // 24시간(86400초) + 파일 시간
  }
  
  return baseSeconds;

}

// 1단계: m3u8 텍스트를 파싱하여 TS 파일 정보(entry) 리스트 생성
function parseM3u8ToEntries(content, defaultStartSec, urlAddress) {
  if (!content) return [];

  var lines = content.split(/\r?\n/);
  var pendingDuration = null;
  var cumulativeOffset = 0;
  var fallbackStartSec = typeof defaultStartSec === 'number' ? defaultStartSec : 0;
  var entries = [];
  var baseDate = null; // 첫 번째 파일의 날짜를 기준으로 사용

  for (var i = 0; i < lines.length; i++) {
    var line = lines[i].trim();
    if (!line) continue;

    // #EXTINF 라인: 다음 TS 파일의 재생 시간
    if (line.indexOf('#EXTINF:') === 0) {
      var value = line.split(':')[1];
      pendingDuration = parseFloat(value);
      continue;
    }

    // 주석 라인은 무시
    if (line.charAt(0) === '#') {
      continue;
    }

    // TS 파일명 라인: #EXTINF 다음에 와야 함
    if (pendingDuration === null) {
      continue;
    }

    // 첫 번째 파일의 날짜 추출 (기준 날짜 설정)
    if (baseDate === null) {
      var dateMatch = line.match(/_(\d{8})_/);
      if (dateMatch) {
        baseDate = dateMatch[1]; // YYYYMMDD 형식
      }
    }

    // TS 파일명에서 시작 시간 추출 (기준 날짜 전달)
    var startSec = extractSecondsFromTsFilename(line, baseDate);
    if (startSec === null) {
      startSec = fallbackStartSec;
    }

    // Entry 생성: 각 TS 파일의 정보
    var entry = {
      url: urlAddress,
      startSec: startSec,
      endSec: startSec + pendingDuration,
      duration: pendingDuration,
      mediaOffset: cumulativeOffset,  // m3u8 파일 내 누적 재생 시간
      mediaEndOffset: cumulativeOffset + pendingDuration
    };

    entries.push(entry);

    // 다음 entry를 위한 준비
    cumulativeOffset += pendingDuration;
    fallbackStartSec = startSec + pendingDuration;
    pendingDuration = null;
  }

  // 시간순 정렬 (TS 파일명의 시간 기준)
  entries.sort(function(a, b) {
    return a.startSec - b.startSec;
  });

  return entries;
}

// 2단계: Entry들을 연속성 체크하여 블록으로 병합
function mergeEntriesToBlocks(entries, urlAddress) {
  if (!entries || !entries.length) {
    return [];
  }

  var blocks = [];
  var currentBlock = null;
  var blockFirstEntry = null;  // 블록의 첫 번째 entry (가장 작은 mediaOffset)

  for (var j = 0; j < entries.length; j++) {
    var entryItem = entries[j];

    // 첫 번째 entry로 새 블록 시작
    if (!currentBlock) {
      blockFirstEntry = entryItem;
      currentBlock = {
        url: urlAddress,
        startSec: entryItem.startSec,
        endSec: entryItem.endSec,
        mediaOffset: entryItem.mediaOffset,
        mediaEndOffset: entryItem.mediaEndOffset
      };
      continue;
    }

    // 현재 블록과의 시간 차이 계산
    var diff = entryItem.startSec - currentBlock.endSec;

    // 공백이 크면 새 블록 시작
    if (diff > GAP_THRESHOLD_SECONDS) {
      blocks.push(currentBlock);
      blockFirstEntry = entryItem;
      currentBlock = {
        url: urlAddress,
        startSec: entryItem.startSec,
        endSec: entryItem.endSec,
        mediaOffset: entryItem.mediaOffset,
        mediaEndOffset: entryItem.mediaEndOffset
      };
    } else {
      // 연속된 경우: 현재 블록에 병합
      currentBlock.endSec = entryItem.endSec;
      currentBlock.mediaEndOffset = entryItem.mediaEndOffset;
    }
  }

  // 마지막 블록 추가
  if (currentBlock) {
    blocks.push(currentBlock);
  }

  return blocks;
}

// 3단계: 블록에 표시용 정보 추가 (startTime 문자열 등)
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
      startTime: secondsToTime(block.startSec),  // 표시용 시간 문자열
      mediaOffset: block.mediaOffset,
      mediaEndOffset: block.mediaEndOffset,
      isGap: false,
      isLive: typeof isLive === 'boolean' ? isLive : false // 실시간 녹화중 여부
    });
  }

  return formattedBlocks;
}

// 메인 함수: m3u8 텍스트 → 녹화 구간(연속 블록) 리스트 변환
function createRecordingBlocksFromContent(content, defaultStartSec, urlAddress) {
  // 1단계: m3u8 파싱하여 entry 리스트 생성
  var entries = parseM3u8ToEntries(content, defaultStartSec, urlAddress);
  if (!entries.length) {
    return [];
  }

  // 2단계: entry들을 연속성 체크하여 block으로 병합
  var blocks = mergeEntriesToBlocks(entries, urlAddress);

  // 3단계 : 실시간 녹화 여부 확인 (#EXT-X-ENDLIST 없으면 실시간 녹화로 판단)
  var isLive = !hasEndList(content);

  // 4단계: block에 표시용 정보 추가
  var formattedBlocks = formatBlocks(blocks, isLive);

  return formattedBlocks;
}

// m3u8 content에 #EXT-X-ENDLIST가 있는지 확인
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

    if (prevEnd !== null && block.startSec - prevEnd > GAP_THRESHOLD_SECONDS) {
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
  if (!segments || !segments.length) return null;
  for (var i = 0; i < segments.length; i++) {
    var seg = segments[i];
    if (isGapSegment(seg)) continue;
    if (timeSec >= seg.startSec && timeSec < seg.endSec) {
      return { index: i, segment: seg };
    }
  }
  return null;
}

function findNextRecordingSegment(fromIndex) {
  if (!segments || !segments.length) return -1;
  var start = typeof fromIndex === 'number' ? fromIndex + 1 : 0;
  for (var i = start; i < segments.length; i++) {
    if (!isGapSegment(segments[i])) {
      return i;
    }
  }
  return -1;
}

function skipForwardOverGap(timeSec) {
  if (!segments || !segments.length) return timeSec;
  for (var i = 0; i < segments.length; i++) {
    var seg = segments[i];
    if (!isGapSegment(seg)) continue;
    if (timeSec >= seg.startSec && timeSec < seg.endSec) {
      return seg.endSec;
    }
  }
  return timeSec;
}

function findSegmentByMediaPosition(url, mediaTime) {
  if (!segments || !segments.length) return null;
  if (!url && typeof mediaTime !== 'number') return null;

  for (var i = 0; i < segments.length; i++) {
    var seg = segments[i];
    if (isGapSegment(seg)) continue;
    if (seg.url !== url) continue;

    var startOffset = seg.mediaOffset || 0;
    var endOffset = typeof seg.mediaEndOffset === 'number' ? seg.mediaEndOffset : (startOffset + seg.duration);

    if (mediaTime >= startOffset - MEDIA_TIME_TOLERANCE && mediaTime < endOffset + MEDIA_TIME_TOLERANCE) {
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

// 세그먼트 초기화 (텍스트 버전 - fetch 없음)
function initSegments() { 
  var dsRecordingDatas = app.lookup("RecordingDatas");
  var count = dsRecordingDatas.getRowCount();
  var recordingBlocks = [];
  var streamID = app.lookup("RecordingInfo").getValue("StreamID");

  for (var i = 0; i < count; i++) {
    var row = dsRecordingDatas.getRow(i);
    var content = row.getValue("m3u8Content");
    var startTime = row.getValue("startTime");
    var fileName = row.getValue("fileName");

    if (!content) continue;

    var urlAddress = document.URL.toString() + '/proxy/core/m3u8?';
    urlAddress += "StreamID=" + streamID;
    urlAddress += "&ChannelID=0";
    urlAddress += "&Filename=" + fileName;

    var inferredStartSec = getFirstTsStartSec(content);
    var defaultStartSec = inferredStartSec !== null ? inferredStartSec : timeToSeconds(startTime);
    var blocks = createRecordingBlocksFromContent(content, defaultStartSec, urlAddress);
    recordingBlocks = recordingBlocks.concat(blocks);
  }

  segments = mergeRecordingBlocksWithGaps(recordingBlocks);

  var recordingCount = 0;
  for (var j = 0; j < segments.length; j++) {
    if (!isGapSegment(segments[j])) {
      recordingCount++;
    }
  }

  if(recordingCount == 0) {
  	status.classList.add('status--alert');
  } else {
  	status.classList.remove('status--alert');
  }
  status.textContent = recordingCount + dataManager.getString("Str_NVRSegmentsInfo");

  createSegments();

  var firstPlayableIndex = findNextRecordingSegment(-1);
  if (firstPlayableIndex >= 0) {
    currentSegmentIndex = firstPlayableIndex;
    currentTimeSec = segments[firstPlayableIndex].startSec;
    updateTimeline();
    var seg = segments[firstPlayableIndex];
    playHLS(seg.url, seg.mediaOffset || 0 , seg.isLive);
  }
}

// 세그먼트 DOM 요소 생성 (깜빡임 방지)
function createSegments() {
  segmentElements = [];

  for (var i = 0; i < segments.length; i++) {
    var seg = segments[i];
    var segDiv = document.createElement('div');
    if (isGapSegment(seg)) {
      segDiv.className = 'segment segment-gap';
      segDiv.title = dataManager ? dataManager.getString("Str_NVRNoRecord") || 'No Recording' : 'No Recording';
    } else {
      segDiv.className = 'segment';
      segDiv.title = seg.startTime + ' (' + Math.floor(seg.duration) + ' Sec)';
      segDiv.setAttribute('data-index', i);
    }
    
    segmentsContainer.appendChild(segDiv);
    segmentElements.push(segDiv);
  }
}


// 세그먼트 위치만 업데이트 (매번 실행)
function updateSegmentPositions(paddingSeconds) {
	// paddingSeconds가 제공되지 않으면 계산 (하위 호환성)
  if (typeof paddingSeconds === 'undefined') {
    var viewportWidth = timelineViewport.clientWidth;
    var currentPixelsPerSecond = pixelsPerSecond;
    paddingSeconds = viewportWidth / currentPixelsPerSecond / 2;
  }

  var currentPixelsPerSecond = pixelsPerSecond;
  
  for (var i = 0; i < segments.length; i++) {
    if (segmentElements[i]) {
      var seg = segments[i];
      // 눈금선과 동일한 계산 방식: (sec + paddingSeconds) * currentPixelsPerSecond
      var left = (seg.startSec + paddingSeconds) * currentPixelsPerSecond;
      var width = seg.duration * currentPixelsPerSecond;
      
      segmentElements[i].style.left = left + 'px';
      segmentElements[i].style.width = width + 'px';

      // playhead 위치 기준으로 이전/이후 
      segmentElements[i].classList.remove('segment-past', 'segment-future', 'segment-current');
      segmentElements[i].removeAttribute('data-playhead-ratio');

      // playhead 위치 확인
      if(seg.endSec <= currentTimeSec) {
        // playhead 이전
        segmentElements[i].classList.add('segment-past');
      } else if (seg.startSec > currentTimeSec) {
        // playhead 이후
        segmentElements[i].classList.add('segment-future');
      } else if (seg.startSec <= currentTimeSec && currentTimeSec < seg.endSec) {
        // playhead가 세그먼트 내부에 있으면 반으로 나눠서 색상 표시
        segmentElements[i].classList.add('segment-current');
        // playhead 위치를 세그먼트 내에서의 비율로 계산 (0~1)
        var playheadRatio = (currentTimeSec - seg.startSec) / seg.duration;
        // CSS 변수로 설정하여 gradient에서 사용
        segmentElements[i].style.setProperty('--playhead-ratio', (playheadRatio * 100) + '%');
      }
    }
  }
}

// 타임라인 렌더링
function updateTimeline() {
  var viewportWidth = timelineViewport.clientWidth;
  
  // 현재 줌 레벨에 맞춰 pixelsPerSecond 동적 계산
  var zoomSeconds = zoomLevels[currentZoomIndex].seconds;
  pixelsPerSecond = viewportWidth / zoomSeconds;
  
  // 세그먼트의 최대 시간 계산 (24시를 넘을 수 있음)
  var maxSegmentTime = MAX_SEGMENT_TIME;

  // 타임라인 캔버스 크기 설정 (24시간 + 양쪽 패딩)
  var paddingSeconds = viewportWidth / pixelsPerSecond / 2;
  var totalSeconds = maxSegmentTime + paddingSeconds * 2;
  var canvasWidth = totalSeconds * pixelsPerSecond;
  timelineCanvas.style.width = canvasWidth + 'px';
  
  // Ruler도 같은 크기로 설정
  ruler.style.width = canvasWidth + 'px';

  // Ruler가 viewport 바깥에 있으면 viewport의 왼쪽 끝과 정렬
  // ruler의 위치를 timelineViewport와 동일하게 설정
  try {
    var viewportRect = timelineViewport.getBoundingClientRect();
    var rulerParent = ruler.parentElement;
    
    if (rulerParent) {
      var parentRect = rulerParent.getBoundingClientRect();
      var rulerPosition = window.getComputedStyle(ruler).position;
      
      // ruler가 absolute 또는 relative positioning인 경우
      if (rulerPosition === 'absolute' || rulerPosition === 'relative') {
        // ruler의 left를 timelineViewport의 left와 동일하게 설정
        var leftOffset = viewportRect.left - parentRect.left;
        ruler.style.left = leftOffset + 'px';
      }
    }
  } catch (e) {
    // 에러 발생 시 무시 (CSS 계산 실패 등)
    console.log('Ruler positioning error:', e);
  }
  
  // 줌 레벨에 따라 플레이헤드 모드 결정
  var isWideView = zoomSeconds >= 3600 * 12; // 12시간 이상
  
  if (isWideView) {
    // 넓은 뷰: 플레이헤드를 타임라인 위에 표시
    playheadFixed.style.display = 'none';
    playheadOnTimeline.style.display = 'block';
    
    // 플레이헤드를 실제 시간 위치에 배치 (패딩 고려)
    var playheadX = (currentTimeSec + paddingSeconds) * pixelsPerSecond;
    playheadOnTimeline.style.left = playheadX + 'px';
    
    // 플레이헤드를 중앙에 배치
    var desiredOffset = -(playheadX - viewportWidth / 2);
    timelineOffset = desiredOffset;
  } else {
    // 좁은 뷰: 플레이헤드를 중앙 고정, 타임라인 스크롤
    playheadFixed.style.display = 'block';
    playheadOnTimeline.style.display = 'none';
    
    // 플레이헤드(중앙)가 currentTimeSec을 가리키도록 타임라인 오프셋 계산
    var centerOffset = (currentTimeSec + paddingSeconds) * pixelsPerSecond;
    timelineOffset = -(centerOffset - viewportWidth / 2);
  }
  
  // 범위 제한 (패딩 영역 포함)
  var minOffset = -(canvasWidth - viewportWidth);
  var maxOffset = 0;
  timelineOffset = Math.max(minOffset, Math.min(maxOffset, timelineOffset));
  
  // ruler와 canvas를 함께 스크롤
  ruler.style.transform = 'translateX(' + timelineOffset + 'px)';
  
  timelineCanvas.style.transform = 'translateX(' + timelineOffset + 'px)';
  
  // 시간 눈금 렌더링
  renderTimeScale(paddingSeconds, maxSegmentTime);
  
  // 세그먼트 위치 업데이트 (재생성 안 함)
  updateSegmentPositions(paddingSeconds);
  
  // 현재 시간 표시
  updateCurrentTime();
  
  // 정보 업데이트
  updateInfo();
}

// 시간 눈금 렌더링
function renderTimeScale(paddingSeconds, maxTime) {
  ruler.innerHTML = '';
  
  var zoomSeconds = zoomLevels[currentZoomIndex].seconds;
  
  // updateTimeline()에서 이미 pixelsPerSecond를 업데이트했으므로 전역 변수 사용
  // 현재 줌 레벨에 맞춰 pixelsPerSecond 계산 (전역변수와 독립적)
  var currentPixelsPerSecond = pixelsPerSecond;

  // paddingSeconds가 제공되지 않으면 계산 (하위 호환성)
  if (typeof paddingSeconds === 'undefined') {
    var viewportWidth = timelineViewport.clientWidth;
    paddingSeconds = viewportWidth / currentPixelsPerSecond / 2;
  }

  // maxTime이 제공되지 않으면 기본값 사용
  if (typeof maxTime === 'undefined') {
    maxTime = DAY_SECONDS;
  }
  
  var tickInterval = 3600; // 기본: 1시간
  var majorInterval = 3600;
  
  // 줌 레벨에 따라 눈금 간격 조정
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
    tickInterval = 300;     //1800
    majorInterval = 900;    //3600
  } else if (zoomSeconds >= 3600) {  // 줌레벨 1시간
    tickInterval = 60; //900
    majorInterval = 600; //3600
  } else if (zoomSeconds >= 900) {   // 줌레벨  15분
    tickInterval = 60;
    majorInterval = 300;
  } 
   
//  var paddingSeconds = viewportWidth / pixelsPerSecond / 2;
  
  for (var sec = 0; sec <= maxTime; sec += tickInterval) {
//    var x = (sec + paddingSeconds ) * pixelsPerSecond;
    var x = (sec + paddingSeconds ) * currentPixelsPerSecond;
    var isMajor = sec % majorInterval === 0;
    
    // 눈금선
    var tick = document.createElement('div');
    tick.className = isMajor ? 'time-tick major' : 'time-tick';
    tick.style.left = x + 'px';
    ruler.appendChild(tick);
    
    // 라벨 (major tick에만)
    if (isMajor) {
      var label = document.createElement('div');
      label.className = 'time-label';
      label.style.left = x + 'px';
      label.textContent = secondsToTime(sec);
      ruler.appendChild(label);
    }
  }
}

// 정보 업데이트
function updateInfo() {
	var viewportWidth = timelineViewport.clientWidth;

	// 현재 줌 레벨에 맞춰 pixelsPerSecond 계산
	var currentPixelsPerSecond = pixelsPerSecond;
	var paddingSeconds = viewportWidth / currentPixelsPerSecond / 2;
	
	var canvasCenterX = -timelineOffset + viewportWidth / 2;
	var viewCenterSec = canvasCenterX / currentPixelsPerSecond - paddingSeconds;
	var viewStartSec = viewCenterSec - (viewportWidth / 2 / currentPixelsPerSecond);
	var viewEndSec = viewCenterSec + (viewportWidth / 2 / currentPixelsPerSecond);
	
  visibleRangeTime.textContent = secondsToTime(Math.max(0, viewStartSec)) + 
                             ' ~ ' + secondsToTime(Math.min(MAX_SEGMENT_TIME, viewEndSec));

}

// 현재 시간 정보 업데이트
function updateCurrentTime() {
  // var timeStr = secondsToTime(Math.max(0, Math.min(DAY_SECONDS, currentTimeSec)));
  var timeStr = secondsToTime(Math.max(0, Math.min(MAX_SEGMENT_TIME, currentTimeSec)));
  // var zoomSeconds = zoomLevels[currentZoomIndex].seconds;
  // var isWideView = zoomSeconds >= 3600 * 12;
  
  currentTimeInfo.textContent = timeStr;
}



function initTimeline() {
	clearTimeline();
  
	initSegments();

	currentZoomIndex = 4; // 2시간으로 리셋

	registEventListeners();  
}

function registEventListeners() {
	timelineCanvas.removeEventListener('mousedown', timelineMouseDownHandler);
    timelineCanvas.addEventListener('mousedown', timelineMouseDownHandler);

	document.removeEventListener('mousemove', mouseMoveHandler);
 	document.addEventListener('mousemove', mouseMoveHandler);

  	document.removeEventListener('mouseup', mouseUpHandler);
  	document.addEventListener('mouseup', mouseUpHandler);

	player.removeEventListener('timeupdate', playerTimeupdateHandler);
  	player.addEventListener('timeupdate', playerTimeupdateHandler);

    timelineViewport.removeEventListener('wheel', timelineViewportWheelHandler);
    try {
      timelineViewport.addEventListener('wheel', timelineViewportWheelHandler, { passive: false });
    } catch(e) {
      // 구형 브라우저에서는 옵션 없이 등록
      timelineViewport.addEventListener('wheel', timelineViewportWheelHandler);
    }
}

// ------------------------------------------- Timeline Event ------------------------------------------

/**
 * 타임라인의 특정 시간으로 이동하고 해당 위치의 비디오를 재생합니다.
 * 
 * @param {number} targetSec - 이동할 타임라인 시간 (초 단위)
 * @param {boolean} skipTimelineUpdate - 타임라인 업데이트를 건너뛸지 여부
 */
function seekToTime(targetSec, skipTimelineUpdate) {
  if (!segments.length) return;

  // 1. Gap 건너뛰기
  // 타겟 시간이 녹화 공백 구간에 있으면 다음 녹화 세그먼트의 시작으로 이동
  var adjustedTarget = skipForwardOverGap(targetSec);

  // 2. 세그먼트 찾기
  // adjustedTarget이 포함된 녹화 세그먼트 확인
  var resolved = findSegmentContainingTime(adjustedTarget);

  // 3. resolved 못찾은 경우 : 2가지
  // - 타겟 시간이 모든 세그먼트보다 앞에 있음
  // - 타겟 시간이 모든 세그먼트보다 뒤에 있음
  if (!resolved) {
    // 3-1) 타겟 시간이 모든 세그먼트보다 뒤임 : 마지막 세그먼트의 끝으로 이동
    for (var i = segments.length - 1; i >= 0; i--) {
      var candidate = segments[i];
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

  // 3-2) 타겟 시간이 모든 세그먼트보다 앞임
  // 첫 번째 세그먼트의 시작으로 이동
  if (!resolved) {
    var firstIdx = findNextRecordingSegment(-1);
    if (firstIdx === -1) return;
    resolved = { index: firstIdx, segment: segments[firstIdx] };
    adjustedTarget = resolved.segment.startSec;
  }

  // 4. 상태 업데이트
  currentSegmentIndex = resolved.index;
  currentTimeSec = adjustedTarget;

  // 5. 타임라인 UI 업데이트
  if(!skipTimelineUpdate) {
    updateTimeline();
  } else {
    // 세그먼트 색상 업데이트
    var viewportWidth = timelineViewport.clientWidth;
    var paddingSeconds = viewportWidth / pixelsPerSecond / 2;
    updateSegmentPositions(paddingSeconds);
  }

  // 6. 미디어 재생 시간 계산 및 재생
  var seg = resolved.segment;

  // 절대 시간을 상대 시간으로 변환 (타임라인 시간 -> 미디어의 재생 시간)
  // mediaOffset : 세그먼트의 첫 번째 TS 파일이 m3u8에서 시작하는 위치
  var relativeTime = (seg.mediaOffset || 0) + (adjustedTarget - seg.startSec);
  if (relativeTime < 0) {
    relativeTime = 0;
  }

  
  if (currentSegmentUrl === seg.url && hls) {
    player.currentTime = relativeTime;
  } else {
    playHLS(seg.url, relativeTime, seg.isLive);
  }
}

// HLS 재생 (깜빡임 방지 로직 포함) - ES5 버전
function playHLS(url, seekTime, isLive) {
  if (typeof seekTime === 'undefined') seekTime = 0;
  
  if (currentSegmentUrl === url && hls) {
    try {
      if (typeof seekTime === 'number' && seekTime >= 0) {
        player.currentTime = seekTime;
      }
    } catch(e) {
      console.log('동일 URL seek 실패:', e);
    }
    if (player.paused) {
      player.play().catch(function(err) {
        console.log('자동 재생 실패:', err);
      });
    }
    return;
  }
  
  // 새로운 URL → HLS 재생성
  currentSegmentUrl = url;
  
  if (hls) {
    hls.destroy();
  }
  
  if (Hls.isSupported()) {
    hls = new Hls({
      enableWorker: true,
      lowLatencyMode: false
    });
    
    hls.loadSource(url);
    hls.attachMedia(player);
    
    hls.on(Hls.Events.MANIFEST_PARSED, function() {
      if (seekTime > 0) {
        player.currentTime = seekTime;
      } else if(!isLive){
        player.currentTime = 1;
      } 
      player.play().catch(function(err) {
        console.log('자동 재생 실패:', err);
      });
    });
    
    hls.on(Hls.Events.ERROR, function(event, data) {
      console.error('HLS Error:', data);
      if (data.fatal) {
        status.textContent = dataManager.getString("Str_Error") + data.type;
      }
    });
  } else if (player.canPlayType('application/vnd.apple.mpegurl')) {
    player.src = url;
    player.currentTime = 0.2;
    if (seekTime > 0) {
      player.currentTime = seekTime;
    }
    player.play();
  } else {
    alert('HLS 재생을 지원하지 않는 브라우저입니다.');
  }
}

// 기존 세그먼트 및 상태 정리
function clearTimeline() {
  // HLS 인스턴스 정리
  if (hls) {
    hls.destroy();
    hls = null;
  }
  
  // 비디오 정지 및 초기화
  if (player) {
    player.pause();
    player.removeAttribute('src');
    player.load();
  }
  
  // 세그먼트 데이터 초기화
  segments = [];
  segmentElements = [];
  currentSegmentIndex = -1;
  currentSegmentUrl = null;
  currentTimeSec = 0;
  
  // DOM 초기화
  if (segmentsContainer) {
    segmentsContainer.innerHTML = '';
  }
  if (ruler) {
    ruler.innerHTML = '';
  }
  
  // 타임라인 상태 초기화
  timelineOffset = 0;
  isDragging = false;
  isMouseDown = false;
  hasMoved = false;
  
  // 표시 시간 초기화
  currentTimeInfo.textContent = '00:00:00';
  visibleRangeTime.textContent = '00:00:00';
  
  // 플레이헤드 숨김
  if (playheadFixed) {
    playheadFixed.style.display = 'none';
  }
  if (playheadOnTimeline) {
    playheadOnTimeline.style.display = 'none';
  }
}


/*
 * 버튼에서 click 이벤트 발생 시 호출.
 * 사용자가 컨트롤을 클릭할 때 발생하는 이벤트.
 */
function onButtonClick(/* cpr.events.CMouseEvent */ e){
	/** 
	 * @type cpr.controls.Button
	 */
	var button = e.control; 
	sendRecordingListReq();
}

function sendRecordingListReq() {
	var grd_StreamList = app.lookup("grd_StreamList");
	var streamID = grd_StreamList.getSelectedRow().getValue("StreamID");
	var date = app.lookup("dti_cctv").value;
	
	var dmRecordingInfo = app.lookup("RecordingInfo");
	dmRecordingInfo.setValue("StreamID", streamID);
	dmRecordingInfo.setValue("Date", date);
	app.lookup("sms_get_m3u8Files").send();
}


/*
 * 이미지에서 click 이벤트 발생 시 호출.
 * 사용자가 컨트롤을 클릭할 때 발생하는 이벤트.
 */
function onTMMGR_imgHelpPageClick(/* cpr.events.CMouseEvent */ e){
	var menu_id = app.getHostProperty("initValue")["programID"];
	var selectionEvent = new cpr.events.CUIEvent("execute-menu", {content: {"Target":DLG_HELP,"ID": menu_id}});
	app.getHostAppInstance().dispatchEvent(selectionEvent);
}


/*
 * 서브미션에서 error-status 이벤트 발생 시 호출.
 * 서버로 부터 에러로 분류되는 HTTP상태 코드를 전송받았을 때 발생합니다.
 */
function onSms_get_streamListErrorStatus(/* cpr.events.CSubmissionEvent */ e){
	/** 
	 * @type cpr.protocols.Submission
	 */
	var sms_get_streamList = e.control;
	e.preventDefault();
}
