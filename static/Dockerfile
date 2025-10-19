FROM nginx:alpine

# Удаляем дефолтный конфиг nginx
RUN rm /etc/nginx/conf.d/default.conf

# Копируем свой конфиг nginx
COPY nginx.conf /etc/nginx/conf.d/

# Копируем остальную статику (app.js, index.html, styles и т.д.)
COPY . /usr/share/nginx/html

# Копируем entrypoint скрипт
COPY docker-entrypoint.sh /docker-entrypoint.sh
RUN chmod +x /docker-entrypoint.sh

# Экспонируем порт
EXPOSE 80

# Используем entrypoint для передачи во фронтенд переменной окружения API_BASE_URL
ENTRYPOINT ["/docker-entrypoint.sh"]