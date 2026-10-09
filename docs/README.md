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
| [GROUND_TRUTH_IMPLEMENTATION_2026-09-28.md](GROUND_TRUTH_IMPLEMENTATION_2026-09-28.md) | Внутренний source-checked реестр конфигурации, preflight, pipeline и rollback |

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
| **ОС** | Strict preflight: Ubuntu 22.04/24.04/26.04/26.10 или Debian 12/13 |
| **RAM** | 2 GiB |
| **Disk** | 10 GiB свободного места на `/` |
| **CPU** | Не проверяется текущим preflight |
| **Доступ** | Effective UID 0; `sudo` должен запускать бинарник от root |
| **Порты** | Strict preflight проверяет только 80 и 443; 8080/9443 не проверяются до запуска |

## Лицензия

Проприетарная. Все права защищены © SGC-NOVUS.
