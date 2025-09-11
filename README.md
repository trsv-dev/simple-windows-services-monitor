# simple-windows-services-monitor

**Simple Windows Services Monitor** — это простой многопользовательский сервис, который позволяет управлять службами (остановка, запуск, перезапуск) на удалённых серверах Windows.  
Для взаимодействия используется [WinRM](https://github.com/masterzen/winrm).

---

## Возможности

- 🔑 Многопользовательская аутентификация через JWT.
- 🕹️ Управление службами Windows (start, stop, restart).
- 📡 Работа с удалёнными серверами по WinRM.
- 📦 Хранение данных в PostgreSQL.
- 📜 Логирование действий.
---

## Требования

- Go 1.22+
- PostgreSQL 14+
- Windows Server с включённым WinRM

---

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

## Установка и запуск

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

3. Подготовьте `.env` файл (см. пример `.env.example`):

   ```env
   DATABASE_URI=postgres://swsm:userpassword@localhost:5432/swsm?sslmode=disable
   RUN_ADDRESS=127.0.0.1:8080
   LOG_LEVEL=debug
   AES_KEY=your-base64-key
   SECRET_KEY=your-jwt-secret
   ```

4. Запустите сервер:

   ```bash
   go run ./cmd/swsm
   ```
---