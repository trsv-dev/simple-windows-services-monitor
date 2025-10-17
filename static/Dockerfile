FROM nginx:alpine

# Удаляем дефолтный конфиг nginx
RUN rm /etc/nginx/conf.d/default.conf

# Копируем свой конфиг nginx
COPY nginx.conf /etc/nginx/conf.d/

# Копируем папку со статикой
COPY . /usr/share/nginx/html

# Экспонируем порт
EXPOSE 80

# Запуск nginx в форграунд режиме
CMD ["nginx", "-g", "daemon off;"]