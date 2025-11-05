// Config.js необходим для передачи во фронтенд переменной окружения API_BASE_URL при локальной разработке.
// const API_BASE = "http://127.0.0.1:8080/api";

const API_BASE = getAPIBase();

function getAPIBase() {
    const protocol = window.location.protocol;
    const hostname = window.location.hostname;

    if (hostname === 'localhost' || hostname === '127.0.0.1') {
        return `${protocol}//127.0.0.1:8080/api`;
    }
    return `${protocol}//${hostname}:8080/api`;
}