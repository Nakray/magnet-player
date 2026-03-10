# Magnet Player

Стриминг музыки и видео с торрент-трекеров через Jackett с кэшированием на диске.

## Возможности

- 🔍 **Поиск торрентов** через Jackett API (поддержка множества трекеров)
- 🎵 **Стриминг** аудио/видео без полной загрузки
- 💾 **Кэш на диске** с LRU-эвикцией и лимитом размера
- 🌐 **Веб-интерфейс** — поиск, плеер, управление кэшем
- 🖥️ **CLI утилита** с интерактивным режимом
- 📡 **Range-запросы** для seek в плеере

## Архитектура

```
┌─────────────┐      HTTP      ┌─────────────┐      API     ┌───────────┐
│   CLI       │ ◄───────────► │   Server    │ ◄──────────► │  Jackett  │
│ (клиент)    │   :8080        │ (плеер)     │   :9117      │ (поиск)   │
└─────────────┘                └─────────────┘              └───────────┘
                                        │
                                        ▼
                                 ┌─────────────┐
                                 │   Кэш/БД    │
                                 │  (файлы)    │
                                 └─────────────┘
```

## Установка

### 1. Скомпилировать из исходников

```bash
go build -o magnet-player ./cmd/server
go build -o magnet-player-cli ./cmd/cli
```

### 2. Настроить Jackett

1. Установите и запустите [Jackett](https://github.com/Jackett/Jackett)
2. Откройте `http://localhost:9117`
3. Добавьте индексеры (трекеры) через веб-интерфейс
4. Скопируйте API ключ (кнопка "API Key" вверху)

### 3. Настроить magnet-player

Создайте `config.json`:

```json
{
  "server": {
    "host": "0.0.0.0",
    "port": 8080
  },
  "storage": {
    "base_dir": "./data",
    "db_path": "./data/meta.db",
    "max_size_gb": 10
  },
  "jackett": {
    "enabled": true,
    "base_url": "http://localhost:9117",
    "api_key": "ВАШ_API_KEY_ИЗ_JACKETT",
    "timeout_sec": 30,
    "audio_category_id": "3000"
  },
  "torrent": {
    "max_active_torrents": 5,
    "max_connections_per_torrent": 40
  }
}
```

## Запуск

### Сервер

```bash
./magnet-player
# или с путём к конфигу
MP_CONFIG=/path/to/config.json ./magnet-player
```

Откройте **http://localhost:8080** в браузере для доступа к веб-интерфейсу.

### CLI

```bash
# Интерактивный режим
./magnet-player-cli -i

# Разовые команды
./magnet-player-cli search "pink floyd"
./magnet-player-cli list
./magnet-player-cli status

# С указанием сервера
./magnet-player-cli -server http://192.168.1.100:8080 search "album"
```

## API

### Поиск торрентов
```bash
GET /api/search?q=<запрос>

# Пример
curl "http://localhost:8080/api/search?q=pink%20floyd"
```

**Ответ:**
```json
{
  "results": [
    {
      "title": "Pink Floyd - The Wall",
      "size": 123456789,
      "seeders": 42,
      "peers": 5,
      "magnet_link": "magnet:?xt=urn:btih:..."
    }
  ],
  "count": 1
}
```

### Добавить magnet-ссылку
```bash
POST /api/add-magnet
Content-Type: application/json

{"magnet": "magnet:?xt=urn:btih:..."}
```

### Стриминг файла
```bash
GET /api/stream?hash=<hash>

# С поддержкой Range (для seek)
curl -H "Range: bytes=0-1024" "http://localhost:8080/api/stream?hash=abc123"
```

### Список файлов в кеше
```bash
GET /api/files
```

### Удалить файл из кеша
```bash
DELETE /api/files/<hash>
```

### Проверка здоровья
```bash
GET /health
```

## CLI команды
| Команда | Описание |
|---------|----------|
| `search <query>` | Поиск торрентов через Jackett |
| `add <magnet>` | Добавить magnet-ссылку |
| `list` | Показать файлы в кеше |
| `remove <hash>` | Удалить файл из кеша |
| `status` | Проверить статус сервера |
| `help` | Показать справку |
| `quit` | Выход из интерактивного режима |

## Веб-интерфейс

Откройте **http://localhost:8080** в браузере.

### Возможности:
- **Поиск** — ввод запроса и поиск через Jackett
- **Результаты** — список с размером, сидами, индексером
- **Play** — добавление торрента и загрузка в кэш
- **Плеер** — встроенный аудио-плеер для файлов из кэша
- **Кэш** — управление загруженными файлами (прослушивание, удаление)

### Скриншот функционала:
1. Введите исполнителя или название в поле поиска
2. Нажмите Enter или кнопку "Поиск"
3. Нажмите "▶ Play" для добавления и воспроизведения
4. Файлы сохраняются в кеше — можно слушать повторно

## Конфигурация

### Server
| Параметр | По умолчанию | Описание |
|----------|--------------|----------|
| `host` | `0.0.0.0` | Адрес слушателя |
| `port` | `8080` | Порт HTTP-сервера |

### Storage
| Параметр | По умолчанию | Описание |
|----------|--------------|----------|
| `base_dir` | `./data` | Директория для кэша |
| `db_path` | `./data/meta.db` | Путь к BoltDB |
| `max_size_gb` | `10` | Максимальный размер кэша |

### Jackett
| Параметр | По умолчанию | Описание |
|----------|--------------|----------|
| `enabled` | `false` | Включить интеграцию |
| `base_url` | - | URL Jackett |
| `api_key` | - | API ключ Jackett |
| `timeout_sec` | `30` | Таймаут запросов |
| `audio_category_id` | `3000` | Категория аудио (Jackett) |

### Torrent
| Параметр | По умолчанию | Описание |
|----------|--------------|----------|
| `max_active_torrents` | `5` | Макс. активных торрентов |
| `max_connections_per_torrent` | `40` | Макс. соединений на торрент |

## Docker

```bash
docker build -t magnet-player .
docker run -d \
  -p 8080:8080 \
  -v ./config.json:/app/config.json \
  -v ./data:/app/data \
  magnet-player
```

## Лицензия

MIT
