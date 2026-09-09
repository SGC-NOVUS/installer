# NOVUS Installer — Web API

**Версия:** 0.1.0

---

## 1. Обзор

Веб-интерфейс инсталлятора предоставляет HTTP API и WebSocket для управления процессом установки.

---

## 2. HTTP API

### Базовый URL
```
http://<vps-ip>:8080
```

### Аутентификация

Все запросы требуют токен:
- **Первый запрос:** `?token=<random>` в URL
- **Последующие:** HttpOnly cookie `novus_installer_token`

---

### POST /api/setup

Запуск процесса установки.

**Request Body (JSON):**
```json
{
  "domain": "panel.example.com",
  "admin_email": "admin@example.com",
  "admin_password": "securePassword123",
  "db_root_password": "dbRootPass456",
  "db_panel_password": "dbPanelPass789",
  "ssl_mode": "letsencrypt",
  "cloudflare_api_token": "cf_token_optional",
  "security_entrance": {
    "enabled": false,
    "path": "secure",
    "password": ""
  },
  "integrations": [
    {
      "key": "telegram",
      "enabled": true,
      "fields": {
        "telegram_bot_token": "...",
        "telegram_admin_chat_id": "..."
      }
    }
  ],
  "github_pat": "ghp_optional",
  "restore": {
    "source_type": "",
    "backup_url": "",
    "import_mode": "overwrite"
  },
  "cloudflare_kms": {
    "enabled": false,
    "api_token": ""
  }
}
```

**Валидация:**
- `domain` — DNS-имя или IP-адрес
- `admin_email` — валидный email
- `admin_password` — не пустой
- `db_root_password` — не пустой
- `db_panel_password` — не пустой
- `security_entrance.path` — 3-64 символа, не зарезервирован
- `integrations[].key` — одно из: telegram, discord_notifications, google, github, github_cli

**Response (200):**
```json
{
  "ok": true,
  "data": {
    "session_id": "01901234-5678-7abc-def0-123456789abc",
    "ws_url": "ws://<vps-ip>:8080/ws/stream"
  }
}
```

**Response (400):**
```json
{
  "ok": false,
  "error": {
    "code": "setup_request_invalid",
    "message": "domain_required"
  }
}
```

---

## 3. WebSocket Stream

### Подключение
```
ws://<vps-ip>:8080/ws/stream
```

### Формат сообщений

#### Статус шага
```json
{
  "type": "step_status",
  "step": 4,
  "name": "Установка стека Nginx, MariaDB и PHP 8.5",
  "status": "running"
}
```

#### PTY-вывод
```json
{
  "type": "pty_output",
  "step": 4,
  "data": "Reading package lists...\n"
}
```

#### Прогресс
```json
{
  "type": "progress",
  "step": 4,
  "percent": 45,
  "message": "Installing php8.5-fpm..."
}
```

#### Ошибка
```json
{
  "type": "error",
  "step": 4,
  "code": "command_failed",
  "message": "apt-get install failed with exit code 100"
}
```

#### Завершение
```json
{
  "type": "complete",
  "success": true,
  "manifest": {
    "version": "0.1.0",
    "installed_at": "2026-06-20T10:45:00Z",
    "panel_url": "https://panel.example.com"
  }
}
```

---

## 4. Коды ошибок API

| Код | Описание |
|-----|----------|
| `setup_request_invalid` | Невалидный запрос |
| `domain_required` | Домен не указан |
| `domain_invalid` | Домен содержит недопустимые символы |
| `email_required` | Email не указан |
| `email_invalid` | Невалидный формат email |
| `password_required` | Пароль не указан |
| `db_password_required` | Пароль БД не указан |
| `ssl_cert_invalid` | Невалидный PEM-сертификат |
| `ssl_key_invalid` | Невалидный PEM-ключ |
| `integration_required_fields` | Не заполнены обязательные поля интеграции |
| `security_entrance_path_reserved` | Зарезервированный путь |
| `panel_release_url_missing` | Не указан URL панели или GitHub PAT |
| `install_already_running` | Установка уже выполняется |

---

## 5. CORS

В production режиме:
- `Access-Control-Allow-Origin: <vps-ip>`
- Доступ только с localhost

В dev режиме:
- `Access-Control-Allow-Origin: *`
