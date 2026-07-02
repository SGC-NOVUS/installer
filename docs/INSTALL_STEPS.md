# NOVUS Installer — Шаги установки

**Версия:** 1.0.0

---

## Обзор

Ниже приведено детальное описание каждого шага установки, выполняемого оркестратором.

---

## Шаг 1: Очистка кэша APT

Удаление несовместимых репозиториев (ondrej/php, sury-php, mariadb) и обновление кэша APT.

```bash
rm -f /etc/apt/sources.list.d/ondrej*.list /etc/apt/sources.list.d/sury-php.list
apt-get update -qq
```

---

## Шаг 2: Системные зависимости

Установка базовых системных пакетов:
```bash
apt-get install -y curl wget git unzip jq openssl ca-certificates gnupg lsb-release
```

---

## Шаг 3: Репозитории PHP 8.5 и MariaDB

Добавление официальных репозиториев:
- **PHP 8.5:** PPA ondrej/php (Ubuntu) или sury.org (Debian)
- **MariaDB:** официальный репозиторий MariaDB

---

## Шаг 4: Установка стека

Полная установка всех компонентов:

| Категория | Пакеты |
|-----------|--------|
| **Web-сервер** | nginx |
| **БД** | mariadb-server |
| **PHP 8.5** | php8.5-fpm, php8.5-cli, php8.5-mysql, php8.5-mbstring, php8.5-xml, php8.5-curl, php8.5-zip, php8.5-bcmath, php8.5-intl, php8.5-gd, php8.5-redis, php8.5-grpc |
| **Сервисы** | redis-server, supervisor, influxdb2, fail2ban, ufw |
| **SSL** | certbot, python3-certbot-nginx, python3-certbot-dns-cloudflare |
| **Утилиты** | curl, wget, git, unzip, rsync, jq, openssh-server |

После установки:
- `systemctl enable --now nginx mariadb php8.5-fpm`
- Создание `/etc/novus/secrets/`
- Настройка php8.5-fpm override (доступ к `/etc/novus/secrets`)

---

## Шаг 5: Настройка MariaDB

Создание системных баз данных и пользователей:

```sql
-- Пользователи
CREATE USER 'novus_panel'@'127.0.0.1' IDENTIFIED BY '<password>';
CREATE USER 'novus_id'@'127.0.0.1' IDENTIFIED BY '<password>';

-- Базы данных
CREATE DATABASE novus_os;
CREATE DATABASE novus_id;
CREATE DATABASE novus_sd;

-- Права
GRANT ALL ON novus_os.* TO 'novus_panel'@'127.0.0.1';
GRANT ALL ON novus_id.* TO 'novus_id'@'127.0.0.1';
GRANT ALL ON novus_sd.* TO 'novus_panel'@'127.0.0.1';
FLUSH PRIVILEGES;
```

**Безопасность:** пароли передаются через `printf` в SQL-файл (не через аргументы командной строки).

---

## Шаг 6: Настройка сервисов

Активация и настройка дополнительных сервисов:

- **Redis:** `systemctl enable --now redis-server`
- **Supervisor:** `systemctl enable --now supervisor`
- **InfluxDB:** `systemctl enable --now influxdb`
- **Fail2Ban:** `systemctl enable --now fail2ban`
- **UFW:** default deny incoming, allow 22, 80, 443, 8080, 9443

---

## Шаг 7: Установка novus-agent

Скачивание и установка Go-агента:

```bash
curl -fL "https://github.com/SGC-NOVUS/agent/releases/latest/download/novus-agent-linux-amd64" \
    -o /tmp/novus-agent
install -m 0755 /tmp/novus-agent /usr/local/bin/novus-agent
```

- Проверка минимального размера бинарника (≥1 KiB)
- Автоматический выбор архитектуры (amd64/arm64)

---

## Шаг 8: Конфигурация Nginx и SSL

Создание виртуального хоста Nginx и настройка SSL:

- **Режимы SSL:** Let's Encrypt, Cloudflare DNS, собственный сертификат, без SSL
- **Шаблон Nginx:** проксирование PHP-FPM, статические файлы, security headers

---

## Шаг 9: Развёртывание панели

Скачивание кода панели из приватного репозитория `SGC-NOVUS/panel-core`:

```bash
curl -H "Authorization: Bearer <PAT>" \
    "https://api.github.com/repos/SGC-NOVUS/panel-core/zipball/main" \
    -o /tmp/panel.zip
unzip /tmp/panel.zip -d /var/www/novus
```

- Требуется GitHub PAT с `repo` доступом
- Проверка минимального размера архива (≥10 KiB)

---

## Шаг 10: Настройка Cron-заданий

Копирование и активация cron-файла панели:

```cron
* * * * * www-data php /var/www/novus/tools/metrics_tick.php
* * * * * www-data php /var/www/novus/tools/monitor_tick.php
* * * * * www-data php /var/www/novus/tools/scheduler_tick.php
* * * * * www-data php /var/www/novus/tools/backup_tick.php
```

---

## Шаг 11: Генерация секретов

Создание криптографических ключей и конфигурации:

- `master.key` — 32 байта через `crypto/rand`, права 0600
- `panel-novus.env` — все переменные окружения панели
- `db_pass`, `id_db_pass`, `payload_db_pass` — пароли БД
- `influxdb_token` — токен InfluxDB (если установлен)

---

## Шаг 12-13: Инициализация БД

- **Bridge Layer:** установка соединений с БД
- **System Bootstrap:** первый HTTP-запрос к панели для автоматического создания схемы

---

## Шаг 14: Настройка администратора

Создание учётной записи администратора панели:

- **Безопасность:** учётные данные передаются через tmpfile (JSON), не через аргументы командной строки
- Создание пользователя с ролью `system_admin`
- Установка пароля через `UsersService::setPassword()`
- Активация учётной записи

---

## Шаг 15: Настройка локального узла

Регистрация локального агента в панели:

- Создание записи в `panel_nodes`
- Установка `connection_type = 'direct'`
- Сопряжение агента с панелью

---

## Шаг 16-18: Дополнительные настройки

- **Интеграции:** Telegram, Discord, Google OAuth, GitHub CLI
- **Cloudflare KMS:** опциональное шифрование через Cloudflare Workers
- **Security Entrance:** защищённый вход (альтернативный URL для панели)

---

## Шаг 19: Восстановление из бекапа (опционально)

- Поддерживаемые источники: URL, файл, inline
- Режимы импорта: overwrite, skip_existing, report_only

---

## Шаг 20: Очистка и Health Check

- Удаление временных файлов
- Health check: HTTP GET на `http://127.0.0.1/`
- Самоуничтожение бинарника инсталлятора (кроме `--dev` режима)
- Запись манифеста в `/etc/novus/manifest.json`
