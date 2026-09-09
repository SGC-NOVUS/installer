# NOVUS Installer — Стратегия отката

**Версия:** 0.1.0

---

## 1. Обзор

Инсталлятор поддерживает два уровня отката при возникновении ошибок на любом шаге установки. Это позволяет избежать частичного состояния системы (partial state).

---

## 2. Уровни отката

### Soft Rollback (по умолчанию)

Откатывает только созданные конфигурационные файлы и артефакты:

| Действие | Что удаляется |
|----------|--------------|
| Nginx config | `/etc/nginx/sites-available/novus-installer.conf` |
| Nginx symlink | `/etc/nginx/sites-enabled/novus-installer.conf` |
| SSL-сертификаты | `/etc/novus/ssl/` (созданные инсталлятором) |
| Временные файлы | `/tmp/novus-*` |
| Манифест | `/etc/novus/manifest.json` |
| PHP-FPM override | `/etc/systemd/system/php8.5-fpm.service.d/override.conf` |

**Не удаляет:**
- Установленные пакеты (nginx, mariadb, php, etc.)
- Директорию `/var/www/novus/` (может содержать пользовательские данные)
- Базы данных
- Системные сервисы

### Hard Rollback (только `--dev`)

Полный откат системы к состоянию до установки:

| Действие | Команда |
|----------|---------|
| Удаление пакетов | `apt-get purge -y nginx mariadb-server php8.5-* redis-server supervisor influxdb2 fail2ban` |
| Удаление директорий | `rm -rf /var/www/novus /etc/novus /var/log/novus-installer` |
| Удаление репозиториев | `rm -f /etc/apt/sources.list.d/php*.list /etc/apt/sources.list.d/mariadb*.list /etc/apt/sources.list.d/influxdb.list` |
| Очистка APT | `apt-get autoremove -y && apt-get clean` |

> [!CAUTION]
> Hard rollback удаляет **все** следы установки, включая базы данных. Используйте только в тестовых окружениях.

---

## 3. Автоматический откат

При возникновении ошибки на шаге N:

1. Оркестратор останавливает выполнение
2. Вызывает `Rollback()` для каждого выполненного шага (в обратном порядке)
3. Логирует ошибку в audit-лог
4. Возвращает ошибку через WebSocket клиенту

Шаги, помеченные `Rollback: nil`, не имеют автоматического отката (например, установка системных пакетов).

---

## 4. Ручной откат

Если автоматический откат не сработал или установка была прервана (SIGINT, потеря соединения):

```bash
# Soft rollback вручную
rm -f /etc/nginx/sites-enabled/novus-installer.conf
rm -f /etc/nginx/sites-available/novus-installer.conf
rm -rf /tmp/novus-*
rm -f /etc/novus/manifest.json
systemctl reload nginx

# Hard rollback вручную (ТОЛЬКО тестовые окружения!)
systemctl stop nginx mariadb php8.5-fpm redis-server supervisor influxdb fail2ban novus-agent
apt-get purge -y nginx mariadb-server 'php8.5-*' redis-server supervisor influxdb2 fail2ban
rm -rf /var/www/novus /etc/novus /var/log/novus-installer
apt-get autoremove -y
```

---

## 5. Обработка частичных сбоев

### Сбой на шаге 1-3 (подготовка)
- Безопасный откат: удаление репозиториев
- Система не изменена

### Сбой на шаге 4 (установка пакетов)
- Пакеты установлены, но не настроены
- Soft rollback: сброс конфигурации
- Рекомендуется ручная проверка `dpkg -l`

### Сбой на шаге 5 (MariaDB)
- Базы данных созданы частично
- Soft rollback: `DROP DATABASE IF EXISTS` для `novus_os`, `novus_id`, `novus_sd`

### Сбой на шаге 7 (агент)
- Бинарник агента установлен, но не сопряжён
- Безопасно для повторной установки

### Сбой на шаге 9 (панель)
- Файлы панели распакованы частично
- Soft rollback: удаление `/var/www/novus`

### Сбой на шаге 10+ (конфигурация)
- Система в mixed state
- Рекомендуется Hard Rollback или ручная очистка
