// API Configuration
// const API_BASE = 'http://127.0.0.1:8080/api';
// const API_BASE = `${window.location.protocol}//${window.location.hostname}:8080/api`;
// const API_BASE = "${API_BASE_URL}";

async function fetchServices() {
    const response = await fetch(`${API_BASE}/services`);
}

let currentUser = localStorage.getItem('swsm_user');
let currentServerId = localStorage.getItem('swsm_current_server_id'); // Сохраняем ID сервера
let currentServerData = null;

// Pagination state for servers
let allServers = [];
let serversCurrentPage = 1;
let serversTotalPages = 1;

// Pagination state for services
let allServices = [];
let currentPage = 1;
let totalPages = 1;

// DOM Elements
const loginPage = document.getElementById('loginPage');
const mainApp = document.getElementById('mainApp');
const loadingSpinner = document.querySelector('.loading-spinner');
const currentUserSpan = document.getElementById('currentUser');
const serversListView = document.getElementById('serversListView');
const serverDetailView = document.getElementById('serverDetailView');
const serversList = document.getElementById('serversList');
const servicesList = document.getElementById('servicesList');

// SSE EventSource
let serviceEventsSource = null; // глобальная переменная для подписки на обновления служб

// Initialize App
document.addEventListener('DOMContentLoaded', function() {
    console.log('App initialized');
    if (currentUser) {
        console.log('User found in localStorage:', currentUser);
        showMainApp();

        if (currentServerId) {
            console.log('Restoring server detail view for server ID:', currentServerId);
            currentServerId = parseInt(currentServerId);
            showServerDetail(currentServerId);
        } else {
            loadServersList();
        }
    } else {
        console.log('No user found, showing login page');
        showLoginPage();
    }

    setupEventListeners();


    document.querySelectorAll('.modal').forEach(modalEl =>
        modalEl.addEventListener('hidden.bs.modal', () => {
            // Удаляем тени
            document.querySelectorAll('.modal-backdrop').forEach(b => b.remove());
            // Сбрасываем класс, блокирующий прокрутку
            document.body.classList.remove('modal-open');
            // Убираем стили overflow, если они были выставлены
            document.body.style.overflow = '';
        })
    );

    // Pagination handlers
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

    // Pagination handlers for servers
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

});

// Pagination functions
function getPageSize() {
    return window.innerWidth < 768 ? 6 : 9;
}

// === Заменить существующую функцию renderCurrentPage ===
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

    const minItems = getMinItemsForPagination('services'); // порог (5 на мобиле, 9 на десктопе)
    const pageIndicator = document.getElementById('pageIndicator');
    const prevPageBtn = document.getElementById('prevPageBtn');
    const nextPageBtn = document.getElementById('nextPageBtn');

    // Показываем пагинацию только если число служб строго больше порога
    const shouldPaginate = allServices.length > minItems;

    if (!shouldPaginate) {
        // Показываем весь список без кнопок пагинации
        renderServicesList(allServices);

        if (pageIndicator) pageIndicator.style.display = 'none';
        if (prevPageBtn) prevPageBtn.style.display = 'none';
        if (nextPageBtn) nextPageBtn.style.display = 'none';

        // Сбрасываем состояние пагинации
        currentPage = 1;
        totalPages = 1;
        return;
    }

    // Иначе — обычная пагинация
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

// === Заменить существующую функцию renderServersCurrentPage ===
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

    const minItems = getMinItemsForPagination('servers'); // порог (3 на мобиле, 9 на десктопе)
    const pageIndicator = document.getElementById('serversPageIndicator');
    const prevPageBtn = document.getElementById('serversPrevPageBtn');
    const nextPageBtn = document.getElementById('serversNextPageBtn');

    // Показываем пагинацию только если число серверов строго больше порога
    const shouldPaginate = allServers.length > minItems;

    if (!shouldPaginate) {
        // Показываем весь список серверов без пагинации
        renderServersList(allServers);

        if (pageIndicator) pageIndicator.style.display = 'none';
        if (prevPageBtn) prevPageBtn.style.display = 'none';
        if (nextPageBtn) nextPageBtn.style.display = 'none';
        if (paginationControls) paginationControls.style.display = 'none';

        // Сброс состояния пагинации
        serversCurrentPage = 1;
        serversTotalPages = 1;
        return;
    }

    // Иначе — обычная пагинация
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

    // Показываем пагинацию только если нужно
    if (paginationControls) {
        paginationControls.style.display = shouldPaginate ? 'flex' : 'none';
    }
}

// Event Listeners
function setupEventListeners() {
    document.getElementById('loginForm').addEventListener('submit', handleLogin);
    document.getElementById('registerForm').addEventListener('submit', handleRegister);
    document.getElementById('addServerForm').addEventListener('submit', handleAddServer);
    document.getElementById('editServerForm').addEventListener('submit', handleEditServer);
    document.getElementById('addServiceForm').addEventListener('submit', handleAddService);
    document.getElementById('refreshFromServerBtn').addEventListener('click', handleRefreshFromServer);
    document.getElementById('logoutBtn').addEventListener('click', handleLogout);
    document.getElementById('deleteServerBtn').addEventListener('click', handleDeleteServer);
}

// Show/Hide Pages
function showLoginPage() {
    loginPage.classList.remove('hidden');
    mainApp.classList.add('hidden');
    localStorage.removeItem('swsm_current_server_id');
    currentServerId = null;

    // Закрываем SSE, если был открыт
    if (serviceEventsSource) {
        serviceEventsSource.close();
        serviceEventsSource = null;
    }
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
    currentServerId = null;
    currentServerData = null;

    // Закрываем SSE
    if (serviceEventsSource) {
        serviceEventsSource.close();
        serviceEventsSource = null;
        allServices = [];
        currentPage = 1;
    }

    loadServersList();
}

function showServerDetail(serverId) {
    serversListView.classList.add('hidden');
    serverDetailView.classList.remove('hidden');

    currentServerId = serverId;
    localStorage.setItem('swsm_current_server_id', serverId);

    loadServerDetail(serverId);

    // Подписка на SSE обновления служб
    subscribeServiceEvents(serverId);
}

// Loading Spinner
function showLoading() {
    loadingSpinner.style.display = 'block';
}

function hideLoading() {
    loadingSpinner.style.display = 'none';
}

// Toast Notifications
function showToast(title, message, type = 'success') {
    const toastContainer = document.querySelector('.toast-container');
    const toastTemplate = document.getElementById('toastTemplate');
    const toast = toastTemplate.cloneNode(true);

    toast.id = 'toast-' + Date.now();
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

    const bsToast = new bootstrap.Toast(toast);
    bsToast.show();

    toast.addEventListener('hidden.bs.toast', () => {
        toast.remove();
    });
}

// ----------------------
// SSE: подписка на обновления статусов служб
// ----------------------

function subscribeServiceEvents(serverId) {
    // Закрываем старую подписку, если была
    if (serviceEventsSource) {
        serviceEventsSource.close();
    }

    const url = `${API_BASE}/user/broadcasting`;
    serviceEventsSource = new EventSource(url, { withCredentials: true });

    serviceEventsSource.onmessage = function(event) {
        try {
            const data = JSON.parse(event.data);
            console.log('SSE received:', data);

            // Фильтруем только службы текущего сервера
            const filtered = data.filter(s => s.server_id === serverId);
            if (filtered.length > 0) {
                updateServicesStatus(filtered);
            }
        } catch (err) {
            console.error('Error parsing SSE data:', err);
        }
    };

    serviceEventsSource.onerror = function(err) {
        console.error('SSE error:', err);
        // Переподключение через 5 секунд при ошибке
        setTimeout(() => {
            if (currentServerId) {
                subscribeServiceEvents(currentServerId);
            }
        }, 5000);
    };

    console.log(`Subscribed to SSE for user: ${currentUser}`);
}

// Обновление UI при приходе новых данных
function updateServicesStatus(statuses) {
    statuses.forEach(status => {
        const card = document.querySelector(`[data-service-id="${status.id}"]`);
        if (!card) return;

        const statusElement = card.querySelector('.service-status');
        const updatedElement = card.querySelector('.service-updated');

        if (statusElement) statusElement.textContent = status.status || '—';
        if (updatedElement) updatedElement.textContent = status.updated_at
            ? new Date(status.updated_at).toLocaleString('ru-RU')
            : '—';
    });
}

// ----------------------
// Авторизация, регистрация, CRUD серверов и служб
// ----------------------

// API Helper Function

// Функция для запросов без кеширования (только для критических операций)
async function apiRequestNoCache(endpoint, options = {}) {
    const url = `${API_BASE}${endpoint}`;
    console.log('Making non-cached API request:', url);

    const config = {
        method: options.method || 'GET',
        headers: {
            'Content-Type': 'application/json',
            ...options.headers
        },
        cache: 'no-cache', // Принудительная проверка с сервером
        credentials: 'include',
        ...options
    };

    try {
        console.log('Fetch config (no-cache):', config);
        const response = await fetch(url, config);
        console.log('Response:', response.status, response.statusText);

        // Проверяем на 401 Unauthorized
        if (response.status === 401) {
            console.log('Token expired, redirecting to login');

            // Очищаем все данные пользователя
            localStorage.removeItem('swsm_user');
            localStorage.removeItem('swsm_current_server_id');
            currentUser = null;
            currentServerId = null;
            currentServerData = null;
            allServices = [];
            currentPage = 1;
            allServers = [];
            serversCurrentPage = 1;

            // Закрываем SSE соединение
            if (serviceEventsSource) {
                serviceEventsSource.close();
                serviceEventsSource = null;
            }

            showToast('Сессия истекла', 'Ваша сессия истекла. Войдите в систему снова.', 'warning');
            showLoginPage();

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

        console.log('Response data:', data);

        if (!response.ok) {
            throw new Error(data.message || data.error || `HTTP error! status: ${response.status}`);
        }

        return data;
    } catch (error) {
        console.error('API Request failed:', error);

        if (error instanceof SessionExpiredError) {
            throw error;
        }

        if (error instanceof TypeError && error.message.includes('fetch')) {
            showToast('Ошибка', 'Не удается подключиться к серверу. Убедитесь что API запущен', 'error');
        }
        throw error;
    }
}


async function apiRequest(endpoint, options = {}) {
    const url = `${API_BASE}${endpoint}`;
    console.log('Making API request:', url);

    const config = {
        method: options.method || 'GET',
        headers: {
            'Content-Type': 'application/json',
            ...options.headers
        },
        credentials: 'include',
        ...options
    };

    try {
        console.log('Fetch config:', config);
        const response = await fetch(url, config);
        console.log('Response:', response.status, response.statusText);

        // Проверяем на 401 Unauthorized
        if (response.status === 401) {
            console.log('Token expired, redirecting to login');

            // Очищаем все данные пользователя
            localStorage.removeItem('swsm_user');
            localStorage.removeItem('swsm_current_server_id');
            currentUser = null;
            currentServerId = null;
            currentServerData = null;
            allServices = [];
            currentPage = 1;
            allServers = [];
            serversCurrentPage = 1;

            // Закрываем SSE соединение
            if (serviceEventsSource) {
                serviceEventsSource.close();
                serviceEventsSource = null;
            }

            // Показываем сообщение и редиректим
            showToast('Сессия истекла', 'Ваша сессия истекла. Войдите в систему снова.', 'warning');
            showLoginPage();

            // Возвращаем ошибку для прерывания дальнейшего выполнения
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

        console.log('Response data:', data);

        if (!response.ok) {
            throw new Error(data.message || data.error || `HTTP error! status: ${response.status}`);
        }

        return data;
    } catch (error) {
        console.error('API Request failed:', error);

        // Не показываем общую ошибку для истечения сессии
        if (error instanceof SessionExpiredError) {
            throw error;
        }

        if (error instanceof TypeError && error.message.includes('fetch')) {
            showToast('Ошибка', 'Не удается подключиться к серверу. Убедитесь что API запущен', 'error');
        }
        throw error;
    }
}

// Создаем специальный класс ошибки для истечения сессии
class SessionExpiredError extends Error {
    constructor(message) {
        super(message);
        this.name = 'SessionExpiredError';
    }
}

// Authentication Handlers
async function handleLogin(event) {
    event.preventDefault();
    console.log('Login attempt started');

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

        console.log('Login response:', response);

        // Сохраняем только имя пользователя, аутентификация через cookie
        currentUser = response.login || response.Login;
        localStorage.setItem('swsm_user', currentUser);

        showToast('Успех', 'Авторизация прошла успешно!');
        showMainApp();
        loadServersList(); // Загружаем серверы через API

    } catch (error) {
        console.error('Login error:', error);
        showToast('Ошибка', error.message, 'error');
    } finally {
        hideLoading();
    }
}

async function handleRegister(event) {
    event.preventDefault();
    console.log('Registration attempt started');

    const username = document.getElementById('registerUsername').value;
    const password = document.getElementById('registerPassword').value;

    showLoading();

    try {
        const response = await apiRequest('/user/register', {
            method: 'POST',
            body: JSON.stringify({
                login: username,
                password: password
            })
        });

        console.log('Registration response:', response);

        const modal = bootstrap.Modal.getInstance(document.getElementById('registerModal'));
        modal.hide();
        document.getElementById('registerForm').reset();

        showToast('Успех', 'Регистрация прошла успешно! Теперь войдите в систему.');

    } catch (error) {
        console.error('Registration error:', error);
        showToast('Ошибка', error.message, 'error');
    } finally {
        hideLoading();
    }
}

function handleLogout() {
    console.log('Logout initiated');

    // Очищаем все данные пользователя
    localStorage.removeItem('swsm_user');
    localStorage.removeItem('swsm_current_server_id');
    currentUser = null;
    currentServerId = null;
    currentServerData = null;
    allServices = [];
    currentPage = 1;
    allServers = [];
    serversCurrentPage = 1;

    showLoginPage();
    showToast('Информация', 'Вы вышли из системы');
}

// Server Management - используем API GetServersList
async function loadServersList() {
    console.log('Loading servers list via API...');
    showLoading();

    try {
        // Вызываем GetServerList хэндлер
        const servers = await apiRequest('/user/servers');
        console.log('Loaded servers from API:', servers);
        // renderServersList(servers);
        allServers = servers || [];
        serversCurrentPage = 1;
        renderServersCurrentPage();


    } catch (error) {
        console.error('Error loading servers:', error);
        showToast('Ошибка', 'Не удалось загрузить список серверов', 'error');
    } finally {
        hideLoading();
    }
}

function renderServersList(servers) {
    console.log('Rendering servers list:', servers);
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
    console.log('Add server attempt started');

    const name = document.getElementById('serverName').value;
    const address = document.getElementById('serverAddress').value;
    const username = document.getElementById('serverUsername').value;
    const password = document.getElementById('serverPassword').value;

    console.log('Server data:', { name, address, username, password: '***' });

    showLoading();

    try {
        const response = await apiRequest('/user/servers', {
            method: 'POST',
            body: JSON.stringify({
                name: name,
                address: address,
                username: username,
                password: password
            })
        });

        console.log('Add server response:', response);

        const modal = bootstrap.Modal.getInstance(document.getElementById('addServerModal'));
        modal.hide();
        document.getElementById('addServerForm').reset();

        showToast('Успех', `Сервер "${name}" успешно добавлен!`);

        // Перезагружаем список серверов через API
        loadServersList();

    } catch (error) {
        console.error('Add server error:', error);
        showToast('Ошибка', error.message, 'error');
    } finally {
        hideLoading();
    }
}

// Server Detail Management - используем API GetServer
async function loadServerDetail(serverId) {
    console.log('Loading server detail for ID:', serverId);
    currentServerId = serverId;
    showLoading();

    try {
        // Вызываем GetServer хэндлер
        const server = await apiRequest(`/user/servers/${serverId}`);
        console.log('Loaded server detail:', server);

        currentServerData = server;
        renderServerDetail(server);

        // Загружаем список служб
        await loadServicesList(serverId);

        // Добавляем обработчик для кнопки редактирования
        const editBtn = document.getElementById('editServerBtn');
        if (editBtn) {
            editBtn.onclick = openEditServerModalFromDetail;
        }

    } catch (error) {
        console.error('Error loading server detail:', error);
        showToast('Ошибка', 'Не удалось загрузить информацию о сервере', 'error');
        // При ошибке загрузки сервера возвращаемся к списку серверов
        showServersList();
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

// Server Delete - используем API DelServer
async function handleDeleteServer() {
    if (!currentServerId || !currentServerData) return;

    if (!confirm(`Вы уверены, что хотите удалить сервер "${currentServerData.name}"?`)) {
        return;
    }

    console.log('Deleting server ID:', currentServerId);
    showLoading();

    try {
        // Вызываем DelServer хэндлер
        await apiRequest(`/user/servers/${currentServerId}`, {
            method: 'DELETE'
        });

        console.log('Server deleted successfully');
        showToast('Успех', `Сервер "${currentServerData.name}" удален`);

        // Возвращаемся к списку серверов
        showServersList();

    } catch (error) {
        console.error('Delete server error:', error);
        showToast('Ошибка', error.message, 'error');
    } finally {
        hideLoading();
    }
}

async function loadServicesList(serverId, silent = false) {
    console.log('Loading services list for server ID:', serverId);
    if (!silent) showLoading();

    try {
        // Добавляем timestamp для cache busting
        const cacheBuster = Date.now();
        const services = await apiRequest(`/user/servers/${serverId}/services?_t=${cacheBuster}`);
        console.log('Loaded services:', services);

        allServices = services || [];
        currentPage = 1;
        renderCurrentPage();

    } catch (error) {
        console.error('Error loading services:', error);
        if (!silent) {
            showToast('Ошибка', 'Не удалось загрузить список служб', 'error');
        }
    } finally {
        if (!silent) hideLoading();
    }
}

function renderServicesList(services) {
    console.log('Rendering services list', services);
    servicesList.innerHTML = '';

    // Получаем ссылку на кнопку
    const refreshBtn = document.getElementById('refreshFromServerBtn');

    if (!services || services.length === 0) {
        servicesList.innerHTML = `
            <div class="col-12">
                <div class="alert alert-warning text-center">
                    <i class="bi bi-exclamation-triangle me-2"></i>
                    Список служб пуст
                </div>
            </div>`;

        // Скрываем кнопку, если нет служб
        if (refreshBtn) {
            refreshBtn.style.display = 'none';
        }
        return;
    }

    // Рендерим службы с помощью template
    services.forEach(service => {
        const template = document.getElementById('serviceCardTemplate');
        const serviceCard = template.content.cloneNode(true);

        // Заполняем данные службы
        serviceCard.querySelector('.service-displayed-name').textContent = service.displayed_name;
        serviceCard.querySelector('.service-name').textContent = service.service_name;
        serviceCard.querySelector('.service-status').textContent = service.status || '—';
        serviceCard.querySelector('.service-updated').textContent = service.updated_at ?
            new Date(service.updated_at).toLocaleString('ru-RU') : '—';

        // Устанавливаем data-service-id
        serviceCard.querySelector('.service-card').setAttribute('data-service-id', service.id);

        // Привязываем обработчики событий
        serviceCard.querySelector('.service-start-btn').addEventListener('click', () =>
            controlService(service.id, 'start', service.displayed_name));
        serviceCard.querySelector('.service-stop-btn').addEventListener('click', () =>
            controlService(service.id, 'stop', service.displayed_name));
        serviceCard.querySelector('.service-restart-btn').addEventListener('click', () =>
            controlService(service.id, 'restart', service.displayed_name));
        serviceCard.querySelector('.service-delete-btn').addEventListener('click', () =>
            handleDeleteService(service.id, service.displayed_name));

        servicesList.appendChild(serviceCard);
    });


    // Показываем кнопку, если есть службы
    if (refreshBtn) {
        refreshBtn.style.display = 'inline-block';
    }
}

// Service Addition - используем API AddService
async function handleAddService(event) {
    event.preventDefault();
    console.log('Add service attempt started');

    if (!currentServerId) {
        showToast('Ошибка', 'Сначала выберите сервер', 'error');
        return;
    }

    const displayedName = document.getElementById('serviceDisplayedName').value;
    const serviceName = document.getElementById('serviceName').value;

    console.log('Service data:', { displayed_name: displayedName, service_name: serviceName });

    showLoading();

    try {
        // Вызываем AddService хэндлер
        const response = await apiRequest(`/user/servers/${currentServerId}/services`, {
            method: 'POST',
            body: JSON.stringify({
                displayed_name: displayedName,
                service_name: serviceName
            })
        });

        console.log('Add service response:', response);

        const modal = bootstrap.Modal.getInstance(document.getElementById('addServiceModal'));
        modal.hide();
        document.getElementById('addServiceForm').reset();

        showToast('Успех', `Служба "${response.displayed_name}" успешно добавлена!`);

        // Перезагружаем список служб
        loadServicesList(currentServerId);

    } catch (error) {
        console.error('Add service error:', error);
        showToast('Ошибка', error.message, 'error');
    } finally {
        hideLoading();
    }
}

// Service Control - используем API ServiceStart/ServiceStop/ServiceRestart
async function controlService(serviceId, action, serviceName) {
    console.log('Controlling service:', serviceId, action, serviceName);
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

        const response = await apiRequest(endpoint, {
            method: 'POST'
        });

        console.log('Service control response:', response);

        // Получаем обновленную информацию о службе
        const updatedService = await apiRequest(`/user/servers/${currentServerId}/services/${serviceId}`);

        // Обновляем UI сразу
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

        let actionText;
        switch (action) {
            case 'start': actionText = 'запущена'; break;
            case 'stop': actionText = 'остановлена'; break;
            case 'restart': actionText = 'перезапущена'; break;
            default: actionText = 'обработана';
        }

        showToast('Успех', response.message || `Служба "${serviceName}" ${actionText}`);

    } catch (error) {
        console.error('Service control error:', error);
        showToast('Ошибка', `Не удалось выполнить операцию "${action}" для службы "${serviceName}"`, 'error');
    } finally {
        hideLoading();
    }
}

// Новая функция для загрузки служб с актуальными статусами и обработкой X-Is-Updated
async function loadServicesListWithActual(serverId, silent = false) {
    console.log('Loading services list with actual status for server ID:', serverId);
    if (!silent) showLoading();

    try {
        const url = `${API_BASE}/user/servers/${serverId}/services?actual=true`;
        console.log('Making API request:', url);

        const config = {
            method: 'GET',
            headers: {
                'Content-Type': 'application/json'
            },
            credentials: 'include'
        };

        const response = await fetch(url, config);
        console.log('Response:', response.status, response.statusText);

        // Проверяем заголовок X-Is-Updated
        const isUpdated = response.headers.get('X-Is-Updated');
        console.log('X-Is-Updated header:', isUpdated);

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

        console.log('Response data:', data);

        if (!response.ok) {
            throw new Error(data.message || data.error || `HTTP error! status: ${response.status}`);
        }

        // renderServicesList(data);
        allServices = data || [];
        currentPage = 1;
        renderCurrentPage();

        // Обрабатываем заголовок X-Is-Updated
        if (isUpdated === 'false') {
            showToast('Предупреждение', 'Проблемы со связью. Показаны данные из кэша. Попробуйте позже.', 'warning');
        } else {
            showToast('Успех', 'Статусы служб обновлены с сервера');
        }

    } catch (error) {
        console.error('Error loading services with actual status:', error);
        if (!silent) {
            showToast('Ошибка', 'Не удалось загрузить список служб с актуальными статусами', 'error');
        }
    } finally {
        if (!silent) hideLoading();
    }
}


async function handleRefreshFromServer() {
    if (!currentServerId) {
        showToast('Ошибка', 'Сначала выберите сервер', 'error');
        return;
    }

    // Используем новую функцию для загрузки с actual=true и обработкой X-Is-Updated
    await loadServicesListWithActual(currentServerId);
}


// Service Delete - используем API DelService
async function handleDeleteService(serviceId, serviceName) {
    if (!confirm(`Вы уверены, что хотите удалить службу "${serviceName}"?`)) {
        return;
    }

    console.log('Deleting service ID:', serviceId);
    showLoading();

    try {
        // Вызываем DelService хэндлер
        await apiRequest(`/user/servers/${currentServerId}/services/${serviceId}`, {
            method: 'DELETE'
        });

        console.log('Service deleted successfully');
        showToast('Успех', `Служба "${serviceName}" удалена`);

        // Перезагружаем список служб
        loadServicesList(currentServerId);

    } catch (error) {
        console.error('Delete service error:', error);
        showToast('Ошибка', error.message, 'error');
    } finally {
        hideLoading();
    }
}

// Функция для открытия модального окна редактирования из Server Detail View
function openEditServerModalFromDetail() {
    if (!currentServerId || !currentServerData) {
        showToast('Ошибка', 'Данные сервера не загружены', 'error');
        return;
    }

    try {
        // Заполняем форму данными текущего сервера
        document.getElementById('editServerId').value = currentServerData.id;
        document.getElementById('editServerName').value = currentServerData.name || '';
        document.getElementById('editServerAddress').value = currentServerData.address || '';
        document.getElementById('editServerUsername').value = currentServerData.username || '';
        document.getElementById('editServerPassword').value = ''; // Пароль не показываем

        // Открываем модальное окно
        const modal = new bootstrap.Modal(document.getElementById('editServerModal'));
        modal.show();

    } catch (error) {
        console.error('Error opening edit modal:', error);
        showToast('Ошибка', 'Не удалось открыть форму редактирования', 'error');
    }
}

// Обработчик формы редактирования сервера
async function handleEditServer(event) {
    event.preventDefault();
    console.log('Edit server attempt started');

    const serverId = document.getElementById('editServerId').value;
    const name = document.getElementById('editServerName').value;
    const address = document.getElementById('editServerAddress').value;
    const username = document.getElementById('editServerUsername').value;
    const password = document.getElementById('editServerPassword').value;

    showLoading();

    try {
        const serverData = {
            name: name,
            address: address,
            username: username
        };

        // Добавляем пароль только если он указан
        if (password.trim()) {
            serverData.password = password;
        }

        const response = await apiRequest(`/user/servers/${serverId}`, {
            method: 'PATCH',
            body: JSON.stringify(serverData)
        });

        console.log('Edit server response:', response);

        // Закрываем модальное окно
        const modal = bootstrap.Modal.getInstance(document.getElementById('editServerModal'));
        modal.hide();

        // Очищаем форму
        document.getElementById('editServerForm').reset();

        // Показываем успешное сообщение
        showToast('Успех', 'Сервер успешно обновлен!');

        // Перезагружаем данные сервера
        if (currentServerId === parseInt(serverId)) {
            loadServerDetail(serverId);
        }

    } catch (error) {
        console.error('Edit server error:', error);
        showToast('Ошибка', error.message || 'Не удалось обновить сервер', 'error');
    } finally {
        hideLoading();
    }
}

// Функция для получения минимального количества элементов для показа пагинации
function getMinItemsForPagination(type) {
    const isMobile = window.innerWidth < 768;
    if (type === 'services') {
        return isMobile ? 5 : 9;
    } else if (type === 'servers') {
        return isMobile ? 3 : 9;
    }
    return 0;
}