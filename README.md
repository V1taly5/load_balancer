# HTTP load balancer

## Конфигурация

Конфигурационный файл `config.yaml` определяет основные параметры работы балансировщика и лимитера запросов. Пример конфигурации:

```yaml
env: "local"
httpserver:
  port: 8080
  timeout: 5s
  idle_timeout: 120s

backends:
  - url: "http://localhost:7071"
  - url: "http://localhost:7072"
  - url: "http://localhost:7073"
  - url: "http://localhost:7074"
  - url: "http://localhost:7075"
  - url: "http://localhost:7076"
  - url: "http://localhost:7077"
  - url: "http://localhost:7078"
  - url: "http://localhost:7079"

health_checker:
  interval: 10s
  health_path: "/health"
  timeout: 5s

rate_limiter:
  enabled: true
  default_capacity: 10
  default_rate: 1
  cleanup_interval: 10m
  bucket_ttl: 60m
  header_ip: "X-Forwarded-For"

storage:
  file_path: "storage/store.json"
```
Параметры конфигурации:

    env: определяет среду, может быть local или prod;
    httpserver: настройки для HTTP-сервера, включет порт, таймауты (ReadTimeout и WriteTimeout задаются одним) и idle таймаут;
    backends: cписок бэкэнд-серверов, среди которых балансировщик распределяет трафик;
    health_checker: параметры для проверки состояния бэкэндов (timeout для таймаута исходящего запроса к бэкендам, health_path - путь для проверки здоровья бэкенда);
    rate_limiter: параметры ограничения частоты запросов (header_ip - заголовок из которого балансировщик может брать ip адресс клиента);
    storage: путь к файлу для хранения состояния лимитеров запросов.

**Замечу, что health_checker работает, только если у бэкендов есть endpoint для проверки**

## Сборка Docker-образа
    Предусловие: находимся в корне проекта.
Для сборки Docker-образа (находится в корне репозитория) используйте следующую команду:
```sh
docker build -t loadbalancer .
```
## Запуск Docker-контейнера
Для запуска контейнера используйте следующую команду:
```sh
docker run -d \
  --name balancer \
  --network сеть \
  -v путь/к/config.yaml:/app/conf/config.yaml \
  -v путь/к/store.json:/app/data/store.json \
  -p 8080:8080 \
  loadbalancer
```
  - `-v путь/к/config.yaml:/app/conf/config.yaml` монтируем локальный файл конфигурации в контейнер;
  - `-v путь/к/store.json:/app/data/store.json` монтируем локальный файл для хранилища в контейнере.

Балансировщик тестировался с тестовыми бэкендами (https://github.com/V1taly5/test_backends) запущеными через docker compose для этого была создана общая сеть 
```sh
docker network create shared-net
```
## Сборка и запуск без Docker
    Предусловие: находимся в корне проекта.
Устанавливаем зависимости (по необходимости)
```sh
go mod tidy
```
Сборка приложения
```sh
go build -o balancer ./cmd/balancer
```
Подготовка конфигурации и файла хранилища

    Путь к конфигу передается через флаг `config`, без установления этого флага, по умолчанию, путь к конфигу устанавливается как `./config/config.yaml`

    Путь к файлу хранилища задается внутри конфигурационного файла, если файл не будет найден, то он будет создан(создан будет только если не найден сам файл, если не найдена директория то будет ошибка)

Запуск приложения
```sh
./balancer -config ./conf/config.yaml
```

## Список конечных точек

| Метод  | Путь                        | Описание                                         | Тело запроса (JSON) / Параметры               | Ответы (коды и описание)                  |
|--------|-----------------------------|-------------------------------------------------|-----------------------------------------------|-------------------------------------------|
| **POST** | `/api/clients`               | Создаёт нового клиента с ограничениями по частоте запросов | ```json { "client_id": "string", "capacity": 1000, "rate_per_sec": 10 }``` | `201 Created` клиент успешно создан.<br>`400 Bad Request` неверный запрос.<br>`409 Conflict` клиент уже существует. |
| **GET**  | `/api/clients/{client_id}`   | Получает информацию о клиенте.                   | Нет (ID клиента передаётся в URL).            | `200 OK` информация о клиенте в JSON.<br>`404 Not Found` клиент не найден. |
| **PUT**  | `/api/clients/{client_id}`   | Обновляет ограничения на частоту запросов для клиента | ```json { "capacity": 1000, "rate_per_sec": 10 }``` | `200 OK` ограничения обновлены.<br>`400 Bad Request` неверный запрос.<br>`404 Not Found` клиент не найден. |
| **DELETE**| `/api/clients/{client_id}`   | Удаляет клиента и его ограничения.              | Нет (ID клиента передаётся в URL).            | `204 No Content` клиент успешно удалён.<br>`404 Not Found` клиент не найден. |
 
## Proxy Handler  

Все запросы которые не соответствуют эндпоинтам выше, передаются на обработку в proxyHandler и будут направлены на один из бэкэнд-серверов, указанных в конфигурации.

### Нагрузочный тест через Apache Bench
```sh
ab -n 5000 -c 1000 http://localhost:8080/
```
### Результаты тестирования после добавления conneciton-pool
**Реализация с использованием conneciton-pool лежит на ветки [feature/connection-pool](https://github.com/V1taly5/load_balancer/tree/feature/connection-pool)**
при connection_pool_size = 10
```sh
Server Software:
Server Hostname:        localhost
Server Port:            8080

Document Path:          /
Document Length:        221 bytes

Concurrency Level:      1000
Time taken for tests:   1.702 seconds
Complete requests:      5000
Failed requests:        0
Total transferred:      1695000 bytes
HTML transferred:       1105000 bytes
Requests per second:    2937.03 [#/sec] (mean)
Time per request:       340.480 [ms] (mean)
Time per request:       0.340 [ms] (mean, across all concurrent requests)
Transfer rate:          972.32 [Kbytes/sec] received

Connection Times (ms)
              min  mean[+/-sd] median   max
Connect:        0    5  11.4      0      43
Processing:    13  308  56.5    309     486
Waiting:        2  308  56.5    309     486
Total:         45  313  51.9    315     495

Percentage of the requests served within a certain time (ms)
  50%    315
  66%    326
  75%    330
  80%    349
  90%    368
  95%    408
  98%    430
  99%    482
 100%    495 (longest request)
```


### Результат тестирования без conneciton-pool
```sh
Server Software:
Server Hostname:        localhost
Server Port:            8080

Document Path:          /
Document Length:        221 bytes

Concurrency Level:      1000
Time taken for tests:   1.619 seconds
Complete requests:      5000
Failed requests:        0
Total transferred:      1695000 bytes
HTML transferred:       1105000 bytes
Requests per second:    3088.06 [#/sec] (mean)
Time per request:       323.828 [ms] (mean)
Time per request:       0.324 [ms] (mean, across all concurrent requests)
Transfer rate:          1022.32 [Kbytes/sec] received

Connection Times (ms)
              min  mean[+/-sd] median   max
Connect:        0    6  11.3      1      42
Processing:     1  293 153.0    299     910
Waiting:        1  292 152.9    299     909
Total:          1  299 155.0    302     927

Percentage of the requests served within a certain time (ms)
  50%    302
  66%    354
  75%    409
  80%    431
  90%    497
  95%    569
  98%    624
  99%    650
 100%    927 (longest request)
```