# NOVUS Installer — Встроенный фронтенд

**Версия:** 0.1.0

---

## 1. Обзор

Инсталлятор включает встроенный одностраничный интерфейс (SPA) на Vue 3, который предоставляет пользователю пошаговый визард установки с live-стримингом вывода терминала.

---

## 2. Технологический стек

- **Vue 3** — реактивный UI
- **Vite** — сборка
- **Tailwind CSS 4.3** — стилизация
- **xterm.js** — эмуляция терминала (live-вывод)
- **vue-i18n** — локализация (русский, английский)
- **WebSocket** — live-stream установки

---

## 3. Структура

```
web/
├── embed.go                  # go:embed директивы
├── tailwind.config.cjs       # Tailwind конфигурация
├── tailwind.input.css        # Входной CSS
└── dist/                     # Скомпилированный SPA (встраивается в бинарник)
    ├── index.html
    ├── assets/
    │   ├── app.js
    │   └── installer.css
    └── ...
```

---

## 4. Сборка

```bash
cd web
npm ci
npm run build
# Результат → dist/
```

Файлы в `dist/` автоматически встраиваются в Go-бинарник через `//go:embed dist/*` в `embed.go`.

---

## 5. Шаги визарда

1. **Вход по токену** — валидация токена из URL
2. **Домен** — ввод домена панели
3. **Администратор** — email + пароль
4. **База данных** — root-пароль MariaDB + пароль пользователя панели
5. **Master Key** — автоматическая генерация или ввод своего ключа
6. **Безопасность** — Security Entrance (опционально)
7. **Интеграции** — Telegram, Discord (опционально)
8. **Cloudflare KMS** — (опционально)
9. **Восстановление** — импорт бекапа (опционально)
10. **Установка** — live-stream выполнения всех шагов
11. **Финал** — health check, ссылка на панель

---

## 6. WebSocket-интеграция

Фронтенд подключается к WebSocket серверу инсталлятора для получения live-вывода:

```javascript
const protocol = location.protocol === 'https:' ? 'wss:' : 'ws:';
const ws = new WebSocket(`${protocol}//${location.host}/ws/stream`);

ws.onmessage = (event) => {
  const msg = JSON.parse(event.data);
  switch (msg.type) {
    case 'step_status':
      updateStepStatus(msg.step, msg.status);
      break;
    case 'pty_output':
      terminal.write(msg.data);
      break;
    case 'progress':
      updateProgress(msg.percent, msg.message);
      break;
    case 'error':
      showError(msg.code, msg.message);
      break;
    case 'complete':
      showCompletion(msg.manifest);
      break;
  }
};
```

---

## 7. i18n

Поддерживаемые языки:
- Русский (основной)
- Английский

Файлы локализации: `web/src/locales/{ru,en}.json`

Ключи:
```json
{
  "wizard": {
    "step": "Шаг {current} из {total}",
    "domain": "Домен панели",
    "admin": "Администратор",
    "database": "База данных",
    "install": "Установка",
    "complete": "Установка завершена"
  }
}
```

---

## 8. Терминал (xterm.js)

Встроенный терминал отображает вывод всех shell-команд в реальном времени:

```javascript
import { Terminal } from 'xterm';
import { FitAddon } from '@xterm/addon-fit';

const term = new Terminal({
  theme: {
    background: '#1a1b26',
    foreground: '#a9b1d6',
  },
  fontSize: 13,
  fontFamily: 'JetBrains Mono, monospace',
});

const fitAddon = new FitAddon();
term.loadAddon(fitAddon);
term.open(document.getElementById('terminal'));
fitAddon.fit();
```

---

## 9. Темы

Фронтенд поддерживает светлую и тёмную темы через Tailwind CSS:

- `dark:` префиксы для тёмной темы
- Авто-определение через `prefers-color-scheme`
- Ручное переключение через кнопку в header
