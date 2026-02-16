<#
.SYNOPSIS
  Создаёт локального пользователя / выдаёт права / откатывает права на службах и в SCM / удаляет локального пользователя.
  С защитой от изменения критических системных служб.

.PARAMETER UserName
  Имя пользователя (локальное).

.PARAMETER Password
  Пароль для создания (только в режиме Create).

.PARAMETER Mode
  Режим: Create | GrantOnly | Rollback | DeleteUser

.PARAMETER Services
  Список служб (имена). Пример: Spooler, wuauserv

.PARAMETER Force
  Пропустить проверки безопасности (НЕ РЕКОМЕНДУЕТСЯ!)

.PARAMETER ReadOnly
  Режим "только просмотр" - пользователь видит все службы, но не может ими управлять

.PARAMETER DryRun
  Тестовый режим - показывает что будет сделано, но не выполняет изменения

.EXAMPLE
  .\script.ps1 -Mode Create -UserName monitor1 -Password "Pass123!" -Services wuauserv,Spooler

.EXAMPLE
  .\script.ps1 -Mode Create -UserName monitor1 -Password "Pass123!" -ReadOnly

.EXAMPLE
  .\script.ps1 -Mode Rollback -UserName monitor1 -Services wuauserv,Spooler

.EXAMPLE
  .\script.ps1 -Mode DeleteUser -UserName monitor1

.EXAMPLE
  .\script.ps1 -Mode GrantOnly -UserName test -Services MSSQLSERVER -DryRun
#>

param(
    [string]$UserName,
    [string]$Password,
    [ValidateSet("Create","GrantOnly","Rollback","DeleteUser")]
    [string]$Mode,
    [string[]]$Services,
    [switch]$Force,
    [switch]$ReadOnly,
    [switch]$DryRun
)

# -------------------------
# КРИТИЧЕСКИЕ СЛУЖБЫ - ПОЛЬЗОВАТЕЛЮ ЗАПРЕЩЕНО ИЗМЕНЯТЬ СОСТОЯНИЕ (нижний регистр)
# -------------------------
$CRITICAL_SERVICES = @(
    'scmanager',
    'rpcss',
    'dcomlaunch',
    'eventlog',
    'plugplay',
    'power',
    'profsvc',
    'samss',
    'lanmanserver',
    'lanmanworkstation',
    'dhcp',
    'dnscache',
    'winhttpautoproxysvc',
    'bits',
    'cryptsvc',
    'netlogon',
    'ntds',
    'w32time',
    'schedule',
    'wsearch',
    'trustedinstaller'
) | ForEach-Object { $_.ToLower() }

# -------------------------
# ГЛОБАЛЬНЫЕ ПЕРЕМЕННЫЕ
# -------------------------
$global:FullBackupPath = $null
$global:LogFilePath = $null

# -------------------------
# УТИЛИТЫ ВЫВОДА
# -------------------------
function Write-Info($m){ Write-Host "[*] $m" -ForegroundColor Cyan }
function Write-Warn($m){ Write-Host "[!] $m" -ForegroundColor Yellow }
function Write-Err($m){ Write-Host "[X] $m" -ForegroundColor Red }
function Write-Success($m){ Write-Host "[OK] $m" -ForegroundColor Green }
function Write-DryRun($m){ Write-Host "[DRY-RUN] $m" -ForegroundColor Magenta }

# -------------------------
# ПРОВЕРКА ПРАВ АДМИНИСТРАТОРА
# -------------------------
$principal = New-Object Security.Principal.WindowsPrincipal([Security.Principal.WindowsIdentity]::GetCurrent())
if (-not $principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)) {
    Write-Err "Скрипт должен быть запущен от имени администратора."
    exit 1
}

# -------------------------
# ПРОВЕРКА ЗАПУСКА ЧЕРЕЗ WinRM
# -------------------------
function Test-RemoteSession {
    $isRemote = $false

    # Проверяем WSMan (PSRemoting)
    if ($PSSenderInfo -or $env:WSManStackVersion) {
        $isRemote = $true
    }

    # Проверяем Session Name
    if ($env:SESSIONNAME -and $env:SESSIONNAME -notmatch "Console|RDP-Tcp") {
        $isRemote = $true
    }

    return $isRemote
}

if ((Test-RemoteSession) -and -not $DryRun) {
    Write-Host "`n========================================" -ForegroundColor Yellow
    Write-Host "    ВНИМАНИЕ: УДАЛЁННАЯ СЕССИЯ !" -ForegroundColor Yellow
    Write-Host "========================================" -ForegroundColor Yellow
    Write-Warn "Скрипт запущен через WinRM/PSRemoting!"
    Write-Warn "При изменении WinRM RootSDDL сессия может оборваться!"
    Write-Host "`nРекомендуется:" -ForegroundColor Cyan
    Write-Host "  1. Запускать локально через RDP"
    Write-Host "  2. Или убедиться, что есть альтернативный доступ (RDP/консоль)"
    Write-Host ""

    $confirm = Read-Host "Продолжить выполнение? (Y/N)"
    if ($confirm -notin @('Y','y')) {
        Write-Info "Выполнение отменено"
        exit 0
    }
}

# -------------------------
# ЦЕНТРАЛИЗОВАННОЕ ЛОГИРОВАНИЕ
# -------------------------
function Start-ScriptLogging {
    $logDir = "C:\Logs\WinRM_ServicePermissions"

    if (-not (Test-Path $logDir)) {
        try {
            New-Item -Path $logDir -ItemType Directory -Force | Out-Null
        } catch {
            $logDir = $env:TEMP
        }
    }

    $global:LogFilePath = Join-Path $logDir "Script_$(Get-Date -Format 'yyyyMMdd_HHmmss').log"

    try {
        Start-Transcript -Path $global:LogFilePath -Append -ErrorAction Stop
        Write-Success "Логирование начато: $global:LogFilePath"
    } catch {
        Write-Warn "Не удалось включить логирование: $_"
        $global:LogFilePath = $null
    }
}

function Stop-ScriptLogging {
    if ($global:LogFilePath) {
        try {
            Stop-Transcript -ErrorAction SilentlyContinue
            Write-Success "Полный лог сохранён: $global:LogFilePath"
        } catch {}
    }
}

# Начинаем логирование
if (-not $DryRun) {
    Start-ScriptLogging
}

# -------------------------
# ПОЛНЫЙ БЭКАП ВСЕХ СЛУЖБ
# -------------------------
function Backup-AllServicesSDDL {
    $backupFile = Join-Path $env:TEMP "AllServices.SDDL.full.backup.$(Get-Date -Format 'yyyyMMdd_HHmmss').json"
    Write-Info "Создаём полный бэкап всех служб (это займёт ~30 секунд)..."

    $allServices = Get-Service -ErrorAction SilentlyContinue
    $backup = @{}
    $processed = 0

    foreach ($svc in $allServices) {
        try {
            $sddl = (sc.exe sdshow $svc.Name 2>&1) -join ""
            if ($LASTEXITCODE -eq 0) {
                $backup[$svc.Name] = $sddl
            }
            $processed++

            if ($processed % 50 -eq 0) {
                Write-Host "." -NoNewline
            }
        } catch {}
    }

    Write-Host ""

    try {
        $backup | ConvertTo-Json -Depth 10 | Out-File $backupFile -Encoding UTF8
        Write-Success "Полный бэкап: $backupFile ($(($backup.Keys | Measure-Object).Count) служб)"
        return $backupFile
    } catch {
        Write-Warn "Не удалось сохранить полный бэкап: $_"
        return $null
    }
}

# -------------------------
# ВАЛИДАЦИЯ SDDL
# -------------------------
function Test-SDDLValid {
    param([string]$SDDL)
    try {
        if (-not $SDDL -or $SDDL.Trim().Length -eq 0) { return $false }
        if ($SDDL -notmatch '[DASU]:' ) { return $false }
        $openCount = ([regex]::Matches($SDDL, '\(')).Count
        $closeCount = ([regex]::Matches($SDDL, '\)')).Count
        if ($openCount -ne $closeCount) { return $false }
        $sd = New-Object System.Security.AccessControl.CommonSecurityDescriptor($false, $false, $SDDL)
        return $true
    } catch {
        return $false
    }
}

# -------------------------
# Получение служб из реестра
# -------------------------
function Get-ServicesFromRegistry {
    param(
        [string]$Filter = "*",
        [switch]$OnlyRunning,
        [switch]$ExcludeCritical
    )

    Write-Info "Получаем список служб из реестра..."

    $servicesPath = "HKLM:\SYSTEM\CurrentControlSet\Services"
    $services = @()

    try {
        $allKeys = Get-ChildItem -Path $servicesPath -ErrorAction SilentlyContinue

        foreach ($key in $allKeys) {
            $serviceName = $key.PSChildName

            if ($Filter -ne "*" -and $serviceName -notlike "*$Filter*") {
                continue
            }

            try {
                $type = Get-ItemProperty -Path $key.PSPath -Name "Type" -ErrorAction SilentlyContinue
                if (-not $type -or $type.Type -notin @(0x10, 0x20, 0x110)) {
                    continue
                }
            } catch {
                continue
            }

            if ($ExcludeCritical -and $CRITICAL_SERVICES -contains $serviceName.ToLower()) {
                continue
            }

            $displayName = $serviceName
            try {
                $dn = Get-ItemProperty -Path $key.PSPath -Name "DisplayName" -ErrorAction SilentlyContinue
                if ($dn -and $dn.DisplayName) {
                    $displayName = $dn.DisplayName
                }
            } catch {}

            if ($OnlyRunning) {
                try {
                    $svc = Get-Service -Name $serviceName -ErrorAction SilentlyContinue
                    if ($svc.Status -ne 'Running') {
                        continue
                    }
                } catch {
                    continue
                }
            }

            $services += [PSCustomObject]@{
                Name = $serviceName
                DisplayName = $displayName
                Path = $key.PSPath
            }
        }

        Write-Success "Найдено служб: $($services.Count)"
        return $services

    } catch {
        Write-Err "Ошибка при чтении реестра: $_"
        return @()
    }
}

# -------------------------
# Интерактивный выбор служб из реестра
# -------------------------
function Select-ServicesFromRegistry {
    Write-Host "`nВыберите способ поиска служб:" -ForegroundColor Cyan
    Write-Host "  1 - Все службы (показать список)"
    Write-Host "  2 - Ввести имена вручную"
    Write-Host ""

    $choice = Read-Host "Введите 1 или 2"

    switch ($choice) {
        "1" {
            $allServices = Get-ServicesFromRegistry -ExcludeCritical

            Write-Host "`nДоступные службы (исключены критические):" -ForegroundColor Cyan
            for ($i = 0; $i -lt $allServices.Count; $i++) {
                $svc = $allServices[$i]
                Write-Host "  [$i] $($svc.Name) - $($svc.DisplayName)"

                if (($i + 1) % 20 -eq 0) {
                    $continue = Read-Host "`nПоказать еще? (Y/N)"
                    if ($continue -notin @('Y','y')) { break }
                }
            }

            Write-Host "`nВведите номера служб через запятую, диапазон (например, 0-7) или ALL (для выбора всех служб):" -ForegroundColor Yellow
            $selection = Read-Host "Выбор"

            if ($selection -eq "ALL") {
                return $allServices.Name
            }

            $selected = @()

            $parts = $selection -split ','
            foreach ($part in $parts) {
                $part = $part.Trim()
                if ($part -match '^(\d+)-(\d+)$') {
                    $start = [int]$matches[1]
                    $end = [int]$matches[2]
                    for ($i = $start; $i -le $end; $i++) {
                        if ($i -lt $allServices.Count) {
                            $selected += $allServices[$i].Name
                        }
                    }
                } elseif ($part -match '^\d+$') {
                    $idx = [int]$part
                    if ($idx -lt $allServices.Count) {
                        $selected += $allServices[$idx].Name
                    }
                }
            }

            return $selected
        }

        "2" {
            return Read-ServicesInteractive
        }
    }
}

# -------------------------
# Ввод служб интерактивно (ручной ввод)
# -------------------------
function Read-ServicesInteractive {
    Write-Host ""
    Write-Host "Вводите имена служб по одному. Для завершения введите STOP." -ForegroundColor Cyan
    Write-Host "ВНИМАНИЕ: Критические системные службы будут автоматически пропущены." -ForegroundColor Yellow
    $list = @()
    while ($true) {
        $svc = Read-Host "Имя службы (или STOP)"
        if ($null -eq $svc) { continue }
        $svc = $svc.Trim()
        if ($svc.Length -eq 0) { continue }
        if ($svc.ToUpper() -eq 'STOP') { break }
        $list += $svc
    }
    return $list
}

# -------------------------
# Режим/имя/пароль/список служб
# -------------------------
if (-not $Mode) {
    Write-Host ""
    Write-Host "Выберите режим работы:" -ForegroundColor Cyan
    Write-Host "  1 - Создать НОВОГО пользователя и выдать права"
    Write-Host "  2 - Выдать права СУЩЕСТВУЮЩЕМУ пользователю"
    Write-Host "  3 - Откатить права пользователя из служб и SCM"
    Write-Host "  4 - Удалить пользователя полностью"
    Write-Host ""
    do { $choice = Read-Host "Введите 1, 2, 3 или 4" } until ($choice -in @("1","2","3","4"))
    $Mode = @{ "1"="Create"; "2"="GrantOnly"; "3"="Rollback"; "4"="DeleteUser" }[$choice]
}

if ($Mode -ne "Rollback" -and $Mode -ne "DeleteUser" -and -not $PSBoundParameters.ContainsKey('ReadOnly')) {
    Write-Host ""
    Write-Host "Выберите уровень прав:" -ForegroundColor Cyan
    Write-Host "  1 - Только просмотр служб (пользователь видит ВСЕ службы, но не может управлять)"
    Write-Host "  2 - Полное управление службами (Start / Stop / Restart)"
    Write-Host ""

    $rightsChoice = Read-Host "Введите 1 или 2 (по умолчанию: 1)"
    if ($rightsChoice -eq "" -or $rightsChoice -eq "1") {
        $ReadOnly = $true
    } else {
        $ReadOnly = $false
    }
}

if (-not $UserName -or $UserName.Trim().Length -eq 0) {
    $UserName = Read-Host "Введите имя пользователя (например: user_123)"
    if (-not $UserName -or $UserName.Trim().Length -eq 0) {
        Write-Err "Имя пользователя не может быть пустым."
        exit 1
    }
}

if ($Mode -eq "Create" -and -not $Password) {
    $sec = Read-Host "Введите пароль (ввод скрыт)" -AsSecureString
    try {
        $Password = [Runtime.InteropServices.Marshal]::PtrToStringAuto([Runtime.InteropServices.Marshal]::SecureStringToBSTR($sec))
    } catch {
        Write-Err "Не удалось прочитать пароль: $_"
        exit 1
    }
}

if ($Mode -ne "Rollback" -and $Mode -ne "DeleteUser" -and (-not $ReadOnly -or ($Services -and $Services.Count -gt 0))) {
        if (-not $Services -or $Services.Count -eq 0) {
            $Services = Select-ServicesFromRegistry
        }
    } elseif ($Mode -eq "Rollback") {
        if (-not $Services -or $Services.Count -eq 0) {
            Write-Host ""
            Write-Warn "Для отката нужно указать службы"
            $Services = Select-ServicesFromRegistry
        }
    }

if (-not $Force -and $Services -and $Services.Count -gt 0) {
    $filtered = @()
    $blocked = @()
    foreach ($s in $Services) {
        if ($s -and ($CRITICAL_SERVICES -contains $s.ToLower())) {
            $blocked += $s
        } else {
            $filtered += $s
        }
    }
    if ($blocked.Count -gt 0) {
        Write-Warn "Заблокированы критические службы: $($blocked -join ', ')"
        Write-Warn "Используйте -Force для обхода (НЕ РЕКОМЕНДУЕТСЯ)."
    }
    $Services = $filtered
}

# -------------------------
# УСИЛЕННАЯ ПРОВЕРКА ДЛЯ -Force
# -------------------------
if ($Force -and -not $DryRun) {
    Write-Host "`n!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!" -ForegroundColor Red
    Write-Host "    ПРЕДУПРЕЖДЕНИЕ   " -ForegroundColor Red
    Write-Host "!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!" -ForegroundColor Red
    Write-Host ""
    Write-Host "Флаг -Force ОТКЛЮЧАЕТ ЗАЩИТУ критических служб!" -ForegroundColor Red
    Write-Host ""
    Write-Host "Возможные последствия:" -ForegroundColor Yellow
    Write-Host "  - Остановка RPC/DCOM --> BSOD и полный отказ системы" -ForegroundColor Red
    Write-Host "  - Остановка EventLog --> потеря всех логов" -ForegroundColor Red
    Write-Host "  - Остановка Schedule --> не работают задачи" -ForegroundColor Red
    Write-Host "  - Остановка Netlogon --> проблемы с AD" -ForegroundColor Red
    Write-Host ""
    Write-Host "Вы должны ТОЧНО знать, что делаете! Все дальнейшие действия вы производите на свой страх и риск!" -ForegroundColor Yellow
    Write-Host ""

    $confirm = Read-Host "Введите 'I UNDERSTAND THE RISK' (заглавными) для продолжения"
    if ($confirm -ne "I UNDERSTAND THE RISK") {
        Write-Info "Выполнение отменено. Это правильное решение!"
        if ($global:LogFilePath) { Stop-ScriptLogging }
        exit 0
    }

    Write-Host ""
    Write-Host "Последнее предупреждение: вы уверены?" -ForegroundColor Red
    $finalConfirm = Read-Host "Введите YES"
    if ($finalConfirm -ne "YES") {
        Write-Info "Выполнение отменено"
        if ($global:LogFilePath) { Stop-ScriptLogging }
        exit 0
    }

    Write-Warn "Защита отключена. Будьте осторожны!"
}

# -------------------------
# DRY-RUN РЕЖИМ
# -------------------------
if ($DryRun) {
    Write-Host "`n========================================" -ForegroundColor Magenta
    Write-Host "   ТЕСТОВЫЙ РЕЖИМ (DRY-RUN)" -ForegroundColor Magenta
    Write-Host "========================================" -ForegroundColor Magenta
    Write-DryRun "Никакие изменения НЕ будут применены!"
    Write-DryRun "Скрипт покажет ЧТО будет сделано, но не выполнит действия."
    Write-Host "========================================`n" -ForegroundColor Magenta
}

# -------------------------
# Создание пользователя и добавление в группы
# -------------------------
function Ensure-LocalUser {
    param($Name, $PlainPassword)

    if ($DryRun) {
        Write-DryRun "Будет создан пользователь: $Name"
        return
    }

    Write-Info "Создаём пользователя: $Name"
    $securePwd = ConvertTo-SecureString $PlainPassword -AsPlainText -Force

    if (Get-Command -Name New-LocalUser -ErrorAction SilentlyContinue) {
        try {
            if (Get-LocalUser -Name $Name -ErrorAction SilentlyContinue) {
                Write-Info "Пользователь '$Name' уже существует."
                return
            }
            New-LocalUser -Name $Name -Password $securePwd -PasswordNeverExpires:$true -UserMayNotChangePassword:$false -AccountNeverExpires:$true -FullName $Name -Description "WinRM remote management" -ErrorAction Stop
            Write-Success "Создан пользователь '$Name'"
            return
        } catch {
            Write-Warn "New-LocalUser failed: $_"
        }
    }

    Write-Info "Используем net user..."
    cmd.exe /c "net user `"$Name`" `"$PlainPassword`" /add" | Out-Null
    if ($LASTEXITCODE -eq 0) {
        Write-Success "Создан пользователь '$Name' (net user)"
    } else {
        throw "Не удалось создать пользователя"
    }
}

function Remove-LocalUserComplete {
    param([string]$Name)

    Write-Host "`n========================================" -ForegroundColor Red
    Write-Host "  УДАЛЕНИЕ ПОЛЬЗОВАТЕЛЯ: $Name" -ForegroundColor Red
    Write-Host "========================================" -ForegroundColor Red

    $userExists = $false
    try {
        if (Get-Command -Name Get-LocalUser -ErrorAction SilentlyContinue) {
            $userExists = (Get-LocalUser -Name $Name -ErrorAction SilentlyContinue) -ne $null
        }

        if (-not $userExists) {
            try {
                $computer = (Get-CimInstance Win32_ComputerSystem).Name
                $userPath = "WinNT://$computer/$Name,user"
                $user = [ADSI]$userPath
                $userExists = ($user -and $user.psbase -and $user.Name)
            } catch {
                $userExists = $false
            }
        }

        if (-not $userExists) {
            Write-Warn "Пользователь '$Name' не найден."
            return $false
        }
    } catch {
        Write-Err "Ошибка при проверке пользователя: $_"
        return $false
    }

    Write-Host "`nВНИМАНИЕ! Будет выполнено:" -ForegroundColor Yellow
    Write-Host "  - Удаление пользователя '$Name'" -ForegroundColor Yellow
    Write-Host "  - Откат прав на SCM" -ForegroundColor Yellow
    Write-Host "  - Удаление из WinRM RootSDDL" -ForegroundColor Yellow
    Write-Host "  - Удаление из всех групп" -ForegroundColor Yellow
    Write-Host ""

    $confirm = Read-Host "Продолжить удаление? (введите YES для подтверждения)"
    if ($confirm -ne "YES") {
        Write-Info "Удаление отменено"
        return $false
    }

    if ($DryRun) {
        Write-DryRun "Будет удалён пользователь: $Name"
        Write-DryRun "Будут удалены права из SCM"
        Write-DryRun "Будет удалён из WinRM RootSDDL"
        return $true
    }

    try {
        $computer = (Get-CimInstance Win32_ComputerSystem).Name
        $acctFull = "$computer\$Name"
        $nt = New-Object System.Security.Principal.NTAccount($acctFull)
        $sid = $nt.Translate([System.Security.Principal.SecurityIdentifier]).Value
        Write-Info "SID пользователя: $sid"
    } catch {
        Write-Warn "Не удалось получить SID: $_"
        $sid = $null
    }

    if ($sid) {
        try {
            $result = Remove-FromSCManagerSDDL -Account $acctFull
            if ($result) {
                Write-Success "SCM: права удалены"
            }
        } catch {
            Write-Warn "Не удалось удалить права SCM: $_"
        }
    }

    if ($sid) {
        try {
            $item = Get-Item -Path WSMan:\localhost\Service\RootSDDL -ErrorAction Stop
            $sddl = $item.Value

            if ($sddl -match [regex]::Escape($sid)) {
                Write-Info "Удаляем из WinRM RootSDDL..."

                $bak = Join-Path $env:TEMP "WinRM.RootSDDL.backup.$(Get-Date -Format 'yyyyMMdd_HHmmss').txt"
                $sddl | Out-File -FilePath $bak -Encoding ASCII

                $pattern = "\(A;;[^;]+;;;$([regex]::Escape($sid))\)"
                $newSddl = [regex]::Replace($sddl, $pattern, "")

                if (Test-SDDLValid $newSddl) {
                    Set-Item -Path WSMan:\localhost\Service\RootSDDL -Value $newSddl -Force
                    Write-Success "WinRM: права удалены"

                    Restart-Service -Name WinRM -Force -ErrorAction Stop
                    Start-Sleep -Seconds 2
                    Write-Success "WinRM перезапущен"
                }
            }
        } catch {
            Write-Warn "Не удалось удалить из WinRM: $_"
        }
    }

    Write-Info "Удаляем пользователя '$Name'..."

    if (Get-Command -Name Remove-LocalUser -ErrorAction SilentlyContinue) {
        try {
            Remove-LocalUser -Name $Name -ErrorAction Stop
            Write-Success "Пользователь удалён (Remove-LocalUser)"
            return $true
        } catch {
            Write-Warn "Remove-LocalUser failed: $_"
        }
    }

    try {
        $result = cmd.exe /c "net user `"$Name`" /delete" 2>&1
        if ($LASTEXITCODE -eq 0) {
            Write-Success "Пользователь удалён (net user)"
            return $true
        } else {
            Write-Err "net user вернул ошибку: $($result -join ' ')"
            return $false
        }
    } catch {
        Write-Err "Не удалось удалить пользователя: $_"
        return $false
    }
}

function Add-ToRemoteManagementGroup {
    param([string]$Name)

    if ($DryRun) {
        Write-DryRun "Будет добавлен '$Name' в Remote Management Users"
        return
    }

    Write-Info "Проверяем членство в группе `Пользователи удаленного управления`..."

    $groupNames = @("Пользователи удаленного управления","Remote Management Users")

    if ($Name -match '\\') {
        $acctFull = $Name
    } else {
        $computer = (Get-CimInstance Win32_ComputerSystem -ErrorAction SilentlyContinue).Name
        if (-not $computer) { $computer = $env:COMPUTERNAME }
        $acctFull = "$computer\$Name"
    }

    function Group-Exists($gName) {
        try {
            $out = cmd.exe /c "net localgroup `"$gName`"" 2>&1
            return ($LASTEXITCODE -eq 0)
        } catch { return $false }
    }

    function Is-MemberOf($gName, $account) {
        try {
            $members = net localgroup "$gName" 2>&1 | Select-String -Pattern $account -SimpleMatch
            return ($members -ne $null)
        } catch {
            return $false
        }
    }

    # Сначала проверяем, не состоит ли пользователь уже в одной из групп
    foreach ($g in $groupNames) {
        if (Group-Exists $g) {
            if (Is-MemberOf $g $Name) {
                Write-Success "Пользователь '$Name' уже в группе '$g' "
                return
            }
        }
    }

    # Если не состоит ни в одной - пытаемся добавить
    foreach ($g in $groupNames) {
        # Проверяем существование группы
        if (-not (Group-Exists $g)) {
            Write-Info "Группа '$g' не найдена, пропускаем..."
            continue
        }

        # Пытаемся Add-LocalGroupMember
        if (Get-Command -Name Add-LocalGroupMember -ErrorAction SilentlyContinue) {
            try {
                Add-LocalGroupMember -Group $g -Member $acctFull -ErrorAction Stop
                Write-Success "Добавлен в '$g' (Add-LocalGroupMember)"
                return
            } catch {
                # Проверяем, не ошибка ли "уже в группе"
                if ($_.Exception.Message -match "уже входит в группу|already a member") {
                    Write-Success "Пользователь '$Name' уже в группе '$g' "
                    return
                }
                Write-Warn "Add-LocalGroupMember не сработал для '$g': $_"
            }
        }

        # Пытаемся через net localgroup
        try {
            $cmd = "net localgroup `"$g`" `"$acctFull`" /add"
            $res = cmd.exe /c $cmd 2>&1

            if ($LASTEXITCODE -eq 0) {
                Write-Success "Добавлен в '$g' (net localgroup)"
                return
            } elseif ($LASTEXITCODE -eq 2 -and ($res -join ' ') -match "уже входит в|already a member") {
                Write-Success "Пользователь '$Name' уже в группе '$g' "
                return
            } else {
                $joinedRes = $res -join ' '
                Write-Warn "net localgroup вернул код $LASTEXITCODE"
            }
        } catch {
            Write-Warn "net localgroup fallback не сработал для '$g': $_"
        }

        # Пытаемся через ADSI (WinNT)
        try {
            $computerName = $acctFull.Split('\')[0]
            $userName = $acctFull.Split('\')[1]
            $groupPath = "WinNT://$computerName/$g,group"
            $group = [ADSI]$groupPath

            if ($group -and $group.psbase) {
                $userPath = "WinNT://$computerName/$userName,user"
                $group.Add($userPath)
                Write-Success "Добавлен в '$g' (ADSI)"
                return
            }
        } catch {
            # Проверяем, не ошибка ли "уже в группе"
            if ($_.Exception.Message -match "уже входит в|already a member") {
                Write-Success "Пользователь '$Name' уже в группе '$g' "
                return
            }
            Write-Warn "ADSI добавление не сработало для '$g': $_"
        }
    }

    # Если дошли сюда - ничего не сработало
    Write-Err "Не удалось добавить '$acctFull' в Remote Management Users!"
    Write-Err "Пожалуйста, добавьте пользователя вручную:"
    Write-Host "  net localgroup `"Пользователи удаленного управления`" `"$acctFull`" /add" -ForegroundColor Yellow
    Write-Host ""

    $continue = Read-Host "Продолжить без добавления в группу? (Y/N)"
    if ($continue -notin @('Y','y')) {
        throw "Выполнение прервано: пользователь не добавлен в Remote Management Users"
    }
}


function Add-ToDcomUsers {
    param($Name)

    if ($DryRun) {
        Write-DryRun "Будет добавлен в Distributed COM Users"
        return
    }

    Write-Info "Добавляем в Distributed COM Users..."
    $groups = @("Distributed COM Users","Пользователи DCOM")
    foreach ($g in $groups) {
        try {
            Add-LocalGroupMember -Group $g -Member $Name -ErrorAction Stop
            Write-Success "Добавлен в '$g'"
            return
        } catch {}
    }
}

function Add-ToEventLogReaders {
    param($Name)

    if ($DryRun) {
        Write-DryRun "Будет добавлен в Event Log Readers"
        return
    }

    Write-Info "Добавляем в Event Log Readers..."
    $groups = @("Event Log Readers","Читатели журнала событий")
    foreach ($g in $groups) {
        try {
            Add-LocalGroupMember -Group $g -Member $Name -ErrorAction Stop
            Write-Success "Добавлен в '$g'"
            return
        } catch {}
    }
}

# -------------------------
# WinRM RootSDDL
# -------------------------
function Add-ToWinRMSDDL {
    param($Account)

    if ($DryRun) {
        Write-DryRun "Будет добавлена ACE в WinRM RootSDDL для $Account"
        Write-DryRun "WinRM будет перезапущен"
        return
    }

    Write-Info "Добавляем ACE в WinRM RootSDDL для $Account..."

    try {
        $nt = New-Object System.Security.Principal.NTAccount($Account)
        $sidObj = $nt.Translate([System.Security.Principal.SecurityIdentifier])
        $sid = $sidObj.Value
    } catch {
        Write-Err "Не удалось получить SID: $_"
        throw
    }

    try {
        $item = Get-Item -Path WSMan:\localhost\Service\RootSDDL -ErrorAction Stop
        $sddl = $item.Value
        if (-not $sddl) { throw "RootSDDL пуст" }

        $bak = Join-Path $env:TEMP "WinRM.RootSDDL.backup.$(Get-Date -Format 'yyyyMMdd_HHmmss').txt"
        $sddl | Out-File -FilePath $bak -Encoding ASCII
        Write-Info "Backup WinRM -> $bak"
    } catch {
        Write-Err "Не удалось прочитать RootSDDL: $_"
        throw
    }

    if ($sddl -match [regex]::Escape($sid)) {
        Write-Info "SID уже в RootSDDL - пропускаем"
        return
    }

    try {
        $sd = New-Object System.Security.AccessControl.CommonSecurityDescriptor($false,$false,$sddl)
        $GENERIC_ALL = 0x10000000
        $sd.DiscretionaryAcl.AddAccess([System.Security.AccessControl.AccessControlType]::Allow, $sidObj, $GENERIC_ALL, [System.Security.AccessControl.InheritanceFlags]::None, [System.Security.AccessControl.PropagationFlags]::None)
        $newSddl = $sd.GetSddlForm([System.Security.AccessControl.AccessControlSections]::All)

        if (-not (Test-SDDLValid $newSddl)) { throw "Новый RootSDDL невалиден" }

        Set-Item -Path WSMan:\localhost\Service\RootSDDL -Value $newSddl -Force
        Write-Success "RootSDDL обновлён"

        Restart-Service -Name WinRM -Force -ErrorAction Stop
        Start-Sleep -Seconds 2
        Write-Success "WinRM перезапущен"
    } catch {
        Write-Err "Ошибка при изменении RootSDDL: $_"
        if (Test-Path $bak) {
            try {
                Set-Item -Path WSMan:\localhost\Service\RootSDDL -Value ((Get-Content $bak) -join "") -Force
                Write-Warn "RootSDDL откатан из backup"
            } catch {}
        }
        throw
    }
}

# -------------------------
# SCM: Service Control Manager
# -------------------------
$SCM_ACE_RIGHTS_READ = "LCRPRC"
$SCM_ACE_RIGHTS_FULL = "CCLCSWRPWPDTLOCRRC"

function Add-ToSCManagerSDDL {
    param(
        $Account,
        [switch]$ReadOnlyAccess
    )

    if ($DryRun) {
        if ($ReadOnlyAccess) {
            Write-DryRun "Будут добавлены READ права на SCM для $Account"
        } else {
            Write-DryRun "Будут добавлены FULL права на SCM для $Account"
        }
        return
    }

    Write-Info "Добавляем права на SCM для $Account..."

    try {
        $nt = New-Object System.Security.Principal.NTAccount($Account)
        $sid = $nt.Translate([System.Security.Principal.SecurityIdentifier]).Value
    } catch {
        Write-Err "Не удалось получить SID: $_"
        throw
    }

    $sddlLines = sc.exe sdshow scmanager 2>&1
    if ($LASTEXITCODE -ne 0) { throw "sc sdshow failed: $($sddlLines -join ' ')" }
    $sddl = ($sddlLines -join "")

    $bakPath = Join-Path $env:TEMP "SCManager.SDDL.backup.$(Get-Date -Format 'yyyyMMdd_HHmmss').txt"
    $sddl | Out-File -FilePath $bakPath -Encoding ASCII
    Write-Info "Backup SCM -> $bakPath"

    if ($sddl -match [regex]::Escape($sid)) {
        Write-Info "SID уже в SCM - пропускаем"
        return
    }

    if ($ReadOnlyAccess) {
        $rights = $SCM_ACE_RIGHTS_READ
        Write-Info "Режим: только просмотр служб (LC+RP+RC)"
    } else {
        $rights = $SCM_ACE_RIGHTS_FULL
        Write-Info "Режим: полное управление службами"
    }

    $ace = "(A;;$rights;;;$sid)"

    $m = [regex]::Match($sddl,'(?s)D:(.*?)S:')
    if ($m.Success) {
        $dacl = $m.Groups[1].Value
        $needle = "D:${dacl}S:"
        $newSddl = $sddl.Replace($needle, "D:${dacl}${ace}S:")

        if ($newSddl -eq $sddl) {
            Write-Err "Replace не сработал - неожиданная структура SDDL"
            throw "SDDL modification failed"
        }
    } else {
        $newSddl = $sddl + $ace
    }

    if (-not (Test-SDDLValid $newSddl)) {
        Write-Err "Новый SCM SDDL невалиден - отмена"
        return
    }

    $setOut = sc.exe sdset scmanager "$newSddl" 2>&1
    if ($LASTEXITCODE -ne 0) {
        Write-Err "sc sdset failed: $($setOut -join ' ') - откат"
        sc.exe sdset scmanager ((Get-Content $bakPath) -join "") | Out-Null
        throw "Не удалось записать SCM SDDL"
    }

    Write-Success "SCM: ACE добавлено ($ace)"
}

function Remove-FromSCManagerSDDL {
    param($Account)

    if ($DryRun) {
        Write-DryRun "Будет удалена ACE из SCM для $Account"
        return $true
    }

    Write-Info "Удаляем ACE из SCM..."

    try {
        $nt = New-Object System.Security.Principal.NTAccount($Account)
        $sid = $nt.Translate([System.Security.Principal.SecurityIdentifier]).Value
    } catch {
        Write-Err "Не удалось получить SID: $_"
        return $false
    }

    $sddlLines = sc.exe sdshow scmanager 2>&1
    if ($LASTEXITCODE -ne 0) { Write-Err "sc sdshow failed"; return $false }
    $sddl = ($sddlLines -join "")

    $bakPath = Join-Path $env:TEMP "SCManager.SDDL.backup.$(Get-Date -Format 'yyyyMMdd_HHmmss').txt"
    $sddl | Out-File -FilePath $bakPath -Encoding ASCII
    Write-Info "Backup SCM -> $bakPath"

    $aceRead = "(A;;$SCM_ACE_RIGHTS_READ;;;$sid)"
    $aceFull = "(A;;$SCM_ACE_RIGHTS_FULL;;;$sid)"

    $escapedRead = [regex]::Escape($aceRead)
    $escapedFull = [regex]::Escape($aceFull)

    $found = $false
    $new = $sddl

    if ([regex]::IsMatch($sddl, $escapedRead)) {
        $new = [regex]::Replace($new, $escapedRead, "")
        $found = $true
        Write-Info "Удалена ACE (ReadOnly)"
    }

    if ([regex]::IsMatch($sddl, $escapedFull)) {
        $new = [regex]::Replace($new, $escapedFull, "")
        $found = $true
        Write-Info "Удалена ACE (Full)"
    }

    if (-not $found) {
        Write-Info "ACE не найдена в SCM - пропускаем"
        return $false
    }

    if (-not (Test-SDDLValid $new)) {
        Write-Err "Результирующий SCM SDDL невалиден - откат"
        sc.exe sdset scmanager ((Get-Content $bakPath) -join "") | Out-Null
        return $false
    }

    $setOut = sc.exe sdset scmanager "$new" 2>&1
    if ($LASTEXITCODE -ne 0) {
        Write-Err "sc sdset failed - откат"
        sc.exe sdset scmanager ((Get-Content $bakPath) -join "") | Out-Null
        return $false
    }

    Write-Success "SCM: ACE удалено"
    return $true
}

# -------------------------
# Добавление READ прав на ВСЕ службы
# -------------------------
function Grant-ReadAccessToAllServices {
    param([string]$UserName)

    if ($DryRun) {
        Write-DryRun "Будет выдан READ доступ ко ВСЕМ службам для $UserName"
        return
    }

    Write-Info "Даём READ доступ ко ВСЕМ службам..."

    try {
        if ($UserName -notmatch '\\') {
            $computer = (Get-CimInstance Win32_ComputerSystem).Name
            $acctFull = "$computer\$UserName"
        } else {
            $acctFull = $UserName
        }

        $nt = New-Object System.Security.Principal.NTAccount($acctFull)
        $sid = $nt.Translate([System.Security.Principal.SecurityIdentifier]).Value
    } catch {
        Write-Err "Не удалось получить SID: $_"
        return
    }

    $readRights = "LCLORCRC"
    $allServices = Get-Service -ErrorAction SilentlyContinue

    $granted = 0
    $skipped = 0
    $failed = 0

    Write-Info "Обрабатываем $($allServices.Count) служб..."

    foreach ($svc in $allServices) {
        $svcName = $svc.Name

        try {
            $sddlLines = sc.exe sdshow $svcName 2>&1
            if ($LASTEXITCODE -ne 0) {
                $failed++
                continue
            }
            $sddl = ($sddlLines -join "")

            if ($sddl -match [regex]::Escape($sid)) {
                $skipped++
                continue
            }

            $readAce = "(A;;$readRights;;;$sid)"

            $m = [regex]::Match($sddl, '(?s)D:(.*?)S:')
            if ($m.Success) {
                $dacl = $m.Groups[1].Value
                $needle = "D:${dacl}S:"
                $newSddl = $sddl.Replace($needle, "D:${dacl}${readAce}S:")

                if ($newSddl -eq $sddl) {
                    $failed++
                    continue
                }
            } else {
                $newSddl = $sddl + $readAce
            }

            if (-not (Test-SDDLValid $newSddl)) {
                $failed++
                continue
            }

            sc.exe sdset $svcName "$newSddl" 2>&1 | Out-Null
            if ($LASTEXITCODE -eq 0) {
                $granted++
            } else {
                $failed++
            }

        } catch {
            $failed++
        }

        if (($granted + $skipped + $failed) % 50 -eq 0) {
            Write-Host "." -NoNewline
        }
    }

    Write-Host ""
    Write-Success "READ доступ: $granted служб добавлено"
    if ($skipped -gt 0) { Write-Info "Пропущено (уже есть): $skipped" }
    if ($failed -gt 0) { Write-Warn "Не удалось: $failed" }
}

# -------------------------
# Службы: добавление и удаление прав
# -------------------------
$SERVICE_ACE_RIGHTS = "CCLCSWRPWPDTLOCRRC"

function Add-ServicePermissions {
    param(
        [string]$ServiceName,
        [string]$Account,
        [string]$Rights = $SERVICE_ACE_RIGHTS
    )

    if ($ServiceName -and ($CRITICAL_SERVICES -contains $ServiceName.ToLower()) -and -not $Force) {
        Write-Warn "Пропущено: $ServiceName (критическая служба)"
        return "Blocked"
    }

    $svc = Get-Service -Name $ServiceName -ErrorAction SilentlyContinue
    if (-not $svc) {
        Write-Warn "Служба '$ServiceName' не найдена"
        return "NotFound"
    }

    if ($DryRun) {
        Write-DryRun "Будут добавлены права на '$ServiceName' для $Account"
        return "DryRun"
    }

    Write-Info "Добавляем права на '$ServiceName'..."

    try {
        if ($Account -notmatch '\\') {
            $computer = (Get-CimInstance Win32_ComputerSystem).Name
            $acctFull = "$computer\$Account"
        } else {
            $acctFull = $Account
        }

        $nt = New-Object System.Security.Principal.NTAccount($acctFull)
        $sid = $nt.Translate([System.Security.Principal.SecurityIdentifier]).Value
    } catch {
        Write-Err "Не удалось получить SID: $_"
        return "Failed"
    }

    $sddlLines = sc.exe sdshow $ServiceName 2>&1
    if ($LASTEXITCODE -ne 0) {
        Write-Err "sc sdshow failed"
        return "Failed"
    }
    $sddl = ($sddlLines -join "")

    $bakPath = Join-Path $env:TEMP "$ServiceName.SDDL.backup.$(Get-Date -Format 'yyyyMMdd_HHmmss').txt"
    $sddl | Out-File -FilePath $bakPath -Encoding ASCII
    Write-Info "Backup $ServiceName -> $bakPath"

    $fullAce = "(A;;$Rights;;;$sid)"
    $escapedFullAce = [regex]::Escape($fullAce)

    if ($sddl -match $escapedFullAce) {
        Write-Info "FULL права уже есть - пропускаем"
        return "Skipped"
    }

    $readAce = "(A;;LCRPRC;;;$sid)"
    $escapedReadAce = [regex]::Escape($readAce)

    if ($sddl -match $escapedReadAce) {
        Write-Info "Найдены READ права, обновляем до FULL..."
        $sddl = [regex]::Replace($sddl, $escapedReadAce, "")
    }

    $ace = "(A;;$Rights;;;$sid)"

    $m = [regex]::Match($sddl,'(?s)D:(.*?)S:')
    if ($m.Success) {
        $dacl = $m.Groups[1].Value
        $needle = "D:${dacl}S:"
        $newSddl = $sddl.Replace($needle, "D:${dacl}${ace}S:")

        if ($newSddl -eq $sddl) {
            Write-Err "Replace не сработал - неожиданная структура SDDL"
            return "Failed"
        }
    } else {
        $newSddl = $sddl + $ace
    }

    if (-not (Test-SDDLValid $newSddl)) {
        Write-Err "Новый SDDL невалиден - откат"
        sc.exe sdset $ServiceName ((Get-Content $bakPath) -join "") | Out-Null
        return "Failed"
    }

    sc.exe sdset $ServiceName "$newSddl" 2>&1 | Out-Null
    if ($LASTEXITCODE -ne 0) {
        Write-Err "sc sdset failed - откат"
        sc.exe sdset $ServiceName ((Get-Content $bakPath) -join "") | Out-Null
        return "Failed"
    }

    Write-Success "Права добавлены/обновлены: $ServiceName"
    return "Updated"
}

function Remove-ServicePermissions {
    param(
        [string]$ServiceName,
        [string]$Account,
        [string]$Rights = $SERVICE_ACE_RIGHTS
    )

    if ($ServiceName -and ($CRITICAL_SERVICES -contains $ServiceName.ToLower()) -and -not $Force) {
        Write-Warn "Пропущено: $ServiceName (критическая служба)"
        return "Blocked"
    }

    $svc = Get-Service -Name $ServiceName -ErrorAction SilentlyContinue
    if (-not $svc) {
        Write-Warn "Служба '$ServiceName' не найдена"
        return "NotFound"
    }

    if ($DryRun) {
        Write-DryRun "Будут удалены права на '$ServiceName' для $Account"
        return "DryRun"
    }

    Write-Info "Удаляем ACE из '$ServiceName'..."

    try {
        if ($Account -notmatch '\\') {
            $computer = (Get-CimInstance Win32_ComputerSystem).Name
            $acctFull = "$computer\$Account"
        } else {
            $acctFull = $Account
        }

        $nt = New-Object System.Security.Principal.NTAccount($acctFull)
        $sid = $nt.Translate([System.Security.Principal.SecurityIdentifier]).Value
    } catch {
        Write-Err "Не удалось получить SID: $_"
        return "Failed"
    }

    $sddlLines = sc.exe sdshow $ServiceName 2>&1
    if ($LASTEXITCODE -ne 0) {
        Write-Err "sc sdshow failed"
        return "Failed"
    }
    $sddl = ($sddlLines -join "")

    $bakPath = Join-Path $env:TEMP "$ServiceName.SDDL.backup.$(Get-Date -Format 'yyyyMMdd_HHmmss').txt"
    $sddl | Out-File -FilePath $bakPath -Encoding ASCII
    Write-Info "Backup $ServiceName -> $bakPath"

    $exactAce = "(A;;$Rights;;;$sid)"
    $escaped = [regex]::Escape($exactAce)

    if (-not ([regex]::IsMatch($sddl, $escaped))) {
        Write-Info "Точная ACE не найдена - пропускаем"
        return "Skipped"
    }

    $new = [regex]::Replace($sddl, $escaped, "")

    if (-not (Test-SDDLValid $new)) {
        Write-Err "Результирующий SDDL невалиден - откат"
        sc.exe sdset $ServiceName ((Get-Content $bakPath) -join "") | Out-Null
        return "Failed"
    }

    sc.exe sdset $ServiceName "$new" 2>&1 | Out-Null
    if ($LASTEXITCODE -ne 0) {
        Write-Err "sc sdset failed - откат"
        sc.exe sdset $ServiceName ((Get-Content $bakPath) -join "") | Out-Null
        return "Failed"
    }

    Write-Success "ACE удалено: $ServiceName"
    return "Removed"
}

# -------------------------
# Добавление прав на реестр службы
# -------------------------
function Add-ServiceRegistryPermissions {
    param([string]$ServiceName, [string]$UserName)

    if ($DryRun) {
        Write-DryRun "Будут добавлены права на реестр службы: $ServiceName"
        return
    }

    $keyPath = "HKLM:\SYSTEM\CurrentControlSet\Services\$ServiceName"

    if (-not (Test-Path $keyPath)) {
        Write-Warn "Ключ реестра $keyPath не найден"
        return
    }

    try {
        $acl = Get-Acl $keyPath
        $rule = New-Object System.Security.AccessControl.RegistryAccessRule(
            $UserName, "ReadKey",
            "ContainerInherit,ObjectInherit", "None", "Allow"
        )
        $acl.AddAccessRule($rule)
        Set-Acl $keyPath $acl
        Write-Info "Реестр ${ServiceName}: права добавлены"
    } catch {
        Write-Warn "Реестр ${ServiceName}: $_"
    }
}

# -------------------------
# ГЛАВНЫЙ БЛОК ВЫПОЛНЕНИЯ
# -------------------------
$report = [ordered]@{
    UpdatedServices = @()
    SkippedServices = @()
    BlockedServices = @()
    NotFoundServices = @()
    FailedServices = @()
    RemovedServices = @()
    SCM_Removed = $false
    SCM_Added = $false
    WinRM_Updated = $false
    ReadOnlyMode = $ReadOnly
    AllServicesReadGranted = $false
    UserDeleted = $false
    DryRun = $DryRun
}

try {
    Write-Host "`n========================================" -ForegroundColor Cyan
    Write-Host "  Режим: $Mode | Пользователь: $UserName" -ForegroundColor Cyan
    if ($Mode -ne "DeleteUser") {
        if ($ReadOnly) {
            Write-Host "  Уровень: Только просмотр служб" -ForegroundColor Yellow
        } else {
            Write-Host "  Уровень: Полное управление" -ForegroundColor Green
        }
    }
    if ($DryRun) {
        Write-Host "   DRY-RUN: Тестовый режим (без изменений)" -ForegroundColor Magenta
    }
    Write-Host "========================================`n" -ForegroundColor Cyan

    # === ПОЛНЫЙ БЭКАП ПЕРЕД НАЧАЛОМ ===
    if ($Mode -ne "Rollback" -and $Mode -ne "DeleteUser" -and -not $DryRun) {
        Write-Host ""
        $createBackup = Read-Host "Создать полный бэкап ВСЕХ служб? (рекомендуется) (Y/N)"
        if ($createBackup -in @('Y','y')) {
            $global:FullBackupPath = Backup-AllServicesSDDL
        }
    }

    # === РЕЖИМ УДАЛЕНИЯ ПОЛЬЗОВАТЕЛЯ ===
    if ($Mode -eq "DeleteUser") {
        $result = Remove-LocalUserComplete -Name $UserName
        if ($result) {
            $report.UserDeleted = $true
            Write-Host "`n========================================" -ForegroundColor Green
            Write-Host "  УДАЛЕНИЕ ЗАВЕРШЕНО" -ForegroundColor Green
            Write-Host "========================================" -ForegroundColor Green
            Write-Success "Пользователь '$UserName' полностью удалён"
        } else {
            Write-Host "`n========================================" -ForegroundColor Red
            Write-Host "  УДАЛЕНИЕ НЕ ВЫПОЛНЕНО" -ForegroundColor Red
            Write-Host "========================================" -ForegroundColor Red
        }

        exit 0
    }

    if ($Mode -eq "Create") {
        Ensure-LocalUser -Name $UserName -PlainPassword $Password
        Add-ToRemoteManagementGroup -Name $UserName
    }

    $computer = (Get-CimInstance Win32_ComputerSystem).Name
    $acctFull = "$computer\$UserName"

    # === ПРОВЕРКА СУЩЕСТВОВАНИЯ ПОЛЬЗОВАТЕЛЯ ===
    if ($Mode -ne "Rollback" -and -not $DryRun) {
        Write-Host "`n[Проверка] Существование пользователя..." -ForegroundColor Cyan
        try {
            $userExists = $false

            if (Get-Command -Name Get-LocalUser -ErrorAction SilentlyContinue) {
                $userExists = (Get-LocalUser -Name $UserName -ErrorAction SilentlyContinue) -ne $null
            }

            if (-not $userExists) {
                try {
                    $userPath = "WinNT://$computer/$UserName,user"
                    $user = [ADSI]$userPath
                    $userExists = ($user -and $user.psbase -and $user.Name)
                } catch {
                    $userExists = $false
                }
            }

            if (-not $userExists) {
                Write-Err "Пользователь '$UserName' не найден на компьютере '$computer'!"
                Write-Err "Для режима GrantOnly пользователь должен существовать."
                Write-Host "`nВарианты решения:" -ForegroundColor Yellow
                Write-Host "  1. Используйте режим Create для создания нового пользователя"
                Write-Host "  2. Проверьте правильность имени пользователя"
                Write-Host "  3. Создайте пользователя вручную: net user $UserName <password> /add"

                Stop-ScriptLogging
                exit 1
            }

            Write-Success "Пользователь '$UserName' найден"
        } catch {
            Write-Err "Ошибка при проверке пользователя: $_"
            Stop-ScriptLogging
            exit 1
        }
    }

    if ($Mode -ne "Rollback") {
        try {
            Add-ToWinRMSDDL -Account $acctFull
            $report.WinRM_Updated = $true
        } catch {
            Write-Warn "WinRM: $_"
        }

        try { Add-ToDcomUsers -Name $UserName } catch {}
        try { Add-ToEventLogReaders -Name $UserName } catch {}

        try {
            if ($ReadOnly) {
                Add-ToSCManagerSDDL -Account $acctFull -ReadOnlyAccess
            } else {
                Add-ToSCManagerSDDL -Account $acctFull
            }
            $report.SCM_Added = $true
        } catch {
            Write-Warn "SCM: $_"
        }

        if ($Services -and $Services.Count -gt 0) {
            Write-Host "`n[Шаг 4/6] Управление конкретными службами..." -ForegroundColor Cyan
            foreach ($svc in $Services) {
                try {
                    $res = Add-ServicePermissions -ServiceName $svc -Account $acctFull -Rights $SERVICE_ACE_RIGHTS

                    if ($res -eq "Updated" -and -not $DryRun) {
                        Add-ServiceRegistryPermissions -ServiceName $svc -UserName $UserName
                    }

                    switch ($res) {
                        "Updated"    { $report.UpdatedServices += $svc }
                        "Skipped"    { $report.SkippedServices += $svc }
                        "Blocked"    { $report.BlockedServices += $svc }
                        "NotFound"   { $report.NotFoundServices += $svc }
                        "DryRun"     { $report.UpdatedServices += $svc }
                        default      { $report.FailedServices += $svc }
                    }
                } catch {
                    $errMsg = $_.Exception.Message
                    Write-Err "Ошибка при обработке ${svc} - $errMsg"
                    $report.FailedServices += $svc
                }
            }
        }

        Write-Host "`n[Шаг 5/6] READ доступ на ВСЕ службы..." -ForegroundColor Cyan
        try {
            Grant-ReadAccessToAllServices -UserName $UserName
            $report.AllServicesReadGranted = $true
        } catch {
            Write-Warn "Ошибка при установке READ прав: $_"
        }

        if (-not $Services -or $Services.Count -eq 0) {
            Write-Info "Конкретные службы не указаны - выданы только права SCM и READ на все службы"
        }

    } else {
        if ($Services -and $Services.Count -gt 0) {
            foreach ($svc in $Services) {
                try {
                    $res = Remove-ServicePermissions -ServiceName $svc -Account $acctFull -Rights $SERVICE_ACE_RIGHTS
                    switch ($res) {
                        "Removed"    { $report.RemovedServices += $svc }
                        "Skipped"    { $report.SkippedServices += $svc }
                        "Blocked"    { $report.BlockedServices += $svc }
                        "NotFound"   { $report.NotFoundServices += $svc }
                        "DryRun"     { $report.RemovedServices += $svc }
                        default      { $report.FailedServices += $svc }
                    }
                } catch {
                    $errMsg = $_.Exception.Message
                    Write-Err "Ошибка при удалении прав на ${svc} - $errMsg"
                    $report.FailedServices += $svc
                }
            }
        }

        try {
            $scRes = Remove-FromSCManagerSDDL -Account $acctFull
            if ($scRes) { $report.SCM_Removed = $true }
        } catch {
            Write-Warn "SCM rollback: $_"
        }
    }

    Write-Host "`n========================================" -ForegroundColor Green
    Write-Host "  ОТЧЁТ О ВЫПОЛНЕНИИ" -ForegroundColor Green
    Write-Host "========================================" -ForegroundColor Green

    if ($DryRun) {
        Write-Host "`n ТЕСТОВЫЙ РЕЖИМ - никакие изменения не были применены!" -ForegroundColor Magenta
    }

    if ($report.UpdatedServices.Count -gt 0) {
        $action = if ($DryRun) { "Будут обновлены службы" } else { "Обновлено служб" }
        Write-Host "`n$action : $($report.UpdatedServices.Count)" -ForegroundColor Green
        $report.UpdatedServices | ForEach-Object { Write-Host "  - $_" -ForegroundColor Green }
    }

    if ($report.RemovedServices.Count -gt 0) {
        $action = if ($DryRun) { "Будут удалены ACE из служб" } else { "Удалено ACE из служб" }
        Write-Host "`n$action : $($report.RemovedServices.Count)" -ForegroundColor Cyan
        $report.RemovedServices | ForEach-Object { Write-Host "  - $_" -ForegroundColor Cyan }
    }

    if ($report.SkippedServices.Count -gt 0) {
        Write-Host "`nПропущено (уже настроено): $($report.SkippedServices.Count)" -ForegroundColor Yellow
        $report.SkippedServices | ForEach-Object { Write-Host "  - $_" }
    }

    if ($report.BlockedServices.Count -gt 0) {
        Write-Host "`nЗаблокировано (критические): $($report.BlockedServices.Count)" -ForegroundColor Red
        $report.BlockedServices | ForEach-Object { Write-Host "  - $_" -ForegroundColor Red }
    }

    if ($report.NotFoundServices.Count -gt 0) {
        Write-Host "`nНе найдено: $($report.NotFoundServices.Count)" -ForegroundColor Yellow
        $report.NotFoundServices | ForEach-Object { Write-Host "  - $_" }
    }

    if ($report.FailedServices.Count -gt 0) {
        Write-Host "`nОшибки: $($report.FailedServices.Count)" -ForegroundColor Red
        $report.FailedServices | ForEach-Object { Write-Host "  - $_" -ForegroundColor Red }
    }

    Write-Host ""
    if ($report.WinRM_Updated) {
        if ($DryRun) {
            Write-Host "WinRM RootSDDL будет обновлён" -ForegroundColor Magenta
        } else {
            Write-Host "WinRM RootSDDL обновлён" -ForegroundColor Green
        }
    }
    if ($report.SCM_Added) {
        if ($report.ReadOnlyMode) {
            $msg = "SCM права добавлены (READ-ONLY: просмотр всех служб)"
        } else {
            $msg = "SCM права добавлены (FULL: управление службами)"
        }
        if ($DryRun) {
            Write-Host $msg -ForegroundColor Magenta
        } else {
            Write-Host $msg -ForegroundColor Green
        }
    }
    if ($report.AllServicesReadGranted) {
        if ($DryRun) {
            Write-Host "READ доступ ко ВСЕМ службам будет установлен" -ForegroundColor Magenta
        } else {
            Write-Host "READ доступ ко ВСЕМ службам установлен" -ForegroundColor Green
        }
    }
    if ($report.SCM_Removed) {
        if ($DryRun) {
            Write-Host "SCM права будут удалены" -ForegroundColor Magenta
        } else {
            Write-Host "SCM права удалены" -ForegroundColor Cyan
        }
    }

    if (-not $DryRun) {
        Write-Host "`nБэкапы SDDL сохранены в: $env:TEMP" -ForegroundColor Cyan

        if ($global:FullBackupPath) {
            Write-Host "Полный бэкап всех служб: $global:FullBackupPath" -ForegroundColor Cyan
        }
    }

    if ($DryRun) {
        Write-Host "`n========================================" -ForegroundColor Magenta
        Write-Host "  Для реального выполнения запустите без -DryRun" -ForegroundColor Magenta
        Write-Host "========================================" -ForegroundColor Magenta
    }

    Write-Host "========================================" -ForegroundColor Green
    Write-Success "ГОТОВО!"
    Write-Host "========================================`n" -ForegroundColor Green
}
catch {
    Write-Err "Критическая ошибка: $_"
    Write-Host "`nПроверьте бэкапы в $env:TEMP и восстановите систему при необходимости." -ForegroundColor Yellow

    if ($global:FullBackupPath) {
        Write-Host "Полный бэкап: $global:FullBackupPath" -ForegroundColor Cyan
    }

    Stop-ScriptLogging
    exit 1
}
finally {
    Stop-ScriptLogging
}
