# simple-windows-services-monitor

**Simple Windows Services Monitor** — это простой многопользовательский сервис, 
который позволяет управлять службами (остановка, запуск, перезапуск, получение текущего статуса) 
на удалённых серверах Windows. 

Для взаимодействия используется [WinRM](https://github.com/masterzen/winrm).

---

## Возможности

- 🔑 Многопользовательская аутентификация через JWT.
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
- Windows Server с включённым WinRM

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

## Установка и запуск (для разработки)

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
   ```

3. Создайте в корне файл `.env` и заполните своими данными (пример дан в env_example):
    ```env
    # SWSM init vars
    ####################################################################################
    # Раскомментировать для локальной разработки
    DATABASE_URI=postgres://swsm:userpassword@localhost:5432/swsm?sslmode=disable
    # Раскомментировать на продакшене
    #DATABASE_URI=postgres://swsm:userpassword@db:5432/swsm?sslmode=disable
    RUN_ADDRESS=127.0.0.1:8080
    LOG_LEVEL=debug
    LOG_OUTPUT=./logs/swsm.log
    AES_KEY=your-base64-key
    SECRET_KEY=your-jwt-secret
    WEB_INTERFACE=true
    
    # Postgres init vars (для образа postgres)
    ####################################################################################
    # тот же логин, что в URI
    POSTGRES_USER=swsm
    # тот же пароль, что в URI
    POSTGRES_PASSWORD=userpassword
    # то же имя БД, что в URI
    POSTGRES_DB=swsm
   ```

4. Соберите бинарник и запустите сервер:
   ```bash
   cd ./cmd/swsm/
   go build -o "swsm"
   ./swsm
   ```
   
5. Если в `env` вы оставили `WEB_INTERFACE=true` то нужно запустить сервер статики. 
Для запуска сервера статики:
   ```bash
   cd ../../static
   go build -o static-server
   ./static-server -port=3000 -dir=./
   ```
---

## Запуск в docker-контейнерах

1. Клонируйте репозиторий:
   ```bash
   git clone https://github.com/trsv-dev/simple-windows-services-monitor.git
   cd simple-windows-services-monitor
   ```

2. Создайте в корне файл `.env` и заполните своими данными (пример дан в env_example):
    ```env
    # SWSM init vars
    ####################################################################################
    # Раскомментировать для локальной разработки
    #DATABASE_URI=postgres://swsm:userpassword@localhost:5432/swsm?sslmode=disable
    # Раскомментировать на продакшене
    DATABASE_URI=postgres://swsm:userpassword@db:5432/swsm?sslmode=disable
    RUN_ADDRESS=127.0.0.1:8080
    LOG_LEVEL=debug
    LOG_OUTPUT=./logs/swsm.log
    AES_KEY=your-base64-key
    SECRET_KEY=your-jwt-secret
    WEB_INTERFACE=true
    
    # Postgres init vars (для образа postgres)
    ####################################################################################
    # тот же логин, что в URI
    POSTGRES_USER=swsm
    # тот же пароль, что в URI
    POSTGRES_PASSWORD=userpassword
    # то же имя БД, что в URI
    POSTGRES_DB=swsm
   ```
3. Если вы хотите собрать проект с веб-интерфейсом (в `env` вы оставили `WEB_INTERFACE=true`), 
то из корня проекта (где расположен `docker-compose.yml`) выполните:
   ```bash
   docker compose --profile frontend up -d --build
   ```
   Если вы хотите использовать только API (в `env` вы оставили `WEB_INTERFACE=false`),
   то из корня проекта (где расположен `docker-compose.yml`) выполните:
   ```bash
   docker compose up -d --build
   ```
4. 