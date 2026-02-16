// ============================================
// CONFIGURATION
// ============================================

const CONFIG = {
    // SSE
    SSE_MAX_RECONNECTS: 5,
    SSE_RECONNECT_DELAYS: [1000, 2000, 5000, 10000, 30000],

    // Toast
    TOAST_DUPLICATE_CHECK_TIME: 3000,
    TOAST_AUTO_HIDE_DELAY: 3000,

    // Rate limiting
    RATE_LIMIT_DELAY: 500,
    RATE_LIMIT_CLEANUP_MS: 10 * 60 * 1000, // 10 minutes

    // Pagination
    PAGE_SIZE_MOBILE: 5,
    PAGE_SIZE_DESKTOP: 9,
    MIN_ITEMS_PAGINATION_MOBILE: 5,
    MIN_ITEMS_PAGINATION_DESKTOP: 9,
    MIN_SERVERS_PAGINATION_MOBILE: 5,
    MIN_SERVERS_PAGINATION_DESKTOP: 9,

    // Polling
    POLLING_INTERVAL: 5000,

    // Cache limits to avoid unbounded growth
    MAX_SERVICES_CACHE: 2000,
    MAX_SERVERS_CACHE: 1000
};

// ============================================
// GLOBAL STATE
// ============================================

let currentUser = localStorage.getItem('swsm_user');
let currentServerId = localStorage.getItem('swsm_current_server_id');
let currentServerData = null;

let isDarkMode = localStorage.getItem('swsm_dark_mode') === 'true';

// Pagination localStorage keys
const LS_SERVERS_PAGE_KEY  = 'swsm_servers_page';
const LS_SERVICES_PAGE_KEY = 'swsm_services_page';

// Pagination state for servers
let allServers = [];
let serversCurrentPage = parseInt(localStorage.getItem(LS_SERVERS_PAGE_KEY) || '1', 10);
let serversTotalPages = 1;

// Pagination state for services
let allServices = [];
let currentPage = parseInt(localStorage.getItem(LS_SERVICES_PAGE_KEY) || '1', 10);
let totalPages = 1;

// SSE EventSource
let serviceEventsSource = null;
let servicePollingInterval = null;
let sseReconnectAttempts = 0;
let sseConnectionStatus = 'closed'; // 'closed', 'connecting', 'open'

let serverEventsSource = null;
let serverPollingInterval = null;
let serverSseReconnectTimerId = null;
let serverSseReconnectAttempts = 0;

// Rate limiting map (actionName -> timestamp)
const REQUEST_RATE_LIMIT = new Map();

// Page size cache
let cachedPageSize = null;
let lastWindowWidth = window.innerWidth;
let resizeDebounceTimer = null;

// Toast history for deduplication
let toastHistory = [];

// Session expired flag
window._sessionExpiredNotified = false;

let lastMobileView = null;

// helper timestamps
let lastServicesUpdateAt = 0; // timestamp (ms) последнего успешного обновления статусов
let sseReconnectTimerId = null;

// ============================================
// App timers registry (to prevent leaks)
// ============================================

const AppTimers = {
    intervals: new Set(),
    timeouts: new Set(),

    addInterval(id) { if (id != null) this.intervals.add(id); },
    addTimeout(id) { if (id != null) this.timeouts.add(id); },

    clearAll() {
        for (const id of this.intervals) {
            try { clearInterval(id); } catch (e) {}
        }
        this.intervals.clear();

        for (const id of this.timeouts) {
            try { clearTimeout(id); } catch (e) {}
        }
        this.timeouts.clear();
    }
};

// ============================================
// DOM ELEMENTS (grab once)
// ============================================

const loginPage = document.getElementById('loginPage');
const mainApp = document.getElementById('mainApp');
const loadingSpinner = document.querySelector('.loading-spinner');
const currentUserSpan = document.getElementById('currentUser');
const serversListView = document.getElementById('serversListView');
const serverDetailView = document.getElementById('serverDetailView');
const serversList = document.getElementById('serversList');
const servicesList = document.getElementById('servicesList');

// ============================================
// INITIALIZATION
// ============================================

document.addEventListener('DOMContentLoaded', function() {
    initTheme();

    if (currentUser) {
        showMainApp();

        // Всегда подписываемся на события серверов
        subscribeServerEvents();

        if (currentServerId) {
            currentServerId = parseInt(currentServerId);
            showServerDetail(currentServerId);
        } else {
            loadServersList();
        }
    } else {
        showLoginPage();
    }

    setupEventListeners();
});

// ============================================
// UTILITY FUNCTIONS
// ============================================

function debounce(func, delay = 500) {
    let timeoutId;
    return function(...args) {
        if (timeoutId) clearTimeout(timeoutId);
        timeoutId = setTimeout(() => {
            func.apply(this, args);
            timeoutId = null;
        }, delay);
    };
}

function cleanupOldRateLimits() {
    const now = Date.now();
    for (const [key, ts] of REQUEST_RATE_LIMIT.entries()) {
        if (now - ts > CONFIG.RATE_LIMIT_CLEANUP_MS) {
            REQUEST_RATE_LIMIT.delete(key);
        }
    }
}

function canPerformAction(actionName) {
    cleanupOldRateLimits();

    const now = Date.now();
    const lastTime = REQUEST_RATE_LIMIT.get(actionName) || 0;

    if (now - lastTime < CONFIG.RATE_LIMIT_DELAY) {
        // rate limited
        return false;
    }

    REQUEST_RATE_LIMIT.set(actionName, now);
    return true;
}

function isMobileDevice() {
    // Проверяем и User Agent, и ширину окна для более точного определения
    const isMobileByUA = /iPhone|iPad|iPod|Android/i.test(navigator.userAgent);
    const isMobileByWidth = window.innerWidth < 768;

    return isMobileByUA || isMobileByWidth;
}

function getPageSize() {
    if (cachedPageSize === null || window.innerWidth !== lastWindowWidth) {
        lastWindowWidth = window.innerWidth;
        cachedPageSize = window.innerWidth < 768 ? CONFIG.PAGE_SIZE_MOBILE : CONFIG.PAGE_SIZE_DESKTOP;
    }
    return cachedPageSize;
}

// 111111111111111111
function switchConnectionsMode() {
    if (!currentUser) return;

    // Определяем текущий режим
    const isMobileView = isMobileDevice();

    // Если режим не изменился - ничего не делаем
    if (lastMobileView === isMobileView) {
        return;
    }

    lastMobileView = isMobileView;

    // Закрываем все текущие соединения для служб
    if (serviceEventsSource) {
        try { serviceEventsSource.close(); } catch (e) { /* noop */ }
        serviceEventsSource = null;
    }
    stopServicePolling();

    // Закрываем все текущие соединения для серверов
    if (serverEventsSource) {
        try { serverEventsSource.close(); } catch (e) { /* noop */ }
        serverEventsSource = null;
    }
    stopServerPolling();

    // Переподключаемся в зависимости от текущего режима
    subscribeServerEvents();
    if (currentServerId) {
        subscribeServiceEvents(currentServerId);
    }
}
// function switchConnectionsMode() {
//     if (!currentUser) return;
//
//     // Фиксируем текущий режим (мобильный или десктопный)
//     const isMobileView = window.innerWidth < 768;
//     const wasMobileView = lastWindowWidth < 768;
//
//     // Переключаем только если режим изменился
//     if (isMobileView !== wasMobileView) {
//         console.log('Режим изменился с', wasMobileView ? 'мобильного' : 'десктопного', 'на', isMobileView ? 'мобильный' : 'десктопный');
//
//         // Закрываем все текущие соединения для служб
//         if (serviceEventsSource) {
//             try { serviceEventsSource.close(); } catch (e) { /* noop */ }
//             serviceEventsSource = null;
//         }
//         stopServicePolling();
//
//         // Закрываем все текущие соединения для серверов
//         if (serverEventsSource) {
//             try { serverEventsSource.close(); } catch (e) { /* noop */ }
//             serverEventsSource = null;
//         }
//         stopServerPolling();
//
//         // Перезаписываем lastWindowWidth после изменения
//         lastWindowWidth = window.innerWidth;
//
//         // Переподключаемся в зависимости от текущего режима
//         subscribeServerEvents();
//         if (currentServerId) {
//             subscribeServiceEvents(currentServerId);
//         }
//     }
// }

function getMinItemsForPagination(type) {
    const isMobile = window.innerWidth < 768;
    if (type === 'services') {
        return isMobile ? CONFIG.MIN_ITEMS_PAGINATION_MOBILE : CONFIG.MIN_ITEMS_PAGINATION_DESKTOP;
    } else if (type === 'servers') {
        return isMobile ? CONFIG.MIN_SERVERS_PAGINATION_MOBILE : CONFIG.MIN_SERVERS_PAGINATION_DESKTOP;
    }
    return 0;
}

function getServerStatusClass(status) {
    const normalized = (status || '').toUpperCase();
    switch (normalized) {
        case 'OK': return 'server-status-ok';
        case 'DEGRADED': return 'server-status-degraded';
        case 'DOWN':
        case 'UNREACHABLE': return 'server-status-down';
        default: return 'server-status-pending';
    }
}

// named resize handler so we can reason about it
// 2222222222222222222222

function onWindowResize() {
    cachedPageSize = null;

    // Добавляем дебаунсинг для переключения режима
    if (resizeDebounceTimer) {
        clearTimeout(resizeDebounceTimer);
        AppTimers.timeouts.delete(resizeDebounceTimer);
    }

    resizeDebounceTimer = setTimeout(() => {
        AppTimers.timeouts.delete(resizeDebounceTimer);
        resizeDebounceTimer = null;
        switchConnectionsMode();
    }, 300);

    AppTimers.addTimeout(resizeDebounceTimer);
}

// function onWindowResize() {
//     cachedPageSize = null;
//
//     // Добавляем дебаунсинг для переключения режима
//     if (resizeDebounceTimer) {
//         clearTimeout(resizeDebounceTimer);
//     }
//     resizeDebounceTimer = setTimeout(switchConnectionsMode, 300);
// }

// function onWindowResize() {
//     cachedPageSize = null;
// }

window.addEventListener('resize', onWindowResize);

// ============================================
// TOAST NOTIFICATIONS (with deduplication & proper cleanup)
// ============================================

function showToast(title, message, type = 'success') {
    const toastId = `${title}|${message}|${type}`;
    const now = Date.now();

    // Проверка недавних дубликатов
    const recentSimilar = toastHistory.find(t =>
        t.id === toastId &&
        (now - t.time) < CONFIG.TOAST_DUPLICATE_CHECK_TIME
    );

    if (recentSimilar) {
        console.warn('[Toast] Дублирование предотвращено:', toastId);
        return;
    }

    toastHistory.push({ id: toastId, time: now });

    // Ограничение размера массива (максимум 50, оставляем 30)
    if (toastHistory.length > 50) {
        toastHistory = toastHistory.slice(-30);
    }

    // Фильтруем старые записи (старше 10 секунд)
    toastHistory = toastHistory.filter(t => (now - t.time) < 10000);

    const toastContainer = document.querySelector('.toast-container');
    const toastTemplate = document.getElementById('toastTemplate');
    const toast = toastTemplate.cloneNode(true);

    toast.id = 'toast-' + Date.now() + Math.random();
    toast.querySelector('.toast-title').textContent = title;
    toast.querySelector('.toast-message').textContent = message;

    if (type === 'error') {
        toast.classList.add('text-bg-danger');
    } else if (type === 'warning') {
        toast.classList.add('text-bg-warning');
    } else {
        toast.classList.add('text-bg-success');
    }

    toastContainer.appendChild(toast);

    const bsToast = new bootstrap.Toast(toast, {
        autohide: true,
        delay: CONFIG.TOAST_AUTO_HIDE_DELAY
    });
    bsToast.show();

    // Принудительное закрытие через 3 секунды
    setTimeout(() => {
        bsToast.hide();
    }, 3000);

    // Закрытие по клику на тост
    toast.addEventListener('click', (e) => {
        // Не закрываем если тапнули на крестик
        if (e.target.classList.contains('btn-close')) {
            return;
        }
        bsToast.hide();  // Закрываем при любом другом тапе
    });

    toast.addEventListener('hidden.bs.toast', () => {
        toast.remove();
    });
}

// ============================================
// THEME MANAGEMENT
// ============================================

function initTheme() {
    applyTheme(isDarkMode);
}

function applyTheme(darkMode) {
    isDarkMode = darkMode;
    localStorage.setItem('swsm_dark_mode', isDarkMode);

    if (isDarkMode) {
        document.documentElement.setAttribute('data-bs-theme', 'dark');
        document.body.classList.add('dark-mode');
    } else {
        document.documentElement.removeAttribute('data-bs-theme');
        document.body.classList.remove('dark-mode');
    }

    updateThemeButton();
}

function toggleTheme() {
    applyTheme(!isDarkMode);
}

function updateThemeButton() {
    const themeBtn = document.getElementById('themeToggleBtn');
    if (themeBtn) {
        if (isDarkMode) {
            themeBtn.innerHTML = '☀️';
            themeBtn.title = 'Переключиться на светлый режим';
            themeBtn.className = 'btn me-2';
        } else {
            themeBtn.innerHTML = '🌚';
            themeBtn.title = 'Переключиться на тёмный режим';
            themeBtn.className = 'btn me-2';
        }
    }
}

// ============================================
// API FUNCTIONS
// ============================================

class SessionExpiredError extends Error {
    constructor(message) {
        super(message);
        this.name = 'SessionExpiredError';
    }
}

async function apiRequest(endpoint, options = {}) {
    const url = `${API_BASE}${endpoint}`;

    const config = {
        method: options.method || 'GET',
        headers: {
            'Content-Type': 'application/json',
            ...options.headers
        },
        credentials: 'include',
        cache: options.cache || 'default',
        ...options
    };

    try {
        const response = await fetch(url, config);

        if (response.status === 401) {

            // Для endpoint логина НЕ показываем "сессия истекла"
            const isLoginEndpoint = endpoint.includes('/user/login');

            if (!isLoginEndpoint) {
                if (!window._sessionExpiredNotified) {
                    window._sessionExpiredNotified = true;

                    // не удаляем слушатели форм, лишь очищаем состояние и таймеры
                    cleanupOnLogout();

                    localStorage.removeItem('swsm_user');
                    localStorage.removeItem('swsm_current_server_id');
                    currentUser = null;
                    currentServerId = null;
                    currentServerData = null;
                    allServices = [];
                    currentPage = 1;
                    allServers = [];
                    serversCurrentPage = 1;

                    if (serviceEventsSource) {
                        try {
                            serviceEventsSource.close();
                        } catch (e) {
                        }
                        serviceEventsSource = null;
                    }

                    stopServicePolling();

                    showToast('Сессия истекла', 'Ваша сессия истекла. Пожалуйста, авторизуйтесь снова.', 'warning');
                    showLoginPage();
                }
                throw new SessionExpiredError('Session expired');
            }
        }

        let data;
        const contentType = response.headers.get('Content-Type');
        if (contentType && contentType.includes('application/json')) {
            data = await response.json();
        } else {
            const text = await response.text();
            try {
                data = JSON.parse(text);
            } catch {
                data = { message: text };
            }
        }

        if (!response.ok) {
            throw new Error(data.message || data.error || `Ошибка HTTP! статус: ${response.status}`);
        }

        window._sessionExpiredNotified = false;
        return data;

    } catch (error) {
        if (error instanceof SessionExpiredError) {
            throw error;
        }

        if (error instanceof TypeError && error.message.includes('fetch')) {
            showToast('Ошибка', 'Не удается подключиться к серверу', 'error');
        }
        throw error;
    }
}

// ============================================
// SSE FUNCTIONS (with improvements & timers registration)
// ============================================

function subscribeServiceEvents(serverId) {
    // Использование полинга для мобильных устройств
    if (isMobileDevice()) {
        startServicePolling(serverId);
        return;
    }

    // Если уже есть открытое соединение - не создаём новое!
    if (serviceEventsSource && serviceEventsSource.readyState === EventSource.OPEN) {
        sseConnectionStatus = 'open';
        return;
    }

    // Закрытие старого соединения если оно есть и повреждено
    if (serviceEventsSource) {
        try { serviceEventsSource.close(); } catch (e) { /* noop */ }
        serviceEventsSource = null;
    }

    // Очистим таймер переподключения, если есть
    if (sseReconnectTimerId) {
        try { clearTimeout(sseReconnectTimerId); } catch (e) {}
        AppTimers.timeouts.delete(sseReconnectTimerId);
        sseReconnectTimerId = null;
    }

    const url = `${API_BASE}/user/broadcasting?stream=services`;

    try {
        serviceEventsSource = new EventSource(url, { withCredentials: true });
    } catch (e) {
        console.error('Не удалось создать EventSource:', e);
        startServicePolling(serverId);
        return;
    }

    sseConnectionStatus = 'connecting';

    // Обработчик открытия соединения
    serviceEventsSource.onopen = function() {
        sseConnectionStatus = 'open';
        sseReconnectAttempts = 0;
        if (sseReconnectTimerId) {
            try { clearTimeout(sseReconnectTimerId); } catch (e) {}
            AppTimers.timeouts.delete(sseReconnectTimerId);
            sseReconnectTimerId = null;
        }
    };

    serviceEventsSource.onmessage = function(event) {
        try {
            const data = JSON.parse(event.data);
            const filtered = data.filter(s => s.server_id === serverId);
            if (filtered.length > 0) {
                updateServicesStatus(filtered);
            }
            sseReconnectAttempts = 0;
        } catch (err) {
            console.error('Ошибка разбора данных SSE:', err);
        }
    };

    serviceEventsSource.onerror = function(err) {
        console.error('Ошибка SSE:', err);
        sseConnectionStatus = 'closed';

        if (sseReconnectAttempts >= CONFIG.SSE_MAX_RECONNECTS) {
            startServicePolling(serverId);
            return;
        }

        const delay = CONFIG.SSE_RECONNECT_DELAYS[sseReconnectAttempts] || 30000;
        sseReconnectAttempts++;

        if (sseReconnectTimerId) {
            try { clearTimeout(sseReconnectTimerId); } catch (e) {}
            AppTimers.timeouts.delete(sseReconnectTimerId);
            sseReconnectTimerId = null;
        }

        sseReconnectTimerId = setTimeout(() => {
            const timerId = sseReconnectTimerId;
            AppTimers.timeouts.delete(timerId);
            sseReconnectTimerId = null;
            if (currentServerId) {
                subscribeServiceEvents(currentServerId);
            }
        }, delay);
        AppTimers.addTimeout(sseReconnectTimerId);
    };
}

function subscribeServerEvents() {
    if (isMobileDevice()) {
        startServerPolling();
        return;
    }

    if (serverEventsSource) {
        try {
            serverEventsSource.close();
            serverEventsSource = null;
        } catch (e) {}
    }

    if (serverSseReconnectTimerId) {
        try { clearTimeout(serverSseReconnectTimerId); } catch (e) {}
        AppTimers.timeouts.delete(serverSseReconnectTimerId);
        serverSseReconnectTimerId = null;
    }

    const url = `${API_BASE}/user/broadcasting?stream=servers`;

    try {
        serverEventsSource = new EventSource(url, { withCredentials: true });
    } catch (e) {
        console.error('Не удалось создать EventSource для серверов:', e);
        // Если SSE не работает, запускаем поллинг как fallback
        startServerPolling();
        return;
    }

    serverSseReconnectAttempts = 0;

    serverEventsSource.onopen = function () {
        serverSseReconnectAttempts = 0;
        if (serverSseReconnectTimerId) {
            try { clearTimeout(serverSseReconnectTimerId); } catch (e) {}
            AppTimers.timeouts.delete(serverSseReconnectTimerId);
            serverSseReconnectTimerId = null;
        }
    };

    serverEventsSource.onmessage = function (event) {
        try {
            const data = JSON.parse(event.data);

            // Если мы в деталях сервера - фильтруем только текущий сервер
            if (!serversListView.classList.contains('hidden') && serverDetailView.classList.contains('hidden')) {
                // В списке серверов - обновляем все
                updateServersStatus(data);
            } else {
                // В деталях сервера - фильтруем только текущий
                if (currentServerId) {
                    const filtered = data.filter(s => s.server_id === currentServerId);
                    if (filtered.length > 0) {
                        updateServersStatus(filtered);
                    }
                }
            }
            serverSseReconnectAttempts = 0;
        } catch (err) {
            console.error('Ошибка разбора данных SSE (servers):', err);
        }
    };

    serverEventsSource.onerror = function (err) {
        console.error('Ошибка SSE (servers):', err);
        if (serverSseReconnectAttempts >= CONFIG.SSE_MAX_RECONNECTS) {
            try { serverEventsSource.close(); } catch (e) {}
            serverEventsSource = null;
            // При неудаче переключаемся на поллинг
            startServerPolling();

            // Очищаем таймер переподключения
            if (serverSseReconnectTimerId) {
                try { clearTimeout(serverSseReconnectTimerId); } catch (e) {}
                AppTimers.timeouts.delete(serverSseReconnectTimerId);
                serverSseReconnectTimerId = null;
            }
            return;
        }

        const delay = CONFIG.SSE_RECONNECT_DELAYS[serverSseReconnectAttempts] || 30000;
        serverSseReconnectAttempts++;

        if (serverSseReconnectTimerId) {
            try { clearTimeout(serverSseReconnectTimerId); } catch (e) {}
            AppTimers.timeouts.delete(serverSseReconnectTimerId);
            serverSseReconnectTimerId = null;
        }

        serverSseReconnectTimerId = setTimeout(() => {
            // Сохраняем ссылку на текущий ID перед очисткой
            const timerId = serverSseReconnectTimerId;
            AppTimers.timeouts.delete(timerId);
            serverSseReconnectTimerId = null;
            subscribeServerEvents();
        }, delay);
        AppTimers.addTimeout(serverSseReconnectTimerId);
    };
}

function startServicePolling(serverId) {
    stopServicePolling();

    let consecutiveErrors = 0;
    const MAX_CONSECUTIVE_ERRORS = 3;

    // Функция для выполнения запроса
    const poll = async () => {
        if (document.hidden) return;
        if (currentServerId !== serverId) {
            stopServicePolling();
            return;
        }

        try {
            const response = await apiRequest(`/user/servers/${serverId}/services`);
            if (Array.isArray(response)) {
                updateServicesStatus(response);
                consecutiveErrors = 0;
            }
        } catch (error) {
            consecutiveErrors++;
            console.error(`Ошибка полинга (${consecutiveErrors}/${MAX_CONSECUTIVE_ERRORS}):`, error);
            if (consecutiveErrors >= MAX_CONSECUTIVE_ERRORS) {
                stopServicePolling();
            }
        }
    };

    // Немедленный запрос при запуске
    poll();

    const id = setInterval(poll, CONFIG.POLLING_INTERVAL);
    AppTimers.addInterval(id);
    servicePollingInterval = id;
}

function stopServicePolling() {
    if (servicePollingInterval) {
        try { clearInterval(servicePollingInterval); } catch (e) {}
        AppTimers.intervals.delete(servicePollingInterval);
        servicePollingInterval = null;
    }
}

function updateServicesStatus(statuses) {
    if (!Array.isArray(statuses) || statuses.length === 0) {
        return;
    }

    // Исправлено: создаем карту по ID службы, а не сервера
    const statusMap = new Map();
    statuses.forEach(s => {
        // Используем id службы
        const serviceId = s.id;
        if (serviceId) {
            statusMap.set(Number(serviceId), s);
        }
    });

    // Обновление в памяти
    allServices.forEach(service => {
        const updatedStatus = statusMap.get(service.id);
        if (updatedStatus) {
            service.status = updatedStatus.status;
            service.updated_at = updatedStatus.updated_at ||
                updatedStatus.updatedat ||
                updatedStatus.updatedAt ||
                service.updated_at;
        }
    });

    // Обновление в DOM
    document.querySelectorAll('.service-card').forEach(card => {
        const serviceId = parseInt(card.getAttribute('data-service-id'));
        const status = statusMap.get(serviceId);

        if (!status) return;

        const statusElement = card.querySelector('.service-status');
        const updatedElement = card.querySelector('.service-updated');

        if (statusElement) {
            statusElement.textContent = status.status || '—';
        }

        if (updatedElement) {
            const updatedTime = status.updated_at || status.updatedat || status.updatedAt;
            if (updatedTime) {
                const newDate = new Date(updatedTime).toLocaleString('ru-RU');
                updatedElement.textContent = newDate;
            }
        }
    });

    lastServicesUpdateAt = Date.now();
}

function startServerPolling() {
    if (serverPollingInterval) {
        return; // Уже запущен
    }

    stopServerPolling();

    let consecutiveErrors = 0;
    const MAX_CONSECUTIVE_ERRORS = 3;

    const id = setInterval(async () => {
        if (document.hidden) {
            return;
        }

        try {
            // Если мы в списке серверов
            if (!serversListView.classList.contains('hidden') && serverDetailView.classList.contains('hidden')) {
                // Используем новый endpoint для получения статусов всех серверов
                const statuses = await apiRequest('/user/servers/statuses');
                if (Array.isArray(statuses)) {
                    updateServersStatus(statuses);
                    consecutiveErrors = 0;
                }
            } else {
                // Мы в деталях сервера - поллим только текущий сервер
                if (currentServerId) {
                    try {
                        const statusResponse = await apiRequest(`/user/servers/${currentServerId}/status`);
                        updateServersStatus([{
                            server_id: currentServerId,
                            status: statusResponse.status
                        }]);
                        consecutiveErrors = 0;
                    } catch (error) {
                        consecutiveErrors++;
                        console.error(`Ошибка полинга текущего сервера (${consecutiveErrors}/${MAX_CONSECUTIVE_ERRORS}):`, error);

                        if (consecutiveErrors >= MAX_CONSECUTIVE_ERRORS) {
                            console.warn('Слишком много ошибок полинга текущего сервера. Остановка полинга.');
                            stopServerPolling();
                        }
                    }
                }
            }
        } catch (error) {
            consecutiveErrors++;
            console.error(`Ошибка полинга серверов (${consecutiveErrors}/${MAX_CONSECUTIVE_ERRORS}):`, error);

            if (consecutiveErrors >= MAX_CONSECUTIVE_ERRORS) {
                console.warn('Слишком много ошибок полинга серверов. Остановка полинга.');
                stopServerPolling();
            }
        }
    }, CONFIG.POLLING_INTERVAL);
    AppTimers.addInterval(id);
    serverPollingInterval = id;
}

function stopServerPolling() {
    if (serverPollingInterval) {
        try { clearInterval(serverPollingInterval); } catch (e) {}
        AppTimers.intervals.delete(serverPollingInterval);
        serverPollingInterval = null;
    }
}

// ============================================
// SERVER STATUS INDICATOR HELPER
// ============================================

function updateServerStatusIndicator(el, status) {
    el.classList.remove(
        'server-status-ok',
        'server-status-degraded',
        'server-status-down',
        'server-status-pending'
    );

    if (!status) {
        el.classList.add('server-status-pending');
        return;
    }

    switch (status.toUpperCase()) {
        case 'OK':
            el.classList.add('server-status-ok');
            break;
        case 'DEGRADED':
            el.classList.add('server-status-degraded');
            break;
        case 'DOWN':
        case 'UNREACHABLE':
            el.classList.add('server-status-down');
            break;
        default:
            el.classList.add('server-status-pending');
    }
}

// ============================================
// AUTHENTICATION HANDLERS
// ============================================

async function handleLogin(event) {
    event.preventDefault();

    const username = document.getElementById('loginUsername').value;
    const password = document.getElementById('loginPassword').value;

    showLoading();

    try {
        const response = await apiRequest('/user/login', {
            method: 'POST',
            body: JSON.stringify({
                login: username,
                password: password
            })
        });

        currentUser = response.login || response.Login;
        localStorage.setItem('swsm_user', currentUser);

        document.documentElement.setAttribute('data-user-logged-in', 'true');

        // Полностью очищаем состояние при новом логине
        localStorage.removeItem('swsm_current_server_id');
        localStorage.removeItem(LS_SERVICES_PAGE_KEY);
        localStorage.removeItem(LS_SERVERS_PAGE_KEY);
        currentServerId = null;
        currentServerData = null;
        allServices = [];
        currentPage = 1;
        serversCurrentPage = 1;

        window._sessionExpiredNotified = false;

        showToast('Успех', 'Авторизация прошла успешно!');
        showMainApp();

        // Загружаем список серверов (не детали сервера)
        setTimeout(() => {
            showServersList();
            subscribeServerEvents();
        }, 100);

    } catch (error) {
        if (!(error instanceof SessionExpiredError)) {
            showToast('Ошибка', error.message, 'error');
        }
    } finally {
        hideLoading();
    }
}

async function handleRegister(event) {
    event.preventDefault();

    const username = document.getElementById('registerUsername').value.trim();
    const password = document.getElementById('registerPassword').value;
    const passwordConfirm = document.getElementById('registerPasswordConfirm').value;
    const registrationKey = document.getElementById('registrationKey').value.trim();

    if (password !== passwordConfirm) {
        showToast('Ошибка', 'Пароли не совпадают', 'error');
        return;
    }

    if (username.length < 4) {
        showToast('Ошибка', 'Логин должен содержать не менее 4 символов', 'error');
        return;
    }

    if (password.length < 5) {
        showToast('Ошибка', 'Пароль должен содержать не менее 5 символов', 'error');
        return;
    }

    showLoading();

    try {
        await apiRequest('/user/register', {
            method: 'POST',
            body: JSON.stringify({
                login: username,
                password: password,
                registration_key: registrationKey,
            }),
        });

        const modal = bootstrap.Modal.getInstance(document.getElementById('registerModal'));
        modal.hide();
        document.getElementById('registerForm').reset();

        showToast('Успех', 'Регистрация прошла успешно! Теперь авторизуйтесь.');

    } catch (error) {
        showToast('Ошибка', error.message, 'error');
    } finally {
        hideLoading();
    }
}

// 333333333333333333333333
function cleanupOnLogout() {
    // Закрываем SSE, останавливаем поллинг, очищаем таймеры
    if (serviceEventsSource) {
        try { serviceEventsSource.close(); } catch (e) {}
        serviceEventsSource = null;
    }

    if (serverSseReconnectTimerId) {
        try { clearTimeout(serverSseReconnectTimerId); } catch (e) {}
        AppTimers.timeouts.delete(serverSseReconnectTimerId);
        serverSseReconnectTimerId = null;
    }

    if (serverEventsSource) {
        // ОБНУЛЯЕМ обработчики перед закрытием
        serverEventsSource.onopen = null;
        serverEventsSource.onmessage = null;
        serverEventsSource.onerror = null;
        try { serverEventsSource.close(); } catch (e) {}
        serverEventsSource = null;
    }

    // Очищаем дебаунс ресайза
    if (resizeDebounceTimer) {
        try { clearTimeout(resizeDebounceTimer); } catch (e) {}
        AppTimers.timeouts.delete(resizeDebounceTimer);
        resizeDebounceTimer = null;
    }

    stopServicePolling();
    stopServerPolling();
    AppTimers.clearAll();

    // не очищаем DOM-слушатели для логина/регистрации (чтобы форма работала)
}

// function cleanupOnLogout() {
//     // Закрываем SSE, останавливаем поллинг, очищаем таймеры
//     if (serviceEventsSource) {
//         try { serviceEventsSource.close(); } catch (e) {}
//         serviceEventsSource = null;
//     }
//
//     if (serverSseReconnectTimerId) {
//         try { clearTimeout(serverSseReconnectTimerId); } catch (e) {}
//         AppTimers.timeouts.delete(serverSseReconnectTimerId);
//         serverSseReconnectTimerId = null;
//     }
//
//     if (serverEventsSource) {
//         // ОБНУЛЯЕМ обработчики перед закрытием
//         serverEventsSource.onopen = null;
//         serverEventsSource.onmessage = null;
//         serverEventsSource.onerror = null;
//         try { serverEventsSource.close(); } catch (e) {}
//         serverEventsSource = null;
//     }
//
//     stopServicePolling();
//     stopServerPolling();
//     AppTimers.clearAll();
//
//     // не очищаем DOM-слушатели для логина/регистрации (чтобы форма работала)
// }

function handleLogout() {
    // Cleanup runtime resources
    cleanupOnLogout();

    localStorage.removeItem('swsm_user');
    localStorage.removeItem('swsm_current_server_id');
    localStorage.removeItem(LS_SERVICES_PAGE_KEY);
    localStorage.removeItem(LS_SERVERS_PAGE_KEY);
    currentUser = null;
    currentServerId = null;
    currentServerData = null;
    allServices = [];
    currentPage = 1;
    allServers = [];
    serversCurrentPage = 1;
    window._sessionExpiredNotified = false;

    document.documentElement.removeAttribute('data-user-logged-in');

    showLoginPage();
    showToast('Информация', 'Вы вышли из системы');
}

// ============================================
// SERVER MANAGEMENT
// ============================================

async function loadServersList() {
    showLoading();

    try {
        // Получаем список серверов
        const servers = await apiRequest('/user/servers');
        allServers = (servers || []).slice(0, CONFIG.MAX_SERVERS_CACHE);

        // Получаем статусы всех серверов
        const statuses = await apiRequest('/user/servers/statuses');
        if (Array.isArray(statuses)) {
            // Обновляем статусы в allServers
            const statusMap = new Map();
            statuses.forEach(status => {
                const serverId = Number(status.server_id);
                if (!isNaN(serverId)) {
                    statusMap.set(serverId, status);
                }
            });

            allServers.forEach(server => {
                const sid = Number(server.id || server.server_id);
                if (!isNaN(sid)) {
                    const status = statusMap.get(sid);
                    if (status) {
                        server.status = status.status;
                    }
                }
            });
        }

        serversCurrentPage = parseInt(localStorage.getItem(LS_SERVERS_PAGE_KEY) || '1', 10);

        // Проверяем, что сохранённая страница не выходит за границы
        const pageSize = getPageSize();
        const maxPage = Math.max(1, Math.ceil(allServers.length / pageSize));
        if (serversCurrentPage > maxPage) {
            serversCurrentPage = maxPage;
            localStorage.setItem(LS_SERVERS_PAGE_KEY, String(serversCurrentPage));
        }

        renderServersCurrentPage();
        subscribeServerEvents();
    } catch (error) {
        if (!(error instanceof SessionExpiredError)) {
            showToast('Ошибка', 'Не удалось загрузить список серверов', 'error');
        }
    } finally {
        hideLoading();
    }
}

// Безопасный рендер серверов (не используем innerHTML с user-data)
function renderServersList(servers) {
    serversList.innerHTML = '';

    if (!servers || servers.length === 0) {
        serversList.innerHTML = `
            <div class="alert alert-info text-center">
                <i class="bi bi-info-circle me-2"></i>
                Серверы не добавлены. Нажмите "Добавить сервер" для начала работы.
            </div>
        `;
        return;
    }

    servers.forEach(server => {
        const col = document.createElement('div');
        col.className = 'col-md-6 col-lg-4';

        const card = document.createElement('div');
        card.className = 'card h-100 shadow-sm service-card server-card';
        card.setAttribute('data-server-id', server.id);

        const body = document.createElement('div');
        body.className = 'card-body';

        // Заголовок: лампочка + иконка + имя/адрес
        const title = document.createElement('h5');
        title.className = 'card-title mb-2';

        const indicator = document.createElement('span');
        indicator.className = 'server-status-indicator me-2';
        // привязка индикатора к id — полезно, но не обязательна
        indicator.setAttribute('data-server-id', server.id);
        // установить начальный цвет лампочки
        updateServerStatusIndicator(indicator, server.status);

        const icon = document.createElement('i');
        icon.className = 'bi bi-server me-2';

        const titleText = document.createTextNode(server.name || server.address || '');

        title.appendChild(indicator); // лампочка слева от имени
        title.appendChild(icon);
        title.appendChild(titleText);

        // Информационные строки (адрес, пользователь, дата, отпечаток)
        const p = document.createElement('p');
        p.className = 'card-text';

        const addr = document.createElement('small');
        addr.className = 'text-muted d-block';
        addr.innerHTML = `<i class="bi bi-geo-alt me-1"></i>`;
        addr.appendChild(document.createTextNode(server.address || ''));

        const status = document.createElement('small');
        status.className = 'text-muted d-block';
        status.innerHTML = `<i class="bi bi-hdd-network me-1"></i>`;
        const statusSpan = document.createElement('span');
        statusSpan.textContent = server.status;
        statusSpan.className = 'server-status-text';
        statusSpan.setAttribute('data-status', (server.status).toUpperCase());
        status.appendChild(statusSpan);

        // const status = document.createElement('small');
        // status.className = 'text-muted d-block';
        // status.innerHTML = `<i class="bi bi-hdd-network me-1"></i>`;
        // const statusSpan = document.createElement('span');
        // statusSpan.textContent = server.status;
        // status.appendChild(statusSpan);

        const user = document.createElement('small');
        user.className = 'text-muted d-block';
        user.innerHTML = `<i class="bi bi-person me-1"></i>`;
        user.appendChild(document.createTextNode(server.username || ''));

        const created = document.createElement('small');
        created.className = 'text-muted d-block';
        created.innerHTML = `<i class="bi bi-calendar-check me-1"></i>`;
        try {
            created.appendChild(document.createTextNode(new Date(server.created_at).toLocaleDateString('ru-RU')));
        } catch (e) {
            created.appendChild(document.createTextNode(''));
        }

        const fp = document.createElement('small');
        fp.className = 'text-muted d-block';
        fp.innerHTML = `<i class="bi bi-tags me-1"></i>`;
        fp.appendChild(document.createTextNode(server.fingerprint || ''));

        p.appendChild(addr);
        p.appendChild(status);
        p.appendChild(user);
        p.appendChild(created);
        p.appendChild(fp);

        body.appendChild(title);
        body.appendChild(p);

        const footer = document.createElement('div');
        footer.className = 'card-footer';
        const btn = document.createElement('button');
        btn.className = 'btn btn-primary btn-sm w-100';
        btn.innerHTML = `<i class="bi bi-list-task me-1"></i>Управление`;
        btn.onclick = () => showServerDetail(server.id);

        footer.appendChild(btn);
        card.appendChild(body);
        card.appendChild(footer);
        col.appendChild(card);
        serversList.appendChild(col);
    });
}

async function handleAddServer(event) {
    event.preventDefault();

    if (!canPerformAction('addServer')) return;

    const name = document.getElementById('serverName').value;
    const address = document.getElementById('serverAddress').value;
    const username = document.getElementById('serverUsername').value;
    const password = document.getElementById('serverPassword').value;

    showLoading();

    try {
        await apiRequest('/user/servers', {
            method: 'POST',
            body: JSON.stringify({
                name, address, username, password
            })
        });

        const modal = bootstrap.Modal.getInstance(document.getElementById('addServerModal'));
        modal.hide();
        document.getElementById('addServerForm').reset();

        showToast('Успех', `Сервер "${name}" успешно добавлен!`);
        loadServersList();

    } catch (error) {
        if (!(error instanceof SessionExpiredError)) {
            showToast('Ошибка', error.message, 'error');
        }
    } finally {
        hideLoading();
    }
}

// async function loadServerDetail(serverId) {
//     currentServerId = serverId;
//     showLoading();
//
//     try {
//         const server = await apiRequest(`/user/servers/${serverId}`);
//         currentServerData = server;
//         renderServerDetail(server);
//         await loadServicesList(serverId);
//
//         const editBtn = document.getElementById('editServerBtn');
//         if (editBtn) {
//             editBtn.onclick = openEditServerModalFromDetail;
//         }
//
//     } catch (error) {
//         if (!(error instanceof SessionExpiredError)) {
//             showToast('Ошибка', 'Не удалось загрузить информацию о сервере', 'error');
//             showServersList();
//         }
//     } finally {
//         hideLoading();
//     }
// }

async function loadServerDetail(serverId) {
    currentServerId = serverId;
    showLoading();
    try {
        const server = await apiRequest(`/user/servers/${serverId}`);
        currentServerData = server;
        renderServerDetail(server);
        await loadServicesList(serverId);

        // Убедитесь, что кнопка редактирования работает
        const editBtn = document.getElementById('editServerBtn');
        if (editBtn) {
            // Удаляем старый обработчик если есть
            editBtn.onclick = null;
            // Добавляем новый
            editBtn.onclick = () => openEditServerModalFromDetail();
        }
    } catch (error) {
        if (!(error instanceof SessionExpiredError)) {
            showToast('Ошибка', 'Не удалось загрузить информацию о сервере', 'error');
            showServersList();
        }
    } finally {
        hideLoading();
    }
}


function renderServerDetail(server) {
    document.getElementById('serverBreadcrumb').textContent = server.name || '';

    const nameEl = document.getElementById('serverDetailName');
    if (nameEl) {
        nameEl.textContent = '';

        // Лампочка статуса
        const indicator = document.createElement('span');
        indicator.id = 'serverDetailIndicator';
        indicator.className = 'server-status-indicator me-2';

        // Устанавливаем статус из данных сервера
        updateServerStatusIndicator(indicator, server.status);

        // Иконка сервера
        const icon = document.createElement('i');
        icon.className = 'bi bi-server me-2';

        // Название сервера
        const nameText = document.createTextNode(server.name || server.address || '');

        nameEl.appendChild(indicator);
        nameEl.appendChild(icon);
        nameEl.appendChild(nameText);
    }

    // Текстовый статус
    const detailStatusText = document.getElementById('serverDetailStatus') ||
        document.querySelector('#serverDetailName .server-status-text');
    if (detailStatusText) {
        detailStatusText.textContent = server.status || '—';
        detailStatusText.setAttribute('data-status', (server.status).toUpperCase());
    }

    const addrEl = document.getElementById('serverDetailAddress');
    if (addrEl) addrEl.textContent = server.address || '';

    const userEl = document.getElementById('serverDetailUsername');
    if (userEl) userEl.textContent = server.username || '';

    const createdEl = document.getElementById('serverDetailCreated');
    if (createdEl) {
        try {
            createdEl.textContent = new Date(server.created_at).toLocaleDateString('ru-RU');
        } catch {
            createdEl.textContent = '';
        }
    }

    const fpEl = document.getElementById('serverDetailFingerprint');
    if (fpEl) fpEl.textContent = server.fingerprint || '';

    currentServerId = server.id || server.server_id || null;
}

// updateServersStatus — принимает массив statuses (из SSE) и обновляет:
// - in-memory allServers (если есть)
// - карточки списка (.server-card)
// - детальную карточку (если текущая открыта)
function updateServersStatus(statuses) {
    if (!Array.isArray(statuses) || statuses.length === 0) {
        return;
    }

    const statusMap = new Map();
    statuses.forEach(status => {
        const serverId = Number(status.server_id);
        if (!isNaN(serverId)) {
            statusMap.set(serverId, status);
        }
    });

    // 1) Обновляем in-memory кэш allServers
    if (Array.isArray(allServers)) {
        allServers.forEach(server => {
            const sid = Number(server.id || server.server_id);
            if (!isNaN(sid)) {
                const updated = statusMap.get(sid);
                if (updated) {
                    server.status = updated.status;
                    server.updated_at = updated.updated_at || server.updated_at;
                }
            }
        });
    }

    // 2) Обновляем DOM-список: ищем все карточки .server-card
    try {
        document.querySelectorAll('.server-card').forEach(card => {
            const attr = card.getAttribute('data-server-id');
            const serverId = parseInt(attr, 10);
            if (!Number.isFinite(serverId)) return;

            const updated = statusMap.get(serverId);
            if (!updated) return;

            // Обновление лампочки внутри карточки
            const indicator = card.querySelector('.server-status-indicator');
            if (indicator) {
                updateServerStatusIndicator(indicator, updated.status);
            }

            // Обновление текстового статуса (ДОБАВЛЕНО)
            const statusSpan = card.querySelector('.server-status-text');
            if (statusSpan) {
                statusSpan.textContent = updated.status;
                statusSpan.setAttribute('data-status', (updated.status).toUpperCase());
            }
        });
    } catch (e) {
        console.error('updateServersStatus: error updating list DOM', e);
    }

    // 3) Обновляем карточку детального просмотра (если открыта)
    try {
        if (typeof currentServerId !== 'undefined' && currentServerId !== null) {
            const cid = Number(currentServerId);
            if (Number.isFinite(cid)) {
                const updated = statusMap.get(cid);
                if (updated) {
                    const indicator = document.getElementById('serverDetailIndicator');
                    if (indicator) {
                        updateServerStatusIndicator(indicator, updated.status);
                    }

                    const detailStatusText = document.getElementById('serverDetailStatus') ||
                        document.querySelector('#serverDetailName .server-status-text');
                    if (detailStatusText) {
                        detailStatusText.textContent = updated.status || '—';
                        detailStatusText.setAttribute('data-status', (updated.status).toUpperCase());
                    }
                }
            }
        }
    } catch (e) {
        console.error('updateServersStatus: error updating detail view', e);
    }
}

async function handleDeleteServer() {
    if (!currentServerId || !currentServerData) return;

    if (!confirm(`Вы уверены, что хотите удалить сервер "${currentServerData.name}"?`)) {
        return;
    }

    showLoading();

    try {
        await apiRequest(`/user/servers/${currentServerId}`, {
            method: 'DELETE'
        });

        showToast('Успех', `Сервер "${currentServerData.name}" удален`);
        showServersList();

    } catch (error) {
        if (!(error instanceof SessionExpiredError)) {
            showToast('Ошибка', error.message, 'error');
        }
    } finally {
        hideLoading();
    }
}

function openEditServerModalFromDetail() {
    if (!currentServerId || !currentServerData) {
        showToast('Ошибка', 'Данные сервера не загружены', 'error');
        return;
    }

    try {
        document.getElementById('editServerId').value = currentServerData.id;
        document.getElementById('editServerName').value = currentServerData.name || '';
        document.getElementById('editServerAddress').value = currentServerData.address || '';
        document.getElementById('editServerUsername').value = currentServerData.username || '';
        document.getElementById('editServerPassword').value = '';

        const modal = new bootstrap.Modal(document.getElementById('editServerModal'));
        modal.show();

    } catch (error) {
        showToast('Ошибка', 'Не удалось открыть форму редактирования', 'error');
    }
}

async function handleEditServer(event) {
    event.preventDefault();
    const serverId = document.getElementById('editServerId').value;
    const name = document.getElementById('editServerName').value;
    const address = document.getElementById('editServerAddress').value;
    const username = document.getElementById('editServerUsername').value;
    const password = document.getElementById('editServerPassword').value;

    showLoading();

    try {
        const serverData = { name, address, username };
        if (password.trim()) {
            serverData.password = password;
        }

        await apiRequest(`/user/servers/${serverId}`, {
            method: 'PATCH',
            body: JSON.stringify(serverData)
        });

        // Закрытие модали ПЕРВЫМ
        const modal = bootstrap.Modal.getInstance(document.getElementById('editServerModal'));
        modal.hide();
        document.getElementById('editServerForm').reset();

        // Обновление данных сразу
        currentServerData.name = name;
        currentServerData.address = address;
        currentServerData.username = username;
        renderServerDetail(currentServerData);

        showToast('Успешно', 'Сервер успешно отредактирован!');

        // Обновление списка серверов
        if (currentServerId === parseInt(serverId)) {
            loadServersList();
        }
    } catch (error) {
        if (!(error instanceof SessionExpiredError)) {
            showToast('Ошибка', error.message, 'error');
        }
    } finally {
        hideLoading();
    }
}

// ============================================
// SERVICE MANAGEMENT
// ============================================

async function loadServicesList(serverId, silent = false) {
    if (!silent) showLoading();

    try {
        const cacheBuster = Date.now();
        const services = await apiRequest(`/user/servers/${serverId}/services?_t=${cacheBuster}`);
        allServices = services.slice(0, CONFIG.MAX_SERVICES_CACHE);

        // Восстанавливаем сохранённый номер страницы
        currentPage = parseInt(localStorage.getItem(LS_SERVICES_PAGE_KEY) || '1', 10);

        // Проверяем, что сохранённая страница не выходит за границы
        const pageSize = getPageSize();
        const maxPage = Math.max(1, Math.ceil(allServices.length / pageSize));
        if (currentPage > maxPage) {
            currentPage = maxPage;
        }

        localStorage.setItem(LS_SERVICES_PAGE_KEY, String(currentPage));
        lastServicesUpdateAt = Date.now();
        renderCurrentPage();

    } catch (error) {
        if (!silent && !(error instanceof SessionExpiredError)) {
            showToast('Ошибка', 'Не удалось загрузить список служб', 'error');
        }
        throw error;
    } finally {
        if (!silent) hideLoading();
    }
}

function renderServicesList(services) {
    servicesList.innerHTML = '';

    const refreshBtn = document.getElementById('refreshFromServerBtn');

    if (!services || services.length === 0) {
        servicesList.innerHTML = `
            <div class="col-12">
                <div class="alert alert-warning text-center">
                    <i class="bi bi-exclamation-triangle me-2"></i>
                    Список служб пуст
                </div>
            </div>`;

        if (refreshBtn) refreshBtn.style.display = 'none';
        return;
    }

    services.forEach(service => {
        const template = document.getElementById('serviceCardTemplate');
        const serviceCard = template.content.cloneNode(true);

        serviceCard.querySelector('.service-displayed-name').textContent = service.displayed_name;
        serviceCard.querySelector('.service-name').textContent = service.service_name;
        serviceCard.querySelector('.service-status').textContent = service.status || '—';
        serviceCard.querySelector('.service-updated').textContent = service.updated_at ?
            new Date(service.updated_at).toLocaleString('ru-RU') : '—';

        const card = serviceCard.querySelector('.service-card');
        card.setAttribute('data-service-id', service.id);

        const startBtn = serviceCard.querySelector('.service-start-btn');
        const stopBtn = serviceCard.querySelector('.service-stop-btn');
        const restartBtn = serviceCard.querySelector('.service-restart-btn');
        const deleteBtn = serviceCard.querySelector('.service-delete-btn');

        // Простые присваивания, БЕЗ стрелочных функций
        startBtn.onclick = function() {
            controlService(service.id, 'start', service.displayed_name);
        };
        stopBtn.onclick = function() {
            controlService(service.id, 'stop', service.displayed_name);
        };
        restartBtn.onclick = function() {
            controlService(service.id, 'restart', service.displayed_name);
        };
        deleteBtn.onclick = function() {
            handleDeleteService(service.id, service.displayed_name);
        };

        servicesList.appendChild(serviceCard);
    });

    if (refreshBtn) refreshBtn.style.display = 'inline-block';
}

async function handleAddService(event) {
    event.preventDefault();

    if (!currentServerId) {
        showToast('Ошибка', 'Сначала выберите сервер', 'error');
        return;
    }

    const displayedName = document.getElementById('serviceDisplayedName').value;
    const serviceName = document.getElementById('serviceName').value;

    showLoading();

    try {
        const response = await apiRequest(`/user/servers/${currentServerId}/services`, {
            method: 'POST',
            body: JSON.stringify({
                displayed_name: displayedName,
                service_name: serviceName
            })
        });

        const modal = bootstrap.Modal.getInstance(document.getElementById('addServiceModal'));
        modal.hide();
        document.getElementById('addServiceForm').reset();

        showToast('Успех', `Служба "${response.displayed_name}" успешно добавлена!`);
        loadServicesList(currentServerId);

    } catch (error) {
        if (!(error instanceof SessionExpiredError)) {
            showToast('Ошибка', error.message, 'error');
        }
    } finally {
        hideLoading();
    }
}

async function controlService(serviceId, action, serviceName) {
    if (!canPerformAction(`service_${action}`)) return;

    showLoading();

    try {
        let endpoint;
        switch (action) {
            case 'start':
                endpoint = `/user/servers/${currentServerId}/services/${serviceId}/start`;
                break;
            case 'stop':
                endpoint = `/user/servers/${currentServerId}/services/${serviceId}/stop`;
                break;
            case 'restart':
                endpoint = `/user/servers/${currentServerId}/services/${serviceId}/restart`;
                break;
            default:
                throw new Error('Неизвестное действие');
        }

        const response = await apiRequest(endpoint, { method: 'POST' });
        const updatedService = await apiRequest(`/user/servers/${currentServerId}/services/${serviceId}`);

        // Обновление UI
        const card = document.querySelector(`[data-service-id="${serviceId}"]`);
        if (card) {
            const statusElement = card.querySelector('.service-status');
            const updatedElement = card.querySelector('.service-updated');

            if (statusElement) {
                statusElement.textContent = updatedService.status || '—';
            }
            if (updatedElement) {
                updatedElement.textContent = updatedService.updated_at ?
                    new Date(updatedService.updated_at).toLocaleString('ru-RU') : '—';
            }
        }

        const actionText = action === 'start' ? 'запущена' : action === 'stop' ? 'остановлена' : 'перезапущена';
        showToast('Успех', response.message || `Служба "${serviceName}" ${actionText}`);

    } catch (error) {
        if (!(error instanceof SessionExpiredError)) {
            showToast('Ошибка', `Не удалось выполнить операцию "${action}" для службы "${serviceName}. Ошибка: ${error.message}"`, 'error');
        }
    } finally {
        hideLoading();
    }
}

async function handleDeleteService(serviceId, serviceName) {
    if (!confirm(`Вы уверены, что хотите удалить службу "${serviceName}"?`)) {
        return;
    }

    showLoading();

    try {
        await apiRequest(`/user/servers/${currentServerId}/services/${serviceId}`, {
            method: 'DELETE'
        });

        showToast('Успех', `Служба "${serviceName}" удалена`);
        loadServicesList(currentServerId);

    } catch (error) {
        if (!(error instanceof SessionExpiredError)) {
            showToast('Ошибка', error.message, 'error');
        }
    } finally {
        hideLoading();
    }
}

async function handleRefreshFromServer() {
    if (!currentServerId) {
        showToast('Ошибка', 'Сначала выберите сервер', 'error');
        return;
    }

    showLoading();

    try {
        const url = `${API_BASE}/user/servers/${currentServerId}/services?actual=true`;
        const response = await fetch(url, {
            method: 'GET',
            headers: { 'Content-Type': 'application/json' },
            credentials: 'include'
        });

        const isUpdated = response.headers.get('X-Is-Updated');
        const data = await response.json();

        allServices = data.slice(0, CONFIG.MAX_SERVICES_CACHE);

        // Восстанавливаем сохранённый номер страницы
        currentPage = parseInt(localStorage.getItem(LS_SERVICES_PAGE_KEY) || '1', 10);

        // Проверяем, что сохранённая страница не выходит за границы
        const pageSize = getPageSize();
        const maxPage = Math.max(1, Math.ceil(allServices.length / pageSize));
        if (currentPage > maxPage) {
            currentPage = maxPage;
        }

        localStorage.setItem(LS_SERVICES_PAGE_KEY, String(currentPage));
        lastServicesUpdateAt = Date.now();
        renderCurrentPage();

        if (isUpdated === 'false') {
            showToast('Предупреждение', 'Проблемы со связью. Показаны данные из кэша.', 'warning');
        } else {
            showToast('Успех', 'Статусы служб обновлены с сервера');
        }

    } catch (error) {
        showToast('Ошибка', 'Не удалось обновить статусы', 'error');
    } finally {
        hideLoading();
    }
}

// ============================================
// AVAILABLE SERVICES LOADING
// ============================================

let availableServices = [];
let selectedService = null;

async function loadAvailableServices(serverId) {
    try {
        const data = await apiRequest(`/user/servers/${serverId}/services/available`);
        availableServices = data.services || [];
        return availableServices;
    } catch (error) {
        console.error('Ошибка загрузки доступных служб:', error);
        showToast('Ошибка', 'Не удалось загрузить список служб', 'error');
        return [];
    }
}

function renderServicesInList(services) {
    const container = document.getElementById('serviceListContainer');

    if (!services || services.length === 0) {
        container.innerHTML = '<div class="text-center service-list-empty p-3">Службы не найдены</div>';
        return;
    }

    container.innerHTML = '';

    services.forEach(service => {
        const item = document.createElement('div');
        item.className = 'service-item';
        item.dataset.serviceName = service.name;
        item.dataset.displayName = service.display_name;

        item.innerHTML = `
            <div class="service-item-name">${escapeHtml(service.name)}</div>
            <div class="service-item-display">${escapeHtml(service.display_name)}</div>
        `;

        item.addEventListener('click', () => selectService(service, item));

        container.appendChild(item);
    });
}

function selectService(service, itemElement) {
    selectedService = service;

    // Убираем выделение со всех элементов
    document.querySelectorAll('.service-item').forEach(el => {
        el.classList.remove('selected');
    });

    // Выделяем выбранный элемент
    if (itemElement) {
        itemElement.classList.add('selected');
    }

    // Заполняем поля
    document.getElementById('selectedServiceName').value = service.name;
    document.getElementById('serviceDisplayName').value = service.display_name;
}

// Поиск
document.getElementById('serviceSearch').addEventListener('input', (e) => {
    const searchTerm = e.target.value.toLowerCase().trim();

    if (!searchTerm) {
        renderServicesInList(availableServices);
        return;
    }

    const filtered = availableServices.filter(service =>
        service.name.toLowerCase().includes(searchTerm) ||
        service.display_name.toLowerCase().includes(searchTerm)
    );

    renderServicesInList(filtered);
});

// Открытие модалки
document.getElementById('addServiceModal').addEventListener('show.bs.modal', async () => {
    if (!currentServerId) return;

    const container = document.getElementById('serviceListContainer');
    container.innerHTML = '<div class="text-center service-list-loading p-3"><div class="spinner-border spinner-border-sm me-2"></div>Загрузка служб...</div>';

    const services = await loadAvailableServices(currentServerId);
    renderServicesInList(services);
});

// Закрытие модалки
document.getElementById('addServiceModal').addEventListener('hide.bs.modal', () => {
    document.getElementById('addServiceForm').reset();
    document.getElementById('serviceSearch').value = '';
    document.getElementById('selectedServiceName').value = '';
    document.getElementById('serviceDisplayName').value = '';
    document.getElementById('serviceListContainer').innerHTML = '';
    selectedService = null;
});

// Добавление службы
document.getElementById('addServiceBtn').addEventListener('click', async () => {
    const serviceName = document.getElementById('selectedServiceName').value;
    const displayedName = document.getElementById('serviceDisplayName').value;

    if (!serviceName) {
        showToast('Ошибка', 'Выберите службу', 'error');
        return;
    }

    if (!canPerformAction('addService')) {
        return;
    }

    showLoading();
    try {
        await apiRequest(`/user/servers/${currentServerId}/services`, {
            method: 'POST',
            body: JSON.stringify({
                service_name: serviceName,
                displayed_name: displayedName
            })
        });

        showToast('Успех', 'Служба добавлена');
        bootstrap.Modal.getInstance(document.getElementById('addServiceModal')).hide();
        loadServicesList(currentServerId);
    } catch (error) {
        showToast('Ошибка', error.message, 'error');
    } finally {
        hideLoading();
    }
});

// Утилита для экранирования HTML
function escapeHtml(text) {
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
}

// ============================================
// PAGINATION
// ============================================

function renderCurrentPage() {
    if (!allServices || allServices.length === 0) {
        renderServicesList([]);
        const pageIndicator = document.getElementById('pageIndicator');
        const prevPageBtn = document.getElementById('prevPageBtn');
        const nextPageBtn = document.getElementById('nextPageBtn');
        if (pageIndicator) {
            pageIndicator.textContent = 'Страница 0 из 0';
            pageIndicator.style.display = 'none';
        }
        if (prevPageBtn) prevPageBtn.style.display = 'none';
        if (nextPageBtn) nextPageBtn.style.display = 'none';
        return;
    }

    const minItems = getMinItemsForPagination('services');
    const pageIndicator = document.getElementById('pageIndicator');
    const prevPageBtn = document.getElementById('prevPageBtn');
    const nextPageBtn = document.getElementById('nextPageBtn');

    const shouldPaginate = allServices.length > minItems;

    if (!shouldPaginate) {
        renderServicesList(allServices);
        if (pageIndicator) pageIndicator.style.display = 'none';
        if (prevPageBtn) prevPageBtn.style.display = 'none';
        if (nextPageBtn) nextPageBtn.style.display = 'none';
        currentPage = 1;
        totalPages = 1;
        return;
    }

    const pageSize = getPageSize();
    totalPages = Math.max(1, Math.ceil(allServices.length / pageSize));
    if (currentPage > totalPages) currentPage = totalPages;

    const start = (currentPage - 1) * pageSize;
    const end = start + pageSize;
    const pageServices = allServices.slice(start, end);

    renderServicesList(pageServices);

    if (pageIndicator) {
        pageIndicator.style.display = '';
        pageIndicator.textContent = `Страница ${currentPage} из ${totalPages}`;
    }
    if (prevPageBtn) {
        prevPageBtn.style.display = '';
        prevPageBtn.disabled = currentPage === 1;
    }
    if (nextPageBtn) {
        nextPageBtn.style.display = '';
        nextPageBtn.disabled = currentPage === totalPages;
    }
}

function renderServersCurrentPage() {
    const paginationControls = document.querySelector('.servers-pagination-controls');
    if (!allServers || allServers.length === 0) {
        renderServersList([]);
        const pageIndicator = document.getElementById('serversPageIndicator');
        const prevPageBtn = document.getElementById('serversPrevPageBtn');
        const nextPageBtn = document.getElementById('serversNextPageBtn');
        if (pageIndicator) {
            pageIndicator.textContent = 'Страница 0 из 0';
            pageIndicator.style.display = 'none';
        }
        if (prevPageBtn) prevPageBtn.style.display = 'none';
        if (nextPageBtn) nextPageBtn.style.display = 'none';
        if (paginationControls) paginationControls.style.display = 'none';
        return;
    }

    const minItems = getMinItemsForPagination('servers');
    const pageIndicator = document.getElementById('serversPageIndicator');
    const prevPageBtn = document.getElementById('serversPrevPageBtn');
    const nextPageBtn = document.getElementById('serversNextPageBtn');

    const shouldPaginate = allServers.length > minItems;

    if (!shouldPaginate) {
        renderServersList(allServers);
        if (pageIndicator) pageIndicator.style.display = 'none';
        if (prevPageBtn) prevPageBtn.style.display = 'none';
        if (nextPageBtn) nextPageBtn.style.display = 'none';
        if (paginationControls) paginationControls.style.display = 'none';
        serversCurrentPage = 1;
        return;
    }

    const pageSize = getPageSize();
    serversTotalPages = Math.max(1, Math.ceil(allServers.length / pageSize));
    if (serversCurrentPage > serversTotalPages) serversCurrentPage = serversTotalPages;

    const start = (serversCurrentPage - 1) * pageSize;
    const end = start + pageSize;
    const pageServers = allServers.slice(start, end);

    renderServersList(pageServers);

    if (pageIndicator) {
        pageIndicator.style.display = '';
        pageIndicator.textContent = `Страница ${serversCurrentPage} из ${serversTotalPages}`;
    }
    if (prevPageBtn) {
        prevPageBtn.style.display = '';
        prevPageBtn.disabled = serversCurrentPage === 1;
    }
    if (nextPageBtn) {
        nextPageBtn.style.display = '';
        nextPageBtn.disabled = serversCurrentPage === serversTotalPages;
    }

    if (paginationControls) {
        paginationControls.style.display = shouldPaginate ? 'flex' : 'none';
    }
}

// ============================================
// EVENT LISTENERS (setup once)
// ============================================

function setupEventListeners() {
    const loginForm = document.getElementById('loginForm');
    if (loginForm) loginForm.addEventListener('submit', handleLogin);

    const registerForm = document.getElementById('registerForm');
    if (registerForm) registerForm.addEventListener('submit', handleRegister);

    const addServerForm = document.getElementById('addServerForm');
    if (addServerForm) addServerForm.addEventListener('submit', handleAddServer);

    const editServerForm = document.getElementById('editServerForm');
    if (editServerForm) editServerForm.addEventListener('submit', handleEditServer);

    const editServerModal = document.getElementById('editServerModal');
    if (editServerModal) {
        editServerModal.addEventListener('show.bs.modal', function () {
            if (currentServerData) {
                document.getElementById('editServerId').value = currentServerData.id;
                document.getElementById('editServerName').value = currentServerData.name;
                document.getElementById('editServerAddress').value = currentServerData.address;
                document.getElementById('editServerUsername').value = currentServerData.username;
                document.getElementById('editServerPassword').value = '';
            } else {
                showToast('Ошибка', 'Данные сервера не загружены', 'error');
                bootstrap.Modal.getInstance(editServerModal)?.hide();
            }
        });
    }

    const addServiceForm = document.getElementById('addServiceForm');
    if (addServiceForm) addServiceForm.addEventListener('submit', handleAddService);

    const refreshBtn = document.getElementById('refreshFromServerBtn');
    if (refreshBtn) refreshBtn.addEventListener('click', handleRefreshFromServer);

    const logoutBtn = document.getElementById('logoutBtn');
    if (logoutBtn) logoutBtn.addEventListener('click', handleLogout);

    const deleteServerBtn = document.getElementById('deleteServerBtn');
    if (deleteServerBtn) deleteServerBtn.addEventListener('click', handleDeleteServer);

    // Очистка модальных backdrop'ов после закрытия модали
    document.querySelectorAll('.modal').forEach(modalEl => {
        modalEl.addEventListener('hidden.bs.modal', function() {
            document.querySelectorAll('.modal-backdrop').forEach(b => b.remove());
            document.body.classList.remove('modal-open', 'overflow');
            document.body.style.overflow = '';
        });
    });

    // Обработчики пагинации
    const prevPageBtn = document.getElementById('prevPageBtn');
    const nextPageBtn = document.getElementById('nextPageBtn');
    if (prevPageBtn) {
        prevPageBtn.addEventListener('click', () => {
            if (currentPage > 1) {
                currentPage--;
                localStorage.setItem(LS_SERVICES_PAGE_KEY, String(currentPage));
                renderCurrentPage();
            }
        });
    }

    if (nextPageBtn) {
        nextPageBtn.addEventListener('click', () => {
            if (currentPage < totalPages) {
                currentPage++;
                localStorage.setItem(LS_SERVICES_PAGE_KEY, String(currentPage));
                renderCurrentPage();
            }
        });
    }

    // Обработчики пагинации для серверов
    const serversPrevPageBtn = document.getElementById('serversPrevPageBtn');
    const serversNextPageBtn = document.getElementById('serversNextPageBtn');
    if (serversPrevPageBtn) {
        serversPrevPageBtn.addEventListener('click', () => {
            if (serversCurrentPage > 1) {
                serversCurrentPage--;
                localStorage.setItem(LS_SERVERS_PAGE_KEY, String(serversCurrentPage));
                renderServersCurrentPage();
            }
        });
    }

    if (serversNextPageBtn) {
        serversNextPageBtn.addEventListener('click', () => {
            if (serversCurrentPage < serversTotalPages) {
                serversCurrentPage++;
                localStorage.setItem(LS_SERVERS_PAGE_KEY, String(serversCurrentPage));
                renderServersCurrentPage();
            }
        });
    }
}

document.addEventListener('visibilitychange', () => {
    if (document.hidden) {
        handlePageBackground();
        // Закрываем SSE для серверов
        if (serverEventsSource) {
            try { serverEventsSource.close(); } catch (e) {}
            serverEventsSource = null;
        }
    } else {
        handlePageResume();
        // Переподключаемся к серверам
        subscribeServerEvents();
    }
}, false);

// ==========================================
// PAGE VISIBILITY OPTIMIZATION (single handler)
// ==========================================

let isPageVisible = true;

function handlePageBackground() {
    // Закрываем SSE и полинг, но НЕ очищаем allServices
    if (serviceEventsSource) {
        try { serviceEventsSource.close(); } catch (e) { /* noop */ }
        serviceEventsSource = null;
        sseConnectionStatus = 'closed';
        sseReconnectAttempts = 0;
    }

    if (sseReconnectTimerId) {
        try { clearTimeout(sseReconnectTimerId); } catch (e) {}
        AppTimers.timeouts.delete(sseReconnectTimerId);
        sseReconnectTimerId = null;
    }

    stopServicePolling();
    stopServerPolling();

    // Закрываем SSE для серверов
    if (serverEventsSource) {
        try { serverEventsSource.close(); } catch (e) {}
        serverEventsSource = null;
    }

    // Очищаем таймер переподключения для серверов
    if (serverSseReconnectTimerId) {
        try { clearTimeout(serverSseReconnectTimerId); } catch (e) {}
        AppTimers.timeouts.delete(serverSseReconnectTimerId);
        serverSseReconnectTimerId = null;
    }
}

const MIN_RESUME_INTERVAL = 1000;
let lastResumeTime = 0;

function handlePageResume() {
    const now = Date.now();
    if (now - lastResumeTime < 300) return; // Уменьшено с 1000 до 300мс
    lastResumeTime = now;

    if (!currentUser || !currentServerId || document.hidden) {
        return;
    }

    switchConnectionsMode();

    // Для мобильных устройств - немедленно обновляем данные
    if (isMobileDevice()) {
        // Немедленный запрос актуальных данных
        loadServicesList(currentServerId, true).catch(err => {
            console.warn('Ошибка при обновлении данных:', err);
        });

        // Запускаем поллинг (включая немедленный запрос)
        if (!servicePollingInterval) {
            startServicePolling(currentServerId);
        }
    } else {
        // Для десктопа - обычная логика
        if (!serviceEventsSource || serviceEventsSource.readyState === EventSource.CLOSED) {
            subscribeServiceEvents(currentServerId);
        }
    }

    // Всегда переподключаемся к серверам
    subscribeServerEvents();
}

// ============================================
// SHOW/HIDE FUNCTIONS
// ============================================

function showLoginPage() {
    if (loginPage) {
        loginPage.classList.remove('hidden');
        loginPage.style.display = '';  // Убрать inline display, использовать CSS классы
    }
    if (mainApp) {
        mainApp.classList.add('hidden');
        mainApp.style.display = 'none';  // Явно скрыть
    }
    // Удалить атрибут авторизации
    document.documentElement.removeAttribute('data-user-logged-in');

    localStorage.removeItem('swsm_current_server_id');
    currentServerId = null;
    window._sessionExpiredNotified = false;

    if (serviceEventsSource) {
        try { serviceEventsSource.close(); } catch (e) {}
        serviceEventsSource = null;
    }
    stopServicePolling();
}

function showMainApp() {
    if (loginPage) {
        loginPage.classList.add('hidden');
        loginPage.style.display = 'none';  // Явно скрыть
    }
    if (mainApp) {
        mainApp.classList.remove('hidden');
        mainApp.style.display = '';  // Убрать inline display, использовать CSS классы
    }
    if (currentUser && currentUserSpan) {
        currentUserSpan.textContent = currentUser;
    }
}

function showServersList() {
    serversListView.classList.remove('hidden');
    serverDetailView.classList.add('hidden');
    localStorage.removeItem('swsm_current_server_id');

    // ПОЛНАЯ ОЧИСТКА UI/контекста
    currentServerId = null;
    currentServerData = null;
    allServices = [];
    currentPage = 1;
    totalPages = 1;
    sseReconnectAttempts = 0;
    serversCurrentPage = 1;
    serversTotalPages = 1;

    // Остановка полинга служб
    stopServicePolling();

    // Перезапускаем поллинг серверов (он теперь будет поллить все серверы)
    stopServerPolling();

    // Загрузка списка серверов
    loadServersList();

    // Подписываемся на события серверов (для мобильных - поллинг, для десктопа - SSE)
    subscribeServerEvents();
}

function showServerDetail(serverId) {
    serversListView.classList.add('hidden');
    serverDetailView.classList.remove('hidden');

    currentServerId = serverId;
    localStorage.setItem('swsm_current_server_id', serverId);

    // Подписываемся на обновления статусов серверов.
    // Обратите внимание: для мобильных устройств это запустит поллинг,
    // который теперь учитывает, что мы в деталях и будет поллить только текущий сервер
    subscribeServerEvents();

    // Попытка взять объект сервера из кеша allServers
    let cached = null;
    if (Array.isArray(allServers)) {
        cached = allServers.find(s => Number(s.id || s.server_id) === Number(serverId));
    }

    if (cached) {
        // используем кеш для немедленного рендера (включая статус)
        currentServerData = cached;
        renderServerDetail(cached);

        // загружаем список служб без спиннера (silent = true)
        loadServicesList(serverId, true).catch(err => {
            console.warn('Ошибка загрузки служб при открытии деталки (silent):', err);
            // fallback: запустим поллинг как запасной вариант
            startServicePolling(serverId);
        });

        // Фоновый апдейт деталей сервера
        let lastServerDetailRequestId = 0;
        const requestId = ++lastServerDetailRequestId;

        (async () => {
            try {
                const fresh = await apiRequest(`/user/servers/${serverId}`);
                // Проверяем, актуален ли еще этот запрос
                if (requestId === lastServerDetailRequestId && currentServerId === serverId) {
                    fresh.status = cached.status || fresh.status;
                    currentServerData = Object.assign({}, cached, fresh);
                    renderServerDetail(currentServerData);
                }
            } catch (e) {
                console.debug('Фоновый апдейт детальки не удался:', e);
            }
        })();
    } else {
        // если в кеше нет — делаем обычный запрос и рендерим как раньше
        loadServerDetail(serverId);
    }

    // Подписываемся на SSE для служб этого сервера
    subscribeServiceEvents(serverId);
}

function showLoading() {
    if (loadingSpinner) loadingSpinner.style.display = 'block';
}

function hideLoading() {
    if (loadingSpinner) loadingSpinner.style.display = 'none';
}

// ============================================
// Before unload - make sure timers are cleared
// ============================================

window.addEventListener('beforeunload', () => {
    AppTimers.clearAll();
    if (serviceEventsSource) {
        try { serviceEventsSource.close(); } catch (e) {}
        serviceEventsSource = null;
    }

    cleanupOnLogout();
});

// ============================================
// Cleanup on page unload
// ============================================

window.addEventListener('beforeunload', () => {
    AppTimers.clearAll();
    if (serviceEventsSource) {
        try { serviceEventsSource.close(); } catch (e) {}
        serviceEventsSource = null;
    }
    if (serverEventsSource) {
        try { serverEventsSource.close(); } catch (e) {}
        serverEventsSource = null;
    }

    // Очищаем все таймеры
    if (resizeDebounceTimer) {
        try { clearTimeout(resizeDebounceTimer); } catch (e) {}
        resizeDebounceTimer = null;
    }

    if (sseReconnectTimerId) {
        try { clearTimeout(sseReconnectTimerId); } catch (e) {}
        sseReconnectTimerId = null;
    }

    if (serverSseReconnectTimerId) {
        try { clearTimeout(serverSseReconnectTimerId); } catch (e) {}
        serverSseReconnectTimerId = null;
    }
});

// ============================================
// Cleanup on page hide (for SPA navigation if implemented later)
// ============================================

function cleanupApp() {
    cleanupOnLogout();

    // Очищаем массивы
    allServers = [];
    allServices = [];
    toastHistory = [];
    REQUEST_RATE_LIMIT.clear();
    AppTimers.clearAll();

    // Очищаем обработчики событий
    window.removeEventListener('resize', onWindowResize);
    document.removeEventListener('visibilitychange', handlePageBackground);
}