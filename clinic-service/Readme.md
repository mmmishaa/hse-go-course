# 1. Запустить всё
docker compose up -d

# 2. Подождать 10 секунд и проверить
docker compose ps

# 3. Открыть в браузере:
#    - API: http://localhost:8080
#    - Grafana: http://localhost:3000 (admin/admin)
#    - Prometheus: http://localhost:9090
#    - RabbitMQ: http://localhost:15672 (guest/guest)

# 4. Запустить нагрузочный тест
docker compose --profile load run --rm loadgen

# 5. Смотреть метрики в Grafana
