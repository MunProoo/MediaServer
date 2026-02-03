// 메인 초기화 및 통합

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
