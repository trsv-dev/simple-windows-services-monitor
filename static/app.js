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
    
    // Pagination
    PAGE_SIZE_MOBILE: 6,
    PAGE_SIZE_DESKTOP: 9,
    MIN_ITEMS_PAGINATION_MOBILE: 5,
    MIN_ITEMS_PAGINATION_DESKTOP: 9,
    MIN_SERVERS_PAGINATION_MOBILE: 3,
    MIN_SERVERS_PAGINATION_DESKTOP: 9,
    
    // Polling
    POLLING_INTERVAL: 5000
};

// ============================================
// GLOBAL STATE
// ============================================

let currentUser = localStorage.getItem('swsm_user');
let currentServerId = localStorage.getItem('swsm_current_server_id');
let currentServerData = null;

// Pagination state for servers
let allServers = [];
let serversCurrentPage = 1;
let serversTotalPages = 1;

// Pagination state for services
let allServices = [];
let currentPage = 1;
let totalPages = 1;

// SSE EventSource
let serviceEventsSource = null;
let servicePollingInterval = null;
let sseReconnectAttempts = 0;
let sseConnectionStatus = 'closed'; // 'closed', 'connecting', 'open'

// Rate limiting
const REQUEST_RATE_LIMIT = {};

// Page size cache
let cachedPageSize = null;
let lastWindowWidth = window.innerWidth;

// Toast history for deduplication
let toastHistory = [];

// Session expired flag
window._sessionExpiredNotified = false;

// ============================================
// DOM ELEMENTS
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
    if (currentUser) {
        showMainApp();

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

function canPerformAction(actionName) {
    const now = Date.now();
    const lastTime = REQUEST_RATE_LIMIT[actionName] || 0;
    
    if (now - lastTime < CONFIG.RATE_LIMIT_DELAY) {
        console.warn(`Действие '${actionName}' ограничено по времени`);
        return false;
    }
    
    REQUEST_RATE_LIMIT[actionName] = now;
    return true;
}

function isMobileDevice() {
    return /iPhone|iPad|iPod|Android/i.test(navigator.userAgent);
}

function getPageSize() {
    if (cachedPageSize === null || window.innerWidth !== lastWindowWidth) {
        lastWindowWidth = window.innerWidth;
        cachedPageSize = window.innerWidth < 768 ? CONFIG.PAGE_SIZE_MOBILE : CONFIG.PAGE_SIZE_DESKTOP;
    }
    return cachedPageSize;
}

function getMinItemsForPagination(type) {
    const isMobile = window.innerWidth < 768;
    if (type === 'services') {
        return isMobile ? CONFIG.MIN_ITEMS_PAGINATION_MOBILE : CONFIG.MIN_ITEMS_PAGINATION_DESKTOP;
    } else if (type === 'servers') {
        return isMobile ? CONFIG.MIN_SERVERS_PAGINATION_MOBILE : CONFIG.MIN_SERVERS_PAGINATION_DESKTOP;
    }
    return 0;
}

// Очистка кэша при изменении размера окна
window.addEventListener('resize', () => {
    cachedPageSize = null;
});

// Обработка изменения видимости страницы - экономия памяти при неактивной вкладке
document.addEventListener('visibilitychange', () => {
    if (document.hidden) {

        // Закрытие SSE соединения
        if (serviceEventsSource) {
            serviceEventsSource.close();
            serviceEventsSource = null;
            sseConnectionStatus = 'closed';
        }

        // Закрытие полинга
        stopServicePolling();

    } else {

        // Переподключение при возврате на вкладку
        if (currentServerId) {
            subscribeServiceEvents(currentServerId);
        }
    }
});

// ============================================
// TOAST NOTIFICATIONS (with deduplication)
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
            if (!window._sessionExpiredNotified) {
                window._sessionExpiredNotified = true;
                
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
                    serviceEventsSource.close();
                    serviceEventsSource = null;
                }
                
                stopServicePolling();

                showToast('Сессия истекла', 'Ваша сессия истекла. Пожалуйста, авторизуйтесь снова.', 'warning');
                showLoginPage();
            }
            throw new SessionExpiredError('Session expired');
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
// SSE FUNCTIONS (with improvements)
// ============================================

function subscribeServiceEvents(serverId) {
    // Использование полинга для мобильных устройств
    if (isMobileDevice()) {
        startServicePolling(serverId);
        return;
    }

    // КЛЮЧЕВАЯ ПРОВЕРКА: если уже есть открытое соединение - не создаём новое!
    if (serviceEventsSource && serviceEventsSource.readyState === EventSource.OPEN) {
        sseConnectionStatus = 'open';
        return;
    }

    // Закрытие старого соединения если оно есть и повреждено
    if (serviceEventsSource) {
        serviceEventsSource.close();
        serviceEventsSource = null;
    }

    const url = `${API_BASE}/user/broadcasting`;

    serviceEventsSource = new EventSource(url, { withCredentials: true });
    sseConnectionStatus = 'connecting';

    // Обработчик открытия соединения
    serviceEventsSource.onopen = function() {
        sseConnectionStatus = 'open';
        sseReconnectAttempts = 0;
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

        setTimeout(() => {
            if (currentServerId) {
                subscribeServiceEvents(currentServerId);
            }
        }, delay);
    };
}

function startServicePolling(serverId) {
    stopServicePolling();

    let consecutiveErrors = 0;
    const MAX_CONSECUTIVE_ERRORS = 3;

    servicePollingInterval = setInterval(async () => {
        // НЕ создавать полинг, если страница скрыта
        if (document.hidden) {
            return;
        }

        // Проверка контекста
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
                console.warn('Слишком много ошибок полинга. Остановка полинга.');
                stopServicePolling();
            }
        }
    }, CONFIG.POLLING_INTERVAL);
}

function stopServicePolling() {
    if (servicePollingInterval) {
        clearInterval(servicePollingInterval);
        servicePollingInterval = null;
    }
}

function updateServicesStatus(statuses) {
    // Проверка валидности
    if (!isPageVisible) {
        return;
    }

    if (!Array.isArray(statuses) || statuses.length === 0) {
        return;
    }

    // Обновляем ТОЛЬКО статусы в уже загруженных данных
    // НЕ добавляем новые!
    const statusMap = new Map(statuses.map(s => [s.id, s]));

    // Обновление в памяти
    allServices.forEach(service => {
        const updatedStatus = statusMap.get(service.id);
        if (updatedStatus) {
            service.status = updatedStatus.status;
            service.updatedat = updatedStatus.updatedat;
        }
    });

    // Обновление в DOM только видимых элементов
    document.querySelectorAll('.service-card').forEach(card => {
        const serviceId = parseInt(card.getAttribute('data-service-id'));
        const status = statusMap.get(serviceId);

        if (!status) return;

        const statusElement = card.querySelector('.service-status');
        const updatedElement = card.querySelector('.service-updated');

        if (statusElement && statusElement.textContent !== status.status) {
            statusElement.textContent = status.status;
        }

        if (updatedElement && status.updatedat) {
            const newDate = new Date(status.updatedat).toLocaleString('ru-RU');
            if (updatedElement.textContent !== newDate) {
                updatedElement.textContent = newDate;
            }
        }
    });
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

        // Полностью очищаем состояние при новом логине
        localStorage.removeItem('swsm_current_server_id');
        currentServerId = null;
        currentServerData = null;
        allServices = [];
        currentPage = 1;

        window._sessionExpiredNotified = false;

        showToast('Успех', 'Авторизация прошла успешно!');
        showMainApp();

        // Загружаем список серверов (не детали сервера)
        setTimeout(() => {
            showServersList();  // ← Вместо loadServersList()
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

function handleLogout() {
    stopServicePolling();

    localStorage.removeItem('swsm_user');
    localStorage.removeItem('swsm_current_server_id');
    currentUser = null;
    currentServerId = null;
    currentServerData = null;
    allServices = [];
    currentPage = 1;
    allServers = [];
    serversCurrentPage = 1;
    window._sessionExpiredNotified = false;

    if (serviceEventsSource) {
        serviceEventsSource.close();
        serviceEventsSource = null;
    }

    showLoginPage();
    showToast('Информация', 'Вы вышли из системы');
}

// ============================================
// SERVER MANAGEMENT
// ============================================

async function loadServersList() {
    showLoading();

    try {
        const servers = await apiRequest('/user/servers');
        allServers = servers || [];
        serversCurrentPage = 1;
        renderServersCurrentPage();
    } catch (error) {
        if (!(error instanceof SessionExpiredError)) {
            showToast('Ошибка', 'Не удалось загрузить список серверов', 'error');
        }
    } finally {
        hideLoading();
    }
}

function renderServersList(servers) {
    serversList.innerHTML = '';

    if (!servers || servers.length === 0) {
        serversList.innerHTML = `
            <div class="col-12">
                <div class="alert alert-info text-center">
                    <i class="bi bi-info-circle me-2"></i>
                    Серверы не добавлены. Нажмите "Добавить сервер" для начала работы.
                </div>
            </div>
        `;
        return;
    }

    servers.forEach(server => {
        const serverCard = document.createElement('div');
        serverCard.className = 'col-md-6 col-lg-4';
        serverCard.innerHTML = `
            <div class="card h-100 shadow-sm service-card">
                <div class="card-body">
                    <h5 class="card-title">
                        <i class="bi bi-server me-2"></i>${server.name}
                    </h5>
                    <p class="card-text">
                        <small class="text-muted">
                            <i class="bi bi-geo-alt me-1"></i>${server.address}
                        </small><br>
                        <small class="text-muted">
                            <i class="bi bi-person me-1"></i>${server.username}
                        </small><br>
                        <small class="text-muted">
                            <i class="bi bi-calendar me-1"></i>${new Date(server.created_at).toLocaleDateString('ru-RU')}
                        </small><br>
                        <small class="text-muted">
                            <i class="bi bi-fingerprint me-1"></i>${server.fingerprint}
                        </small>
                    </p>
                </div>
                <div class="card-footer">
                    <button class="btn btn-primary btn-sm w-100" onclick="showServerDetail(${server.id})">
                        <i class="bi bi-list-task me-1"></i>Управление
                    </button>
                </div>
            </div>
        `;
        serversList.appendChild(serverCard);
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

async function loadServerDetail(serverId) {
    currentServerId = serverId;
    showLoading();

    try {
        const server = await apiRequest(`/user/servers/${serverId}`);
        currentServerData = server;
        renderServerDetail(server);
        await loadServicesList(serverId);

        const editBtn = document.getElementById('editServerBtn');
        if (editBtn) {
            editBtn.onclick = openEditServerModalFromDetail;
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
    document.getElementById('serverBreadcrumb').textContent = server.name;
    document.getElementById('serverDetailName').textContent = server.name;
    document.getElementById('serverDetailAddress').textContent = server.address;
    document.getElementById('serverDetailUsername').textContent = server.username;
    document.getElementById('serverDetailCreated').textContent = new Date(server.created_at).toLocaleDateString('ru-RU');
    document.getElementById('serverDetailFingerprint').textContent = server.fingerprint;
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
        allServices = services || [];
        currentPage = 1;
        renderCurrentPage();

    } catch (error) {
        if (!silent && !(error instanceof SessionExpiredError)) {
            showToast('Ошибка', 'Не удалось загрузить список служб', 'error');
        }
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
            showToast('Ошибка', `Не удалось выполнить операцию "${action}" для службы "${serviceName}"`, 'error');
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

        allServices = data || [];
        currentPage = 1;
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
        serversTotalPages = 1;
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
// EVENT LISTENERS
// ============================================

function setupEventListeners() {
    document.getElementById('loginForm').addEventListener('submit', handleLogin);
    document.getElementById('registerForm').addEventListener('submit', handleRegister);
    document.getElementById('addServerForm').addEventListener('submit', handleAddServer);
    document.getElementById('editServerForm').addEventListener('submit', handleEditServer);
    document.getElementById('addServiceForm').addEventListener('submit', handleAddService);
    document.getElementById('refreshFromServerBtn').addEventListener('click', handleRefreshFromServer);
    document.getElementById('logoutBtn').addEventListener('click', handleLogout);
    document.getElementById('deleteServerBtn').addEventListener('click', handleDeleteServer);

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
                renderCurrentPage();
            }
        });
    }
    if (nextPageBtn) {
        nextPageBtn.addEventListener('click', () => {
            if (currentPage < totalPages) {
                currentPage++;
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
                renderServersCurrentPage();
            }
        });
    }
    if (serversNextPageBtn) {
        serversNextPageBtn.addEventListener('click', () => {
            if (serversCurrentPage < serversTotalPages) {
                serversCurrentPage++;
                renderServersCurrentPage();
            }
        });
    }
}

// ==========================================
// PAGE VISIBILITY OPTIMIZATION
// ==========================================

let isPageVisible = true;

document.addEventListener('visibilitychange', () => {
    isPageVisible = !document.hidden;

    if (document.hidden) {
        handlePageBackground();
    } else {
        handlePageResume();
    }
}, false);

function handlePageBackground() {
    // ПОЛНОСТЬЮ закрытие всех соединений

    // 1. Закрытие SSE
    if (serviceEventsSource) {
        serviceEventsSource.close();
        serviceEventsSource = null;
        sseConnectionStatus = 'closed';
        sseReconnectAttempts = 0;
    }

    // 2. Остановка полинга
    stopServicePolling();

    // 3. Очистка больших данных из памяти
    allServices = [];
    currentPage = 1;
    totalPages = 1;
}

function handlePageResume() {
    // Восстановление соединений ТОЛЬКО если нужно

    if (!currentUser) {
        return;
    }

    if (!currentServerId) {
        return;
    }

    // ВАЖНО: Полное перезагрузка данных с сервера
    loadServicesList(currentServerId, false);

    // Переподписка на обновления
    subscribeServiceEvents(currentServerId);
}

// ============================================
// SHOW/HIDE FUNCTIONS
// ============================================

function showLoginPage() {
    loginPage.classList.remove('hidden');
    mainApp.classList.add('hidden');
    localStorage.removeItem('swsm_current_server_id');
    currentServerId = null;
    window._sessionExpiredNotified = false;

    if (serviceEventsSource) {
        serviceEventsSource.close();
        serviceEventsSource = null;
    }
    stopServicePolling();
}

function showMainApp() {
    loginPage.classList.add('hidden');
    mainApp.classList.remove('hidden');
    if (currentUser) {
        currentUserSpan.textContent = currentUser;
    }
}

function showServersList() {
    serversListView.classList.remove('hidden');
    serverDetailView.classList.add('hidden');
    localStorage.removeItem('swsm_current_server_id');

    // ПОЛНАЯ ОЧИСТКА
    currentServerId = null;
    currentServerData = null;
    allServices = [];
    currentPage = 1;
    totalPages = 1;
    sseReconnectAttempts = 0;

    // Закрытие SSE
    if (serviceEventsSource) {
        serviceEventsSource.close();
        serviceEventsSource = null;
        sseConnectionStatus = 'closed';
    }

    // Остановка полинга
    stopServicePolling();

    // Загрузка списка серверов
    loadServersList();
}

function showServerDetail(serverId) {
    serversListView.classList.add('hidden');
    serverDetailView.classList.remove('hidden');

    currentServerId = serverId;
    localStorage.setItem('swsm_current_server_id', serverId);

    loadServerDetail(serverId);
    subscribeServiceEvents(serverId);
}

function showLoading() {
    loadingSpinner.style.display = 'block';
}

function hideLoading() {
    loadingSpinner.style.display = 'none';
}
