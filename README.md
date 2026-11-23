# Разработка интеграционных систем

## Практическая работа №1 — импорт JSON

Go-утилита скачивает JSON из Nobel Prize API v1, сохраняет лауреатов, премии и организации в PostgreSQL и пишет ход импорта в `import.log`.

### Запуск

Требуются Go 1.26.7 и Docker Compose. Команды выполняются из корня проекта.

```powershell
docker compose up -d --wait postgres
go run ./cmd/import
```

Первый запуск применяет миграции Goose автоматически. По умолчанию используется `https://api.nobelprize.org/v1/laureate.json`. Доступные параметры:

```powershell
go run ./cmd/import -help
go run ./cmd/import -url http://127.0.0.1:8000/laureates.json -log import.log
```

Вторая команда предназначена для локального примера. В отдельном терминале его можно раздать командой `python -m http.server 8000 --bind 127.0.0.1 --directory examples`. В примере три лауреата. Настройки PostgreSQL можно переопределить переменной `RIS_DATABASE_URL` или флагом `-dsn`.

### Проверка

```powershell
docker compose exec postgres psql -U ris -d ris -c "SELECT id, first_name, last_name, country_code FROM laureates ORDER BY id LIMIT 10"
docker compose exec postgres psql -U ris -d ris -c "SELECT (SELECT count(*) FROM laureates) AS laureates, (SELECT count(*) FROM prizes) AS prizes, (SELECT count(*) FROM affiliations) AS affiliations, last_updated FROM dataset_metadata"
Get-Content import.log -Tail 10
```

При повторном импорте записи сопоставляются по идентификатору источника, премии и организации обновляются в той же транзакции. Ошибка загрузки, разбора или сохранения приводит к ненулевому коду выхода.

Источник данных — [Nobel Prize API v1](https://nobelprize.readme.io/reference/laureate).
