#!/bin/sh

# Entrypoint нужен для передачи во фронтенд переменной окружения API_BASE_URL.
# Генерируем config.js с переменной окружения API_BASE_URL
cat > /usr/share/nginx/html/config.js << EOF
const API_BASE = "${API_BASE_URL}";
EOF

echo "Generated config.js with API_BASE = ${API_BASE_URL}"

# Запускаем nginx
exec nginx -g 'daemon off;'
