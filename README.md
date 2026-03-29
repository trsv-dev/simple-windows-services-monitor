# simple-windows-services-monitor

**Simple Windows Services Monitor (SWSM)** — многопользовательский сервис для управления службами Windows на удалённых серверах. 
Управление возможно только в рамках доверенного контура, например, когда серверы связаны между собой посредством VPN, где SWSM имеет прямой доступ к управляемым серверам. 
С помощью сервиса можно запускать, останавливать, перезапускать службы и получать их текущий статус.

Для взаимодействия используется [WinRM](https://github.com/masterzen/winrm).

| [<img src="screenshots/screenshot_1.png" width="250"/>](screenshots/screenshot_1.png) | [<img src="screenshots/screenshot_2.png" width="250"/>](screenshots/screenshot_2.png) | [<img src="screenshots/screenshot_3.png" width="250"/>](screenshots/screenshot_2.png) |
|---------------------------------------------------------------------------------------|---------------------------------------------------------------------------------------|---------------------------------------------------------------------------------------|
| Список серверов                                                                       | Детали сервера                                                                       | Добавление службы                                                                     |


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

3. Разверните docker-контейнер с Keycloak


# Нумарация !!!


3. Создайте в корне файл `.env.development` и заполните своими данными (пример дан в env_example):
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
    LOG_LEVEL=debug
    LOG_OUTPUT=./logs/swsm.log
    AES_KEY=your-base64-key
    WEB_INTERFACE=true
    API_BASE_URL=http://localhost:8080/api
    
    # Postgres init vars (для образа postgres)
    ####################################################################################
    # тот же логин, что в URI
    POSTGRES_USER=swsm
    # тот же пароль, что в URI
    POSTGRES_PASSWORD=userpassword
    # то же имя БД, что в URI
    POSTGRES_DB=swsm
   
   # Keycloak init vars (OpenID Connect)
   ####################################################################################
   # Базовый URL Keycloak сервера, включая realm.
   # Формат: http(s)://<host>:<port>/realms/<realm_name>
   # Пример: http://localhost:8081/realms/swsm
   # Или по имени контейнера, например: keycloak:8081/realms/swsm
   KEYCLOAK_ISSUER_URL=keycloak:8081/realms/swsm
   
   # Идентификатор клиента (client_id), зарегистрированного в Keycloak.
   # Должен совпадать с именем клиента, указанным в консоли Keycloak.
   KEYCLOAK_CLIENT_ID=swsm
   ```

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
   Бэкенд будет доступен по адресу, который вы указали в `RUN_ADDRESS`, например: http://127.0.0.1:8080


5. Если в `env` вы оставили `WEB_INTERFACE=true` (веб-интерфейс включен) то нужно запустить сервер статики. 
Для запуска сервера статики:
   ```bash
   cd ../../static
   go build -o static-server
   ./static-server -port=3000 -dir=./
   ```
6. Веб-интерфейс будет доступен по адресу: http://127.0.0.1:3000, API: http://127.0.0.1:8080/api
---

## Запуск в docker-контейнерах

1. Клонируйте репозиторий:
   ```bash
   git clone https://github.com/trsv-dev/simple-windows-services-monitor.git
   cd simple-windows-services-monitor
   ```

2. Создайте в корне файл `.env.production` и заполните своими данными (пример дан в env_example):
    ```env
    # SWSM init vars
    ####################################################################################
    DATABASE_URI=postgres://swsm:userpassword@db:5432/swsm?sslmode=disable
    RUN_ADDRESS=127.0.0.1:8080
    # Используйте 5985 (http) или 5986 (https) порты
    WINRM_PORT=5985
    # Установите флаг в значение true для HTTPS-соединений с WinRM
    WINRM_USE_HTTPS=false
    # Установите флаг в значение true, чтобы пропустить проверку SSL (например, для самоподписанных сертификатов).
    WINRM_INSECURE_FOR_HTTPS=false
    LOG_LEVEL=debug
    LOG_OUTPUT=./logs/swsm.log
    AES_KEY=your-base64-key
    SECRET_KEY=your-jwt-secret
    WEB_INTERFACE=true
    API_BASE_URL=/api
    
    # Postgres init vars (для образа postgres)
    ####################################################################################
    # тот же логин, что в URI
    POSTGRES_USER=swsm
    # тот же пароль, что в URI
    POSTGRES_PASSWORD=userpassword
    # то же имя БД, что в URI
    POSTGRES_DB=swsm
   
    # Keycloak init vars (OpenID Connect)
    ####################################################################################
    # Базовый URL Keycloak сервера, включая realm.
    # Формат: http(s)://<host>:<port>/realms/<realm_name>
    # Пример: http://localhost:8081/realms/swsm
    # Или по имени контейнера, например: keycloak:8081/realms/swsm
    KEYCLOAK_ISSUER_URL=keycloak:8081/realms/swsm
    
    # Идентификатор клиента (client_id), зарегистрированного в Keycloak.
    # Должен совпадать с именем клиента, указанным в консоли Keycloak.
    KEYCLOAK_CLIENT_ID=swsm
   ```
3. Если вы хотите собрать проект с веб-интерфейсом (в `env` вы оставили `WEB_INTERFACE=true`), 
то из корня проекта (где расположен `docker-compose.yml`) выполните:
   ```bash
   docker compose --env-file .env.production --profile frontend up -d --build
   ```
   Если вы хотите использовать только API (в `env` вы оставили `WEB_INTERFACE=false`),
   то из корня проекта (где расположен `docker-compose.yml`) выполните:
   ```bash
   docker compose --env-file .env.production up -d --build
   ```
4. Фронтенд (если поднят): http://127.0.0.1/ (порт 80 по умолчанию), API: http://127.0.0.1:8080/api

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
