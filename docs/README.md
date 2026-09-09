# NOVUS Installer — Документация

**Версия:** 0.1.0  
**Язык:** Go 1.24  
**Назначение:** Автоматический установщик NOVUS-OS на чистый VPS с веб-интерфейсом и пошаговым визардом.

---

## Навигация

| Документ | Описание |
|----------|----------|
| [ARCHITECTURE.md](ARCHITECTURE.md) | Архитектура: preflight → web → orchestrator → rollback |
| [INSTALL_STEPS.md](INSTALL_STEPS.md) | Детальное описание всех 15+ шагов установки |
| [API.md](API.md) | Web API инсталлятора: `/api/setup`, WebSocket stream |
| [SECURITY.md](SECURITY.md) | Токен-аутентификация, HTTPS, самоуничтожение, audit trail |
| [ROLLBACK.md](ROLLBACK.md) | Стратегия отката: soft/hard уровни |
| [FRONTEND.md](FRONTEND.md) | Встроенный SPA: сборка, i18n, WebSocket-стриминг |
| [DEVELOPMENT.md](DEVELOPMENT.md) | Локальная разработка: dry-run, --dev флаг |

---

## Быстрый старт

```bash
# Сборка
cd novus-installer
go build -o novus-installer ./cmd/installer

# Dry-run (без реальных изменений)
sudo ./novus-installer --dev --dry-run

# Production-установка
sudo ./novus-installer
# → http://<vps-ip>:8080/?token=<32-символьный-токен>
```

## Системные требования

| Параметр | Минимум |
|----------|---------|
| **ОС** | Ubuntu 22.04/24.04, Debian 12 |
| **RAM** | 2 GiB |
| **Disk** | 20 GB SSD |
| **CPU** | 2 ядра |
| **Доступ** | root или sudo |
| **Порты** | 80, 443, 8080 (установщик), 9443 (агент) |

## Лицензия

Проприетарная. Все права защищены © SGC-NOVUS.
