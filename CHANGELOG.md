# Changelog

Все значимые изменения инсталлятора `novus-installer` документируются в этом файле.
Формат основан на [Keep a Changelog](https://keepachangelog.com/ru/1.1.0/);
проект строго следует [SemVer 2.0.0](https://semver.org/lang/ru/) в линейке `0.x.y` (Раздел 8 AGENTS.md).

## [Unreleased]

## [0.1.0] — 2026-07-18

### Added
- Интерактивный Web-Wizard первичной настройки на Vue 3 и Tailwind CSS.
- Пакет `internal/version` со стандартизированным SemVer 2.0.0 версионированием и `-ldflags`.
- Preflight валидация окружения для Ubuntu 22.04/24.04 и Debian 12.
- Автоматический pairing локального агента `novus-agent`.
- Генерация криптографических конвертов секретов (master envelope, canary).

### Changed
- Устранен произвольный хардкод версии 1.0.0; проект переведен на честный pre-release `0.1.0-dev`.
- Улучшена обработка ошибок PTY выполнения команд при установке пакетов.

### Security
- Изоляция секретов в `/etc/novus/secrets` с правами доступа `0750` / `0640`.
- Защита канарейки от подделки через сверку с PHP-рантаймом панели.

[0.1.0]: https://github.com/SGC-NOVUS/installer/releases/tag/v0.1.0
