// 설정 관리

// 설정 로드
function loadSettings() {
    const savedSettings = localStorage.getItem('clientSettings');
    if (savedSettings) {
        const settings = JSON.parse(savedSettings);
        const serverUrlElement = document.getElementById('serverUrl');
        const apiKeyElement = document.getElementById('apiKey');
        
        if (serverUrlElement) {
            serverUrlElement.value = settings.serverUrl || '';
        }
        if (apiKeyElement) {
            apiKeyElement.value = settings.apiKey || '';
        }
    }
}

