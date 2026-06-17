# NOVUS Installer

`novus-installer` — standalone Go binary для первичного bootstrap и post-install оркестрации NOVUS-OS на bare-metal или VPS-хосте.

## Установка одной командой (публичный репозиторий)

```bash
curl -fsSL https://raw.githubusercontent.com/SGC-NOVUS/installer/main/install.sh | sudo bash
```

GitHub PAT **не нужен** для скачивания установщика. PAT потребуется позже — веб-интерфейс запросит его для доступа к приватному `SGC-NOVUS/panel-core`.

Можно передать PAT заранее через переменную окружения:

```bash
NOVUS_INSTALLER_GITHUB_PAT="github_pat_..." sudo -E bash install.sh
```

Или вручную:

```bash
go install github.com/SGC-NOVUS/novus-installer/cmd/installer@latest
sudo novus-installer
```

При установке запросит `GitHub PAT` для доступа к приватному репозиторию `SGC-NOVUS/panel-core`.

## Быстрый старт

```bash
# Dry-run проверка
sudo ./novus-installer --dev --dry-run

# Боевая установка (запросит github_pat в веб-интерфейсе)
sudo ./novus-installer

# С явным PAT через env
NOVUS_INSTALLER_GITHUB_PAT="github_pat_..." sudo -E ./novus-installer
```

## Переменные окружения

| Переменная | Назначение | По умолчанию |
|-----------|----------|-------------|
| `NOVUS_INSTALLER_GITHUB_PAT` | GitHub PAT для скачивания панели | (запрашивается в UI) |
| `NOVUS_INSTALLER_PANEL_RELEASE_URL` | URL архива панели | `api.github.com/repos/SGC-NOVUS/panel-core/zipball/main` |
| `NOVUS_INSTALLER_PANEL_CORE_REF` | Ветка/тег panel-core | `main` |
| `NOVUS_INSTALLER_AGENT_BINARY_URL` | URL бинарника агента | `releases/latest/download/novus-agent-linux-amd64` |

## Возможности

- Встроенный HTTP-интерфейс без внешнего frontend build-step: UI вшит в бинарник через `go:embed`.
- Токенизированный вход: первая загрузка по `/?token=...`, далее сессия закрепляется через `HttpOnly` cookie.
- Полноэкранный enterprise wizard с ручным переключением языка/темы, live restore flow, Security Entrance, KMS backend choice и catalog-based integrations surface.
- Два канала данных в WebSocket:
  - text frames: JSON-статусы `step`, `error`, `finish`;
  - binary frames: сырой PTY-вывод для xterm.js.
- Режим `--dev` для ослабления preflight checks на занятом или неидеальном хосте.
- Режим `--dry-run` для безопасной демонстрации полного install pipeline без изменений на сервере.
- Laravel bridge layer: генерация `.env`, `artisan migrate --force`, `artisan novus:setup-foundation`.
- Canonical bootstrap manifest в `/etc/novus/manifest.json`.
- Финальный локальный health check через `Host`-aware HTTP probe.
- Самоуничтожение бинарника после успешной установки в production-режиме.

## Требования

- Linux с root-доступом.
- Поддерживаемые OS для strict preflight: Ubuntu 22.04, Ubuntu 24.04, Debian 12.
- Минимум 2 GiB RAM.
- Для живой установки должны быть доступны `apt`, `systemctl`, `nginx`, `mariadb`, `php8.5`, `certbot`.
- Для production-сценария рекомендуется собранный бинарник, а не `go run`, чтобы self-destruct удалял именно установленный артефакт.

## Сборка и запуск

Сборка бинарника:

```bash
cd /www/wwwroot/novus-installer
go build -o novus-installer ./cmd/installer
```

Запуск в production-режиме:

```bash
sudo ./novus-installer
```

Запуск в relaxed preflight-режиме:

```bash
sudo ./novus-installer --dev
```

Безопасный полный прогон без изменений на хосте:

```bash
sudo ./novus-installer --dev --dry-run
```

Локальный запуск напрямую из исходников:

```bash
sudo go run cmd/installer/main.go --dev --dry-run
```

После старта бинарник печатает одноразовую ссылку вида `http://<host>:8080/?token=<token>`. Откройте её в браузере, заполните форму bootstrap и следите за install stream в терминальном окне UI.

## CLI-флаги

- `--dev`: переводит OS, RAM и port preflight-проблемы в warnings. Root-check остаётся обязательным.
- `--dry-run`: не выполняет реальные shell/local actions. Вместо этого в terminal stream отправляются строки вида `[DRY-RUN] Would execute: ...` с короткой паузой между шагами.

## Release Sources

По умолчанию инсталлятор использует следующие release sources:

- Panel archive: `https://github.com/SGC-NOVUS/panel/releases/latest/download/panel.zip`
- Agent binary для `amd64`: `https://github.com/SGC-NOVUS/agent/releases/latest/download/novus-agent-linux-amd64`
- Agent binary для `arm64`: `https://github.com/SGC-NOVUS/agent/releases/latest/download/novus-agent-linux-arm64`

Их можно переопределить переменными окружения:

- `NOVUS_INSTALLER_PANEL_RELEASE_URL`
- `NOVUS_INSTALLER_AGENT_BINARY_URL`

Логика загрузки:

- инсталлятор использует `curl -fL`, поэтому GitHub Latest redirect (`302 Found`) обрабатывается автоматически;
- для агента URL выбирается динамически по `runtime.GOARCH`, если не задан `NOVUS_INSTALLER_AGENT_BINARY_URL`.

Защита скачивания:

- panel archive скачивается в `/tmp/panel.zip`, проходит проверку на ненулевой размер, минимальный размер и `unzip -tq` до распаковки;
- agent binary скачивается в `/tmp/novus-agent`, проходит size-check перед установкой в `/usr/local/bin/novus-agent`.

## Install Pipeline

Инсталлятор выполняет следующие шаги:

1. Системные зависимости.
2. Репозитории PHP 8.5 и MariaDB.
3. Установка стека Nginx, MariaDB и PHP 8.5.
4. Настройка MariaDB и создание `novus_os`, `novus_id`, `novus_sd`.
5. Установка `novus-agent`.
6. Конфигурация Nginx и SSL.
7. Развертывание кода панели в `/var/www/novus`.
8. Генерация секретов и `/var/www/novus/.env`.
9. Laravel bridge layer: `php artisan migrate --force` и `php artisan novus:setup-foundation`.
10. Запись canonical manifest в `/etc/novus/manifest.json`.
11. Post-install health check локальным HTTP GET на `127.0.0.1` с заголовком `Host`.

## HTTP и WebSocket API

`GET /?token=<token>`

- устанавливает `HttpOnly` cookie `novus_installer_token`;
- после этого UI и API работают только через cookie-сессию.

`POST /api/setup`

- принимает расширенный JSON-контракт bootstrap, включая базовые поля `Domain`, `AdminEmail`, `AdminPassword`, `DBRootPassword`, `DBPanelPassword`, а также `Restore`, `SecurityEntrance`, `MasterKeyBackend`, `CloudflareKMS` и `Integrations`;
- при успехе возвращает `{"status":"installing"}` и запускает install pipeline асинхронно.

`GET /api/stream`

- WebSocket endpoint для live install stream.

Text frame example:

```json
{"type":"step","text":"Проверка работоспособности системы"}
```

Finish frame example:

```json
{"type":"finish","text":"Установка успешно завершена","url":"https://panel.example.com"}
```

Binary frames содержат PTY output или dry-run terminal lines.

## Post-Install Health Check

После завершения install steps инсталлятор выполняет локальный HTTP GET на `http://127.0.0.1/` с заголовком `Host: <domain>`. Успешными считаются статусы:

- `200`
- `301`
- `302`
- `401`

Ошибкой считаются:

- `500`
- `502`
- таймаут соединения
- любой другой неожиданный HTTP status

При ошибке health check frontend получает JSON-статус `error`, а финализация прерывается.

## Self-Destruct

После успешной установки и отправки финального `finish`-события инсталлятор пытается удалить собственный исполняемый файл через `os.Executable()` и `os.Remove()`.

Self-destruct включается только если одновременно выполняются оба условия:

- не используется `--dev`;
- не используется `--dry-run`.

Это снижает риск оставить bootstrap surface на production-сервере после завершения установки.

## Полезные пути

- Panel root: `/var/www/novus`
- Panel public root: `/var/www/novus/public`
- Panel env file: `/var/www/novus/.env`
- Nginx site: `/etc/nginx/sites-available/novus-installer.conf`
- Canonical manifest: `/etc/novus/manifest.json`
- Agent binary: `/usr/local/bin/novus-agent`

## Тестирование

Полная проверка Go-модуля:

```bash
cd /www/wwwroot/novus-installer
go test ./...
```

Ключевой smoke-test WebSocket dry-run контракта:

```bash
go test ./internal/web -run TestDryRunWebSocketSmoke -v
```

Этот тест фиксирует:

- выдачу cookie-сессии через tokenized root;
- успешный запуск `/api/setup`;
- получение text-status frames;
- получение binary PTY/dry-run frames;
- наличие финального health-check шага и `finish`-события.

## Диагностика

- `403 forbidden` на root или API почти всегда означает отсутствие token или cookie-сессии.
- Если `--dev` не включён, занятые `80/443`, неподдерживаемая OS или недостаток RAM остановят запуск на preflight.
- Если pipeline падает на health check, сначала проверьте `nginx -t`, состояние `php8.5-fpm` и локальный ответ `curl -H 'Host: <domain>' http://127.0.0.1/`.
- Для безопасной отладки installer-логики используйте только `--dev --dry-run`.