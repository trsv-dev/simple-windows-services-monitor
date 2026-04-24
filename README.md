# simple-windows-services-monitor

**Simple Windows Services Monitor (SWSM)** — многопользовательский сервис для управления службами Windows на удалённых серверах. 
Управление возможно только в рамках доверенного контура, например, когда серверы связаны между собой посредством VPN, где SWSM имеет прямой доступ к управляемым серверам. 
С помощью сервиса можно запускать, останавливать, перезапускать службы и получать их текущий статус.

Для взаимодействия используется [WinRM](https://github.com/masterzen/winrm).

| [<img src="screenshots/screenshot_1.png" width="250"/>](screenshots/screenshot_1.png) | [<img src="screenshots/screenshot_2.png" width="250"/>](screenshots/screenshot_2.png) | [<img src="screenshots/screenshot_3.png" width="250"/>](screenshots/screenshot_2.png) |
|---------------------------------------------------------------------------------------|---------------------------------------------------------------------------------------|---------------------------------------------------------------------------------------|
| Список серверов                                                                       | Детали сервера                                                                       | Добавление службы                                                                     |


## Возможности

- 🔑 Многопользовательская аутентификация и управление пользователями через Keycloak.
- 🕹️ Управление службами Windows (start, stop, restart).
- 📡 Работа с удалёнными серверами по WinRM.
- 📦 Хранение данных в PostgreSQL.
- 📢 Синхронизация статусов служб между одинаковыми серверами пользователей
- 📻 Получение актуальных статусов служб с сервера
- 📜 Логирование действий
- 🗞️ Возможность публикации событий для использования во фронтенде
---

## Требования

- Go 1.24+
- PostgreSQL 16+
- Keycloak 26+
- FRP (Fast Reverse Proxy) 0.67.0+
- Windows Server с включённым WinRM

---

## Оглавление

- [Документация по API](#документация-по-API)
- [Настройка WinRM на удалённом сервере](#настройка-WinRM-на-удалённом-сервере)
- [Установка и запуск для разработки](#установка-и-запуск-для-разработки)
- [Подготовка к запуску на продакшене](#подготовка-к-запуску-на-продакшене)
- [Запуск на продакшене в docker-контейнерах](#запуск-на-продакшене-в-docker-контейнерах)
- [Создание пользователя на сервере Windows](#создание-пользователя-на-сервере-Windows)

---

## Документация по API
[Коллекция Postman](https://documenter.getpostman.com/view/26097853/2sB3HqHdhN)

## Настройка WinRM на удалённом сервере

На удалённом Windows-хосте необходимо включить и настроить WinRM.  
Откройте PowerShell **от имени администратора** и выполните команды:

```powershell
winrm quickconfig
y
winrm set winrm/config/service/Auth '@{Basic="true"}'
winrm set winrm/config/service '@{AllowUnencrypted="true"}'
winrm set winrm/config/winrs '@{MaxMemoryPerShellMB="1024"}'
```

---

## Установка и запуск для разработки

1. Клонируйте репозиторий:

   ```bash
   git clone https://github.com/trsv-dev/simple-windows-services-monitor.git
   cd simple-windows-services-monitor
   ```
2. Создайте БД в PostgreSQL:
    ```bash
   psql -h localhost -U postgres
   create database swsm;
   create user swsm with encrypted password 'userpassword';
   grant all privileges on database swsm to swsm;
   alter database swsm owner to swsm;

3. Разверните docker-контейнер с Keycloak Phase Two. Для локальной разработки можно использовать Keycloak Phase Two в dev-режиме.  
    Внешний порт выбирайте на своё усмотрение, по умолчанию на сайте Keycloak указан порт 8080:

     **Примечание**:  
     Внешний порт 8081 выбран произвольно, чтобы не конфликтовал с другими сервисами.
     Привязка к 127.0.0.1 ограничивает доступ только с локальной машины.
    
   ```bash
   docker run --name keycloak_events --rm -p 8081:8080 \
     --add-host=host.docker.internal:host-gateway \
     -e KEYCLOAK_ADMIN=admin \
     -e KEYCLOAK_ADMIN_PASSWORD=admin \
     quay.io/phasetwo/phasetwo-keycloak:26.5.0 \
     start-dev -- \
     --spi-events-listener-ext-event-http-enabled=true \
     --spi-email-template-provider=freemarker-plus-mustache \
     --spi-email-template-freemarker-plus-mustache-enabled=true
   ```

    После успешного развертывания контейнера потребуется создать постоянного пользователя с админскими правами,
    создать realm, создать клиента и настроить почту для отправки уведомлений. В примере ниже название realm-а и клиента обозначим как "swsm".

   - Перейдите по адресу http://127.0.0.1:8081, вам откроется панель управления Keycloak.
     Логин / пароль для временного администратора - admin / admin. 
     
   - После входа под временным администратором необходимо создать 
     постоянного пользователя с админскими правами: "Users" -> "Add user" -> Заполнить как минимум Username, 
     Email и поставить переключатель Email verified. Далее в созданном аккаунте перейти в "Role mapping" ->
     "Assign role" -> "Realm roles" -> "realm-admin" -> "Assign".
   
   - Зайдите в "Realm settings" -> "Themes" -> "Admin theme" и выберите тему "phasetwo.v2", без этой манипуляции ни в одном из 
     созданных realm'ов не появятся в дальнейшем нужные пункты меню ("Attributes")
   
   - Создайте realm, т.е "область" для проекта, где будут расположены его настройки и пользователи:
     "Manage realms" -> "Create realm" -> Укажите "Realm name" (swsm) -> "Create". 
     Создастся realm с именем "swsm". Перейдите в "Realm settings", укажите "Realm name" (swsm), "Display name" (swsm),
     "HTML Display name" (swsm), на вкладке "Login" включите переключатели "User registration", "Forgot password",
     "Remember me", "Login with email", "Verify email". На вкладке "Email" введите настройки своего почтового сервера или транспорта.
     На вкладке "Localization" при желании включите переключатель "Internationalization", добавьте необходимую 
     локализацию в "Supported locales" и выберите "Default locale".
   
   - **ВАЖНО!** В созданном realm (swsm) зайдите "Realm settings" -> "Themes" -> "Admin theme" и выберите тему "phasetwo.v2", перезагрузите страницу, на текущей странице
     станет доступна вкладка "Attributes", зайдите в ее, в поле "Key" выставите __providerConfig.ext-event-http.0_, 
     в поле "Value" выставите _{"targetUri":"http://host.docker.internal:8080/keycloak-events","retry":true,"backoffInitialInterval":500}_

   - **ВАЖНО!** Во вкладке "Events" -> "Event listeners" добавьте "ext-event-http" и нажмите "Save". В "User events settings" включите "Save events" и выберите как минимум "Register", "Register error", 
     "Delete account", "Delete account error".
     В "Admin events settings" включите "Save events" и "Include representation".

   - Создайте клиента, через который SWSM будет подключаться к Keycloak. Для этого перейдите в "Clients" -> "Create client"
     "Client type" оставьте без изменений (OpenID Connect), укажите "Client ID" (swsm), "Name" (swsm), "Description" (swsm) -> "Next" ->
     поставьте галочки "Authentication flow" и "Direct access grants" -> "Next" -> Введите "Root URL" (http://127.0.0.1:3000/),
     "Valid redirect URIs" (http://127.0.0.1:3000/*) и "Web origins" (http://127.0.0.1:3000). В данном случае порт 3000 - это порт фронтэнда.
     Нажмите кнопку "Save". Во вкладке "Client scopes" войдите в "swsm-dedicated" -> "Mappers" -> "Configure a new mapper" -> "Audience". 
     Введите "Name" и выберите в "Included Client Audience" название вашего realm-а.

4. Создайте в корне файл `.env.development` и заполните своими данными (пример дан в env_example):
    <details>
    <summary>Пример .env.development</summary>
    
    ```env
    # SWSM init vars
    ####################################################################################
    DATABASE_URI=postgres://swsm:userpassword@localhost:5432/swsm?sslmode=disable
    RUN_ADDRESS=127.0.0.1:8080
    # Используйте 5985 (http) или 5986 (https) порты
    WINRM_PORT=5985
    # Установите флаг в значение true для HTTPS-соединений с WinRM
    WINRM_USE_HTTPS=false
    # Установите флаг в значение true, чтобы пропустить проверку SSL (например, для самоподписанных сертификатов).
    WINRM_INSECURE_FOR_HTTPS=false
    # Уровень логгирования
    LOG_LEVEL=debug
    # Хранилище логов
    LOG_OUTPUT=./logs/swsm.log
    # Ключ для шифрования паролей. Требуется base64 ключ
    AES_KEY=enter_your-base64-key
    # Включен ли веб-интерфейс
    WEB_INTERFACE=true
    # Базовый URL бэкенда
    API_BASE_URL=http://localhost:8080/api
    
    # Postgres init vars
    ####################################################################################
    # тот же логин, что в URI
    POSTGRES_USER=swsm
    # тот же пароль, что в URI
    POSTGRES_PASSWORD=enter_your_userpassword
    # то же имя БД, что в URI
    POSTGRES_DB=swsm
    
    # Keycloak init vars (OpenID Connect)
    ####################################################################################
    # Название realm в Keycloak
    KEYCLOAK_REALM_NAME=swsm
    
    # Базовый URL Keycloak сервера.
    # Формат: http(s)://<host>:<port>
    # Пример: http://localhost:8081
    # Также можно по имени контейнера, например: http://keycloak:8081
    # или URL, где расположен ваш Keycloak, например: https://auth.example.com
    KEYCLOAK_ISSUER_URL=http://localhost:8081
    
    # НЕ ИСПОЛЬЗОВАТЬ В ПРОДАКШЕНЕ! Отключает проверку issuer.
    # Требуется для локальной разработки в контейнерах.
    KEYCLOAK_SKIP_ISSUER_CHECK=false
    
    # Идентификатор клиента (client_id), зарегистрированного в Keycloak.
    # Должен совпадать с именем клиента, указанным в консоли Keycloak.
    KEYCLOAK_CLIENT_ID=swsm
    ```
    </details>


4. Соберите бинарник и запустите сервер:
   ```bash
   cd ./cmd/swsm/
   go build -o "swsm"
   # В ОС Linux перед запуском бинарника необходимо выполнить команду ниже чтобы
   # разрешить работу с RAW-сокетами (например, для ICMP-проверок) без запуска от root.
   # В ОС Windows это не требуется
   sudo setcap cap_net_raw=+ep ./swsm
   ./swsm
   ```
   Бэкенд будет доступен по адресу, который вы указали в `RUN_ADDRESS`, например: http://127.0.0.1:8080/api


5. Если в `env` вы оставили `WEB_INTERFACE=true` (веб-интерфейс включен) то нужно запустить сервер статики. 
Для запуска сервера статики:
   ```bash
   cd ../../static
   go build -o static-server
   ./static-server -port=3000 -dir=./
   ```
6. Веб-интерфейс будет доступен по адресу: http://127.0.0.1:3000, API: http://127.0.0.1:8080/api,
   панель администрирования Keycloak: http://127.0.0.1:8081
---

## Подготовка к запуску на продакшене

Так как SWSM работает в доверенном контуре, то ip адрес у VPS, на котором он развернут будет "серым".
Для полноценного доступа к SWSM нам потребуется VPS с "белым" ip адресом, на котором будет запущен [Fast Reverse Proxy (FRP)](https://github.com/fatedier/frp).

FRP позволяет пробрасывать проекты, которые находятся внутри сети (на локальном сервере организации) и делать их доступными через Интернет.

1. **На VPS с "белым" ip**:
    - Запущен `frps` FRP Server.
2. **На VPS с "серым" ip в доверенном контуре**:
    - На локальном сервере запущен `frpc` (FRP Client).
    - Пробрасывает нужные порты для SWSM на VPS.

**Схема работы:**
```plain
[Браузер] → [FRP Server на VPS с "белым" ip] ⇄ [FRP Client на VPS с "серым" ip] → [SWSM]
```

**Краткое описание компонентов**:
- **FRP** (Fast Reverse Proxy) — инструмент для создания туннелей
- **frps** - серверная часть, работает на VPS с "белым" ip
- **frpc** - клиентская часть, работает в доверенном контуре, VPS с "серым" ip

**Подготовка сервера (VPS с "белым" ip)**

Установите FRP:
```bash
# Скачивание FRP и установка
wget https://github.com/fatedier/frp/releases/download/v0.67.0/frp_0.67.0_linux_amd64.tar.gz
tar -xzf frp_0.67.0_linux_amd64.tar.gz
sudo cp frp_0.67.0_linux_amd64/frps /usr/local/bin/
```

Создайте `/etc/frp/frps.toml`:
```toml
bindPort = 7000

# Токен для связи клиента FRP и сервера FRP
auth.method = "token"
auth.token = "P234sdfR2eH7jwKD1OiC46"

# Админка
webServer.addr = "0.0.0.0"
webServer.port = 7500
webServer.user = "admin"
webServer.password = "O1jdsfghsdgVQgbjpyWXZKIG"

log.level = "info"
log.maxDays = 3
```

**Подготовка клиента (VPS с "серым" ip)**

На клиенте все сервисы работают через Docker. Создадим конфигурацию и запустим FRP клиент в контейнере.

1. Создайте папку для конфигураций в корне домашней папки (например, `/home/user/frpc) и в ней файл `frpc.toml`:
```toml
serverAddr = "XXX.XXX.XXX.XXX"
serverPort = 7000

auth.method = "token"
auth.token = "P234sdfR2eH7jwKD1OiC46"

[[proxies]]
name = "nginx-http"
type = "tcp"
localIP = "127.0.0.1"
localPort = 80
remotePort = 80

[[proxies]]
name = "nginx-https"
type = "tcp"
localIP = "127.0.0.1"
localPort = 443
remotePort = 443

[log]
level = "info"
maxDays = 3
```

Создайте `docker-compose.yml` в той же папке:
```yaml
services:
  frpc:
    image: snowdreamtech/frpc:latest
    container_name: frpc
    restart: unless-stopped
    # используем сеть хоста
    network_mode: host
    volumes:
      - ./frpc.toml:/etc/frp/frpc.toml:ro
```

**Запуск FRP сервера**
```bash
sudo systemctl enable frps
sudo systemctl start frps
```

**Запуск FRP клиента через Docker Compose**
```bash
cd /home/user/frpc
sudo docker compose up -d
```

**Проверка статуса сервисов**
```bash
sudo systemctl status frps  # на сервере
sudo docker ps | grep frpc  # на клиенте
```

**Проверка доступности:**

```bash
curl -I https://your_domain.com
```

**Автозапуск:**

На сервере (**/etc/systemd/system/frps.service**):

```
[Unit]
Description=FRP Server
After=network.target

[Service]
Type=simple
ExecStart=/usr/local/bin/frps -c /etc/frp/frps.toml
Restart=on-failure
User=root
Group=root

[Install]
WantedBy=multi-user.target
```
На клиенте автозапуск обеспечивается через **restart: unless-stopped** в **docker-compose.yml**.

**Админка** будет доступна по адресу http://your_domain.com:7500, логин - admin, пароль - O1jdsfghsdgVQgbjpyWXZKIG

## Запуск на продакшене в docker-контейнерах

1. Клонируйте репозиторий:
   ```bash
   git clone https://github.com/trsv-dev/simple-windows-services-monitor.git
   cd simple-windows-services-monitor
   ```

2. Создайте в корне файл `.env.production` и заполните своими данными (пример дан в env_example):
    <details>
    <summary>Пример .env.production</summary>

   ```env
    # SWSM init vars
    ####################################################################################
    DATABASE_URI=postgres://swsm:userpassword@localhost:5432/swsm?sslmode=disable
    RUN_ADDRESS=127.0.0.1:8080
    # Используйте 5985 (http) или 5986 (https) порты
    WINRM_PORT=5985
    # Установите флаг в значение true для HTTPS-соединений с WinRM
    WINRM_USE_HTTPS=false
    # Установите флаг в значение true, чтобы пропустить проверку SSL (например, для самоподписанных сертификатов).
    WINRM_INSECURE_FOR_HTTPS=false
    # Уровень логгирования
    LOG_LEVEL=debug
    # Хранилище логов
    LOG_OUTPUT=./logs/swsm.log
    # Ключ для шифрования паролей. Требуется base64 ключ
    AES_KEY=enter_your-base64-key
    # Включен ли веб-интерфейс
    WEB_INTERFACE=true
    # Базовый URL бэкенда
    API_BASE_URL=/api
    
    # Backend postgres init vars (для образа backend postgres)
    ####################################################################################
    # тот же логин, что в URI
    POSTGRES_USER=swsm
    # тот же пароль, что в URI
    POSTGRES_PASSWORD=enter_your_userpassword
    # то же имя БД, что в URI
    POSTGRES_DB=swsm
    
    # Auth postgres init vars (для образа auth postgres)
    ####################################################################################
    # тот же логин, что в URI
    KEYCLOAK_POSTGRES_USER=swsm_keycloak
    # тот же пароль, что в URI
    KEYCLOAK_POSTGRES_PASSWORD=1234Rty7890-=
    # то же имя БД, что в URI
    KEYCLOAK_DB=swsm_keycloak
    
    # Keycloak init vars (OpenID Connect)
    ####################################################################################
    # Название realm в Keycloak
    KEYCLOAK_REALM_NAME=swsm
    
    # Базовый URL Keycloak сервера, включая realm.
    # Формат: http(s)://<host>:<port>
    # Пример: http://localhost:8081
    # Также можно по имени контейнера, например: http://keycloak:8081
    # или URL, где расположен ваш Keycloak, например: https://auth.example.com
    KEYCLOAK_ISSUER_URL=https://auth.example.com
    
    # НЕ ИСПОЛЬЗОВАТЬ В ПРОДАКШЕНЕ! Отключает проверку issuer.
    # Требуется для локальной разработки в контейнерах.
    KEYCLOAK_SKIP_ISSUER_CHECK=false
    
    # Идентификатор клиента (client_id), зарегистрированного в Keycloak.
    # Должен совпадать с именем клиента, указанным в консоли Keycloak.
    KEYCLOAK_CLIENT_ID=swsm
    
    # Ник администратора realm'а
    KEYCLOAK_ADMIN_USERNAME=admin
    # Пароль администратора realm'а
    KEYCLOAK_ADMIN_PASSWORD=enter_your_keycloak_admin_password
    ```
    </details>


3. Будем исходить из того, что у вас уже есть домен, например, example.com.
   Создаём два поддомена - auth.example.com (для Keycloak) и swsm.example.com (для SWSM).

4. Ниже показан пример файла конфигурации веб-сервера Nginx с уже полученными ssl-сертификатами (let's encrypt). Отредактируйте nginx.conf со своими значениями ip-адресов и доменов:
    <details>
    <summary>Пример nginx.conf</summary>

    ```nginx configuration
    # SWSM api and frontend
    server {
        server_name your.vds.ip.address swsm.example.com;
    
        # Frontend
        location / {
            proxy_set_header Host $http_host;
            proxy_set_header X-Real-IP $remote_addr;
            proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
            proxy_set_header X-Forwarded-Proto $scheme;
    
            proxy_pass http://127.0.0.1:8080;
        }
    
        # API
        location /api/ {
            proxy_set_header Host $http_host;
            proxy_set_header X-Real-IP $remote_addr;
            proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
            proxy_set_header X-Forwarded-Proto $scheme;
    
            proxy_pass http://127.0.0.1:10000;
            proxy_http_version 1.1;
            proxy_set_header Connection "";
            proxy_buffering off;
            proxy_read_timeout 3600s;
        }
    
        # Keycloak events
        location /keycloak-events {
            proxy_set_header Host $http_host;
            proxy_set_header X-Real-IP $remote_addr;
            proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
            proxy_set_header X-Forwarded-Proto $scheme;
            proxy_set_header Content-Type $http_content_type;

            proxy_pass http://127.0.0.1:10000;
            proxy_http_version 1.1;
            proxy_set_header Connection "";
            proxy_read_timeout 30s;
        }
   
        listen 443 ssl; # managed by Certbot
        ssl_certificate /etc/letsencrypt/live/swsm.example.com/fullchain.pem; # managed by Certbot
        ssl_certificate_key /etc/letsencrypt/live/swsm.example.com/privkey.pem; # managed by Certbot
        include /etc/letsencrypt/options-ssl-nginx.conf; # managed by Certbot
        ssl_dhparam /etc/letsencrypt/ssl-dhparams.pem; # managed by Certbot
    }
    
    # Keycloak
    server {
        server_name your.vds.ip.address auth.example.com;
    
        location ~* /(js|resources|theme)/ {
            proxy_pass http://127.0.0.1:8081;
            proxy_set_header Host $host;
            proxy_set_header X-Real-IP $remote_addr;
            proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
            proxy_set_header X-Forwarded-Proto $scheme;
            proxy_set_header X-Forwarded-Host $host;
        }
    
        location / {
            proxy_pass http://127.0.0.1:8081;
            proxy_set_header Host $host;
            proxy_set_header X-Real-IP $remote_addr;
            proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
            proxy_set_header X-Forwarded-Proto $scheme;
            proxy_set_header X-Forwarded-Host $host;
    
            proxy_buffer_size 128k;
            proxy_buffers 4 256k;
            proxy_busy_buffers_size 256k;
            proxy_read_timeout 300s;
        }
    
        listen 443 ssl; # managed by Certbot
        ssl_certificate /etc/letsencrypt/live/auth.example.comfullchain.pem; # managed by Certbot
        ssl_certificate_key /etc/letsencrypt/live/auth.example.com/privkey.pem; # managed by Certbot
        include /etc/letsencrypt/options-ssl-nginx.conf; # managed by Certbot
        ssl_dhparam /etc/letsencrypt/ssl-dhparams.pem; # managed by Certbot
    }
   
    server {
       if ($host = swsm.example.com) {
           return 301 https://$host$request_uri;
       } # managed by Certbot

       listen 80;
       server_name your.vds.ip.address swsm.example.com;
       return 404; # managed by Certbot
    }

    server {
       if ($host = auth.example.ru) {
           return 301 https://$host$request_uri;
       } # managed by Certbot

       server_name your.vds.ip.address auth.example.ru;
       listen 80;
       return 404; # managed by Certbot
    }
    ```
    </details>


5. Отредактируйте файл docker-compose.yml, подставив свои значения:
    <details>
    <summary>Пример docker-compose.yml</summary>
    
    ```yaml
    services:
      backend_db:
        container_name: postgres_backend
        image: postgres:16
        restart: unless-stopped
        env_file:
          - .env.production
        environment:
          POSTGRES_DB: ${POSTGRES_DB}
          POSTGRES_USER: ${POSTGRES_USER}
          POSTGRES_PASSWORD: ${POSTGRES_PASSWORD}
        volumes:
          - backend_db_data:/var/lib/postgresql/data
        healthcheck:
          test: ["CMD-SHELL", "pg_isready -U ${POSTGRES_USER} -d ${POSTGRES_DB} -h localhost"]
          interval: 5s
          timeout: 5s
          retries: 5
          start_period: 5s
    
      auth_db:
        container_name: postgres_auth
        image: postgres:16
        restart: unless-stopped
        env_file:
          - .env.production
        environment:
          POSTGRES_DB: ${KEYCLOAK_DB}
          POSTGRES_USER: ${KEYCLOAK_POSTGRES_USER}
          POSTGRES_PASSWORD: ${KEYCLOAK_POSTGRES_PASSWORD}
        volumes:
          - auth_db_data:/var/lib/postgresql/data
        healthcheck:
          test: ["CMD-SHELL", "pg_isready -U ${KEYCLOAK_POSTGRES_USER} -d ${KEYCLOAK_DB} -h localhost"]
          interval: 5s
          timeout: 5s
          retries: 5
          start_period: 5s
    
      backend:
        container_name: backend
        build:
          context: .
          dockerfile: Dockerfile
        restart: unless-stopped
        env_file:
          - .env.production
        depends_on:
          backend_db:
            condition: service_healthy
          auth:
            condition: service_healthy
        ports:
          - "10000:8080"
        volumes:
          - backend_logs:/app/logs
        # даёт контейнеру право создавать raw-сокеты, без которых ICMP (ping) работать не может.
        cap_add:
          - NET_RAW
        healthcheck:
          test: ["CMD", "curl", "-f", "http://localhost:8080/health"]
          interval: 5s
          timeout: 5s
          retries: 5
          start_period: 5s
    
      auth:
        container_name: auth
        image: quay.io/phasetwo/phasetwo-keycloak:26.5.0
        restart: unless-stopped
        ports:
          - "8081:8080"
        env_file:
          - .env.production
        environment:
          KC_HOSTNAME: auth.example.com
          KC_PROXY_HEADERS: xforwarded
          KC_HTTP_ENABLED: "true"
          KC_HOSTNAME_STRICT: "false"
          KC_HEALTH_ENABLED: "true"
          KC_DB: postgres
          KC_DB_URL: jdbc:postgresql://auth_db:5432/${KEYCLOAK_DB}
          KC_DB_USERNAME: ${KEYCLOAK_POSTGRES_USER}
          KC_DB_PASSWORD: ${KEYCLOAK_POSTGRES_PASSWORD}
          KC_BOOTSTRAP_ADMIN_USERNAME: ${KEYCLOAK_ADMIN_USERNAME}
          KC_BOOTSTRAP_ADMIN_PASSWORD: ${KEYCLOAK_ADMIN_PASSWORD}
        command: ["start", "--spi-events-listener-ext-event-http-enabled=true", 
                  "--spi-email-template-provider=freemarker-plus-mustache", 
                  "--spi-email-template-freemarker-plus-mustache-enabled=true"]
        depends_on:
          auth_db:
            condition: service_healthy
        volumes:
          - auth_data:/opt/keycloak/data
        healthcheck:
          test: ["CMD-SHELL", "exec 3<>/dev/tcp/localhost/8080 && echo 'OK' || exit 1"]
          interval: 10s
          timeout: 5s
          retries: 5
          start_period: 40s
    
      frontend:
        container_name: frontend
        env_file:
          - .env.production
        build:
          context: ./static
          dockerfile: Dockerfile
        restart: unless-stopped
        ports:
          - "8080:80"
        depends_on:
          backend:
            condition: service_healthy
        profiles:
          - frontend
    
    volumes:
      backend_db_data:
      auth_db_data:
      backend_logs:
      auth_data:
    ```
    </details>


6. Если вы хотите собрать проект с веб-интерфейсом (в `env` вы оставили `WEB_INTERFACE=true`), 
то из корня проекта (где расположен `docker-compose.yml`) выполните:
   ```bash
   docker compose --env-file .env.production --profile frontend up -d --build
   ```
   Если вы хотите использовать только API (в `env` вы оставили `WEB_INTERFACE=false`),
   то из корня проекта (где расположен `docker-compose.yml`) выполните:
   ```bash
   docker compose --env-file .env.production up -d --build
   ```
7. После первоначального успешного развертывания контейнеров не все контейнеры запустятся, т.к. контейнеру Keycloak потребуется дополнительная настройка. 
Нужно будет создать постоянного пользователя с админскими правами, создать realm, создать клиента и настроить почту для отправки уведомлений. 
В примере ниже название realm-а и клиента обозначим как "swsm".

    - Перейдите по адресу, на котором у вас развернут Keycloak. В нашем примере выше это https://auth.example.com, вам откроется панель управления Keycloak.
      Логин / пароль для временного администратора те же, что вы использовали в переменных окружения KEYCLOAK_ADMIN_USERNAME и KEYCLOAK_ADMIN_PASSWORD.

    - После входа под временным администратором необходимо создать
      постоянного пользователя с админскими правами: "Users" -> "Add user" -> Заполнить как минимум Username,
      Email и поставить переключатель Email verified. Далее в созданном аккаунте перейти в "Role mapping" ->
      "Assign role" -> "Realm roles" -> "realm-admin" -> "Assign".

   - Зайдите в "Realm settings" -> "Themes" -> "Admin theme" и выберите тему "phasetwo.v2", без этой манипуляции ни в одном из
     созданных realm'ов не появятся в дальнейшем нужные пункты меню ("Attributes")

   - Создайте realm, т.е "область" для проекта, где будут расположены его настройки и пользователи:
     "Manage realms" -> "Create realm" -> Укажите "Realm name" (swsm) -> "Create".
     Создастся realm с именем "swsm". Перейдите в "Realm settings", укажите "Realm name" (swsm), "Display name" (swsm),
     "HTML Display name" (swsm), на вкладке "Login" включите переключатели "User registration", "Forgot password",
     "Remember me", "Login with email", "Verify email". На вкладке "Email" введите настройки своего почтового сервера или транспорта.
     На вкладке "Localization" при желании включите переключатель "Internationalization", добавьте необходимую
     локализацию в "Supported locales" и выберите "Default locale".

   - **ВАЖНО!** В созданном realm (swsm) зайдите "Realm settings" -> "Themes" -> "Admin theme" и выберите тему "phasetwo.v2", перезагрузите страницу, на текущей странице
     станет доступна вкладка "Attributes", зайдите в ее, в поле "Key" выставите __providerConfig.ext-event-http.0_,
     в поле "Value" выставите _{"targetUri":"https://swsm.example.ru/keycloak-events","retry":true,"backoffInitialInterval":500}_

   - **ВАЖНО!** Во вкладке "Events" -> "Event listeners" добавьте "ext-event-http" и нажмите "Save". В "User events settings" включите "Save events" и выберите как минимум "Register", "Register error",
     "Delete account", "Delete account error".
     В "Admin events settings" включите "Save events" и "Include representation".    

   - Создайте клиента, через который SWSM будет подключаться к Keycloak. Для этого перейдите в "Clients" -> "Create client"
     "Client type" оставьте без изменений (OpenID Connect), укажите "Client ID" (swsm), "Name" (swsm), "Description" (swsm) -> "Next" ->
     поставьте галочки "Authentication flow" и "Direct access grants" -> "Next" -> Введите "Root URL" (https://swsm.example.com),
     "Valid redirect URIs" (https://swsm.example.ru/ и https://swsm.example.ru/*) и "Web origins" (https://swsm.example.com).
     Нажмите кнопку "Save". Во вкладке "Client scopes" войдите в "swsm-dedicated" -> "Mappers" -> "Configure a new mapper" -> "Audience".
     Введите "Name" и выберите в "Included Client Audience" название вашего realm-а.
   
   - В дальнейшем настройки Keycloak можно будет экспортировать и добавить их импорт в настройки контейнера с Keycloak, чтобы 
     в будущем разворачивать проект и не тратить время на настройки. Я намеренно не стал этого делать, т.к.
     при экспорте настроек экспортируются и приватный RSA-ключ, HMAC-секрет, приватный RSA-ключ для шифрования, AES-секрет и т.д.
     Делать шаблон для автогенерации из какой-либо одной конфигурации было бы небезопасно и безответственно.
      
     Экспорт конфигурации можно сделать из консоли контейнера:
     ```bash
      cd opt/keycloak
      bin/kc.sh export --dir temp/exports/swsm --realm swsm --users realm_file
     ```
     Более подробно про экспорт и импорт можно почитать в официальной документации Keycloak (https://www.keycloak.org/server/importExport)  
     Контейнер с заранее подготовленной конфигурацией для импорта при запуске будет выглядеть так:
     <details>
        <summary>Показать пример</summary>

         ```yaml
         auth:
           container_name: auth
           image: quay.io/phasetwo/phasetwo-keycloak:26.5.0
           restart: unless-stopped
           ports:
             - "8081:8080"
           env_file:
             - .env.production
           environment:
             KC_HOSTNAME: auth.example.com
             KC_PROXY_HEADERS: xforwarded
             KC_HTTP_ENABLED: "true"
             KC_HOSTNAME_STRICT: "false"
             KC_HEALTH_ENABLED: "true"
             KC_DB: postgres
             KC_DB_URL: jdbc:postgresql://auth_db:5432/${KEYCLOAK_DB}
             KC_DB_USERNAME: ${KEYCLOAK_POSTGRES_USER}
             KC_DB_PASSWORD: ${KEYCLOAK_POSTGRES_PASSWORD}
             KC_BOOTSTRAP_ADMIN_USERNAME: ${KEYCLOAK_ADMIN_USERNAME}
             KC_BOOTSTRAP_ADMIN_PASSWORD: ${KEYCLOAK_ADMIN_PASSWORD}
           command: ["start", "--optimized", "--spi-events-listener-ext-event-http-enabled=true", "--import-realm"] # Добавляем "--import-realm"
           depends_on:
             auth_db:
               condition: service_healthy
           volumes:
             - auth_data:/opt/keycloak/data
             - ./keycloak:/opt/keycloak/data/import:ro # Указываем директорию импорта
           healthcheck:
             test: ["CMD-SHELL", "exec 3<>/dev/tcp/localhost/8080 && echo 'OK' || exit 1"]
             interval: 10s
             timeout: 5s
             retries: 5
             start_period: 40s
         ```
        </details>

   С параметром --import-realm сервер попытается импортировать любой _.json_ или _.xml_ файл конфигурации
   из указанного (data/import) каталога. Подкаталоги игнорируются.

8. SWSM будет работать на https://swsm.example.com, панель управления Keycloak: https://auth.example.com.
Если запускали без веб-интерфейса, то API будет расположен на https://swsm.example.com/api .

## Создание пользователя на сервере Windows

В корне проекта находится файл **_create_user.ps1_**, являющийся скриптом PowerShell,
с помощью которого вы можете добавить пользователя на сервер Windows с ограниченными правами для:
   - просмотра статусов служб, без возможности управления,
   - просмотр статусов служб и управление только избранными службами,
   - просмотр статусов служб и полный доступ к управлению службами (кроме критических служб).

Также доступно добавление разрешенных для управления служб уже существующему пользователю, отзыв 
разрешений на управление службами, а так же удаление пользователей и их разрешений.

Для запуска скопируйте скрипт на Windows Server, откройте окно PowerShell от имени администратора, 
укажите путь до **_create_user.ps1_** и выполните скрипт, следуйте экранным подсказкам:
```bash
 PS C:\Users\SampleUser\Desktop> .\create_user.ps1
```
![screenshot_4.png](screenshots/screenshot_4.png)

**Рекомендации по безопасности:**
- Всегда используйте сложные пароли (минимум 8 символов, буквы + цифры + спецсимволы)
- Не используйте -Force без крайней необходимости и уверенности в своих действиях (!),
- Тестируйте через -DryRun перед реальным применением,
- Сохраняйте backup'ы SDDL из %TEMP% на случай отката.

<details>
<summary>
Примеры использования скрипта с параметрами:
</summary>

**1. Создание пользователя только с просмотром:**
```powershell
.\create_user.ps1 -Mode Create -UserName readonly_user -Password "Pass456!" -ReadOnly
```   
Что делает:
   - создаёт пользователя _readonly_user_ с доступом только на чтение всех служб,
   - управление службами недоступно.

**2. Добавление прав существующему пользователю:**
```powershell
.\create_user.ps1 -Mode GrantOnly -UserName existing_user -Services MSSQLSERVER,SQLSERVERAGENT
```   
Что делает:
   - добавляет права на управление SQL Server службами существующему пользователю.

**3. Создание нового пользователя с полным управлением избранными службами:**
```powershell
.\create_user.ps1 -Mode Create -UserName monitor_user -Password "SecurePass123!" -Services wuauserv,Spooler,W3SVC
```
Что делает:
   - создаёт локального пользователя _monitor_user_,
   - добавляет в группы: Пользователи удаленного управления, Пользователи DCOM, Читатели журнала событий 
(Remote Management Users, DCOM Users, Event Log Readers),
   - даёт права на просмотр всех служб,
   - Даёт полное управление (Start/Stop/Restart) службами: Windows Update, Диспетчер печати, IIS.

**4. Отзыв прав на службы:**
```powershell
.\create_user.ps1 -Mode Rollback -UserName monitor_user -Services wuauserv,Spooler
```
Что делает:
   - удаляет права на управление указанными службами у пользователя _monitor_user_,
   - убирает ACE из SCM (Service Control Manager).

**5. Полное удаление пользователя:**
```powershell
.\create_user.ps1 -Mode DeleteUser -UserName monitor_user
```
Что делает:
   - удаляет пользователя из системы,
   - очищает права из SCM и WinRM,
   - удаляет из всех групп.

**6. Дополнительные параметры:**

   **_-DryRun_** - тестовый режим. Показывает, какие изменения будут применены, без реального выполнения:
```powershell
 .\create_user.ps1 -Mode Create -UserName test_user -Password "Test123!" -Services W3SVC -DryRun 
```
   **_-Force_** - обход защиты критических служб. НЕ РЕКОМЕНДУЕТСЯ!
Позволяет изменять критические службы (может привести к BSOD):

```powershell
.\create_user.ps1 -Mode Create -UserName admin -Password "P@ss!" -Services Dhcp -Force
```

**7. Восстановление из backup:**

Найдите последний backup:
```powershell
Get-ChildItem $env:TEMP\*.SDDL.backup.*.txt | Sort-Object LastWriteTime -Descending
```
Восстановите службу:
```powershell
sc.exe sdset <ServiceName> (Get-Content "C:\Users\...\Temp\ServiceName.SDDL.backup.20260216_143012.txt" -Raw)
```
</details>
