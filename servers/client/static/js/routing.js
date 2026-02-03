// 탭 전환 및 라우팅 관리

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
    
    // Monitoring 탭을 떠날 때 정리 (iframe 제거)
    if (currentTabName === 'monitoring' && tabName !== 'monitoring') {
        cleanupMonitoringTab();
        // body 클래스도 제거
        document.body.classList.remove('monitoring-active');
    }
    
    // Settings 탭을 떠날 때 정리 (iframe 제거)
    if (currentTabName === 'settings' && tabName !== 'settings') {
        cleanupSettingsTab();
        // body 클래스도 제거
        document.body.classList.remove('monitoring-active');
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
        // body에 클래스 추가하여 스크롤 제거
        document.body.classList.add('monitoring-active');
    }
    
    // Settings 탭인 경우 초기화
    if (tabName === 'settings') {
        initializeSettingsTab();
        // body에 클래스 추가하여 스크롤 제거
        document.body.classList.add('monitoring-active');
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
    const dataManager = DataManager.getInstance();
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
    const dataManager = DataManager.getInstance();
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

