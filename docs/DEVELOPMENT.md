# NOVUS Installer — Разработка

**Версия:** 1.0.0

---

## 1. Локальная разработка

### Структура проекта

```
novus-installer/
├── cmd/installer/main.go     # Точка входа
├── internal/
│   ├── orchestrator/         # Ядро установки
│   │   ├── runner.go         # Шаги и логика
│   │   └── audit.go          # Аудит-логирование
│   ├── preflight/            # Предварительные проверки
│   │   └── preflight.go
│   └── web/                  # HTTP + WebSocket
│       ├── server.go
│       ├── api.go
│       ├── stream.go
│       └── locales.go
├── web/                      # Встроенный фронтенд
│   ├── embed.go              # go:embed
│   └── dist/                 # Скомпилированный SPA
├── go.mod
└── docs/                     # Документация
```

### Зависимости

```
github.com/creack/pty v1.1.21
github.com/gorilla/websocket v1.5.3
```

Минимальный attack surface — только две внешние зависимости.

---

## 2. Режимы запуска

| Флаг | Описание |
|------|----------|
| `--dev` | Ослабленные проверки, отключено самоуничтожение |
| `--dry-run` | Симуляция без реальных изменений (команды выводятся, но не выполняются) |

### Dry-run
```bash
sudo ./novus-installer --dev --dry-run
```

В dry-run режиме:
- PTY-команды логируются, но не выполняются
- Preflight-проверки выполняются
- Файлы не создаются
- Аудит-лог пишется с `dry_run: true`

---

## 3. Сборка

### Бинарник инсталлятора
```bash
cd novus-installer
go build -o novus-installer ./cmd/installer
```

### Встроенный фронтенд
```bash
cd web
npm ci
npm run build
# Результат в web/dist/ — автоматически встраивается через go:embed
```

### Полная сборка
```bash
# Собрать фронтенд
cd web && npm run build && cd ..

# Собрать бинарник
go build -o novus-installer ./cmd/installer

# Проверить размер
ls -lh novus-installer
```

---

## 4. Тестирование

### Unit-тесты
```bash
go test ./... -count=1
```

### Тестирование preflight
```bash
go test ./internal/preflight/... -v
```

### Интеграционное тестирование (требуется VPS)
```bash
# 1. Собрать бинарник
go build -o novus-installer ./cmd/installer

# 2. Скопировать на тестовый VPS
scp novus-installer root@test-vps:/root/

# 3. Dry-run
ssh root@test-vps './novus-installer --dev --dry-run'

# 4. Полная установка
ssh root@test-vps './novus-installer --dev'
```

---

## 5. Отладка

### Логи

```bash
# Аудит-лог инсталлятора
cat /var/log/novus-installer/audit.log | jq .

# Лог systemd (если запущен как сервис)
journalctl -u novus-installer -f
```

### WebSocket-отладка

Подключиться к WebSocket вручную:
```javascript
const ws = new WebSocket('ws://<vps-ip>:8080/ws/stream');
ws.onmessage = (e) => console.log(JSON.parse(e.data));
```

---

## 6. CI/CD

Добавить GitHub Actions workflow (`.github/workflows/ci.yml`):

```yaml
name: CI
on: [push, pull_request]
jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with: { go-version: '1.24' }
      - run: gofmt -w . && git diff --exit-code
      - run: go vet ./...
      - run: go test ./... -count=1
      - run: go build -o novus-installer ./cmd/installer
```

---

## 7. Код-стайл

- `gofmt` для форматирования
- `go vet` для статического анализа
- Имена функций: camelCase
- Имена констант: PascalCase
- Обработка ошибок: явная, без `panic`
- Комментарии к экспортируемым функциям: `// FunctionName does X.`
