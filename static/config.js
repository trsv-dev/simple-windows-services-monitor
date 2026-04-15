// Config.js необходим для передачи во фронтенд переменных окружения API_BASE_URL и KEYCLOAK_CONFIG при локальной разработке.

// ============================================
// API
// ============================================

const API_BASE = getAPIBase();

function getAPIBase() {
    const protocol = window.location.protocol;
    const hostname = window.location.hostname;

    if (hostname === 'localhost' || hostname === '127.0.0.1') {
        return `${protocol}//127.0.0.1:8080/api`;
    }

    return `${protocol}//${hostname}:8080/api`;
}

// ============================================
// KEYCLOAK
// ============================================

const KEYCLOAK_CONFIG = getKeycloakConfig();

function getKeycloakConfig() {
    const protocol = window.location.protocol;
    const hostname = window.location.hostname;

    if (hostname === 'localhost' || hostname === '127.0.0.1') {
        return {
            url: `${protocol}//127.0.0.1:8081`,
            realm: 'swsm',
            clientId: 'swsm'
        };
    }

    return {
        url: `${protocol}//${hostname}:8081`,
        realm: 'swsm',
        clientId: 'swsm'
    };
}