# mygrep — распределённый grep с кворумом

<!-- ПОСЛЕ ПУША НА GITHUB: замени OWNER/REPO на свой GitHub-путь -->
![ci](https://github.com/OWNER/REPO/actions/workflows/ci.yml/badge.svg)
![go](https://img.shields.io/badge/go-1.26-00ADD8?logo=go)
![license](https://img.shields.io/badge/license-MIT-green)

`mygrep` — учебная утилита, повторяющая поведение `grep -nE`, но умеющая
работать как кластер из N воркеров. Каждый чанк входных данных отдаётся
сразу R воркерам (replication factor). Совпадение попадает в финальный
вывод тогда и только тогда, когда о нём сообщили `quorum` (по умолчанию
`R/2+1`) воркеров.

## Структура

```
mygrep/
├── cmd/mygrep/main.go              
├── internal/
│   ├── domain/                     
│   │   ├── match.go                
│   │   ├── chunk.go                
│   │   └── ports.go                
│   ├── usecase/                    
│   │   ├── worker.go               
│   │   ├── quorum.go               
│   │   └── coordinator.go          
│   ├── adapter/                    
│   │   ├── matcher/regex.go       
│   │   ├── splitter/line.go       
│   │   ├── sink/stdout.go         
│   │   └── transport/http/        
│   └── config/config.go           
├── examples/                       
├── tests/                          
├── Dockerfile                      
├── docker-compose.yml              
├── .github/workflows/ci.yml        
├── Makefile                        
└── .golangci.yml                   
```

## Режимы

Один бинарь, три режима — выбирается флагом `-mode`:

| режим | назначение |
|-------|------------|
| `local` | как обычный grep, без сети (один in-process воркер) |
| `server` | поднимает HTTP-воркер; принимает чанки от координатора |
| `coordinator` | главный: режет вход, рассылает R репликам, собирает кворум |

## Quick start

```bash
make build           
make test test-race  
make lint            
```

### Локально (как `grep`)

```bash
./bin/mygrep -mode=local -e='^abc' /usr/share/dict/words
# либо:
make run PATTERN='^abc' FILE=/usr/share/dict/words
```

### Распределённо (bash-скрипты)

```bash
make cluster-up                                     # 3 воркера на :8081..:8083
./bin/mygrep \
  -mode=coordinator \
  -workers='http://localhost:8081,http://localhost:8082,http://localhost:8083' \
  -replication=3 -quorum=2 \
  -e='^abra' /usr/share/dict/words
make compare FILE=/usr/share/dict/words PATTERN='^abra'   # сверить с системным grep
make cluster-down
```

## Docker

Образ собирается в две стадии: на `golang:1.26.3-alpine` компилируем
статический бинарь, в финальном `alpine:3.20` остаётся бинарь, `wget`
(нужен только для healthcheck) и непривилегированный юзер.

```bash
make docker-build                      
make docker-up                         
docker compose ps                      
make docker-run PATTERN='^abra' FILE=examples/start-cluster.sh
make docker-down
```

`docker compose run --rm coordinator ...` поднимает координатор внутри той
же compose-сети с DNS-именами `worker1/worker2/worker3`, текущий каталог
монтируется в `/data:ro`. То есть `FILE=examples/start-cluster.sh`
читается внутри контейнера как `/data/examples/start-cluster.sh`.

## CI

`.github/workflows/ci.yml` запускает на каждый PR/push:
- `make lint` — golangci-lint
- `make test` — юнит + интеграционные
- `make test-race` — детектор гонок
- `docker build` — smoke-проверка `Dockerfile`

## Кворум: что именно проверяется

1. Координатор режет вход на чанки по `-chunk` строк.
2. Каждый чанк отправляется `-replication` воркерам (round-robin по списку).
3. От каждого воркера приходит список совпадений с **абсолютными** номерами строк.
4. `Match.Key() = source + lineno + line` — стабильный отпечаток.
   `Quorum.Reconcile` принимает совпадение, если про него сообщили ≥ `-quorum`
   разных воркеров. «Мнение одиночки» отсеивается.
5. Если успешных ответов на чанк меньше `quorum` — обработка падает с
   понятной ошибкой `кворум не достигнут...`.

## Tests

```bash
make test           
make test-race      
make cover          
```

- `TestCompareWithSystemGrep` гоняет наш пайплайн через 3 локальных воркера
  и сверяет результат с `grep -nE`.
- `TestQuorumDropsMinorityOpinion` — модульный тест на отсев «одиночного
  мнения» в `Quorum.Reconcile`.

## License

MIT. См. [LICENSE](./LICENSE).
