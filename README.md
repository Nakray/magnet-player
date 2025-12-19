# Magnet Player (Skeleton)

## Локальный запуск

```bash
go run ./cmd/server
```

## Docker

```bash
docker build -t magnet-player .
docker run --rm -p 8080:8080 magnet-player
```

## API Endpoints

- `GET /health` - health check
- `POST /api/add-magnet` - добавить magnet ссылку
- `GET /api/stream` - стримить файл

## Environment Variables

- `MP_BASE_DIR` - директория для скачивания (default: ./data)
- `MP_DB_PATH` - путь к BoltDB файлу (default: ./data/meta.db)

## TODO

- [ ] Реализовать полную логику стриминга
- [ ] Добавить транскодинг через FFmpeg
- [ ] Расширить API документацию
- [ ] Добавить unit тесты
