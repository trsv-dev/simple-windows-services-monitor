#!/bin/sh

# Entrypoint нужен для передачи во фронтенд переменных окружения API_BASE_URL и KEYCLOAK_CONFIG.
cat > /usr/share/nginx/html/config.js << EOF
const API_BASE = "${API_BASE_URL}";

const KEYCLOAK_CONFIG = {
    url: window.location.protocol + "//" + window.location.hostname + ":8081",
    realm: "${KEYCLOAK_REALM_NAME}",
    clientId: "${KEYCLOAK_CLIENT_ID}"
};
EOF

# Запускаем nginx
exec nginx -g 'daemon off;'
