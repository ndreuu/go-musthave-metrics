# go-musthave-metrics-tpl

Шаблон репозитория для трека «Сервер сбора метрик и алертинга».

## Начало работы

1. Склонируйте репозиторий в любую подходящую директорию на вашем компьютере.
2. В корне репозитория выполните команду `go mod init <name>` (где `<name>` — адрес вашего репозитория на GitHub без префикса `https://`) для создания модуля.

## Обновление шаблона

Чтобы иметь возможность получать обновления автотестов и других частей шаблона, выполните команду:

```
git remote add -m v2 template https://github.com/Yandex-Practicum/go-musthave-metrics-tpl.git
```

Для обновления кода автотестов выполните команду:

```
git fetch template && git checkout template/v2 .github
```

Затем добавьте полученные изменения в свой репозиторий.

## Запуск автотестов

Для успешного запуска автотестов называйте ветки `iter<number>`, где `<number>` — порядковый номер инкремента. Например, в ветке с названием `iter4` запустятся автотесты для инкрементов с первого по четвёртый.

При мёрже ветки с инкрементом в основную ветку `main` будут запускаться все автотесты.

Подробнее про локальный и автоматический запуск читайте в [README автотестов](https://github.com/Yandex-Practicum/go-autotests).

## Структура проекта

Приведённая в этом репозитории структура проекта является рекомендуемой, но не обязательной.

Это лишь пример организации кода, который поможет вам в реализации сервиса.

При необходимости можно вносить изменения в структуру проекта, использовать любые библиотеки и предпочитаемые структурные паттерны организации кода приложения, например:
- **DDD** (Domain-Driven Design)
- **Clean Architecture**
- **Hexagonal Architecture**
- **Layered Architecture**

## Профилирование и оптимизация производительности

### Бенчмарки

**Результаты бенчмарков (после оптимизации):**

```
BenchmarkMemStorage_SetGauge-8              4925610    294.9 ns/op    114 B/op    2 allocs/op
BenchmarkMemStorage_AddCounter-8           45194760     27.55 ns/op     0 B/op    0 allocs/op
BenchmarkMemStorage_GetGauge-8             86643900     15.02 ns/op     0 B/op    0 allocs/op
BenchmarkMemStorage_GetCounter-8           83813514     14.20 ns/op     0 B/op    0 allocs/op
BenchmarkMemStorage_GetAll-8                 327638     3890 ns/op  15168 B/op  201 allocs/op
BenchmarkMemStorage_UpdateMetricsBatch-8    9588729    115.1 ns/op      0 B/op    0 allocs/op
BenchmarkMemStorage_Concurrent-8            1959252    631.8 ns/op     93 B/op    6 allocs/op
BenchmarkMemStorage_SaveToFile-8               2494  544540 ns/op  44618 B/op  212 allocs/op

BenchmarkMetricsService_UpdateMetric-8      3174408    383.1 ns/op    125 B/op    4 allocs/op
BenchmarkMetricsService_GetMetricValue-8   31857736     37.46 ns/op     8 B/op    1 allocs/op
BenchmarkMetricsService_GetAllMetrics-8      322357     3767 ns/op  15168 B/op  201 allocs/op
BenchmarkMetricsService_GetMetricFromJSON/Gauge-8    52172874    23.06 ns/op    8 B/op    1 allocs/op
```

**Улучшение производительности после оптимизации:**

| Бенчмарк | До оптимизации | После оптимизации | Улучшение |
|----------|----------------|-------------------|-----------|
| `GetAll` | 41088 B/op, 209 allocs/op | 15168 B/op, 201 allocs/op | **-63% памяти** |
| `GetMetricFromJSON` | 72 B/op, 2 allocs/op | 8 B/op, 1 allocs/op | **-89% памяти** |
| `GetAll` (время) | 6566 ns/op | 3890 ns/op | **-40% быстрее** |

### Профилирование памяти

**Процесс профилирования:**

1. Создание базового профиля (до оптимизации):
```bash
go run cmd/profiler/main.go -out profiles/base.pprof
```

2. Оптимизация кода:
   - **Предварительное выделение слайса в `GetAll()`** — использование `make([]models.Metrics, 0, totalCount)` вместо `var metrics []models.Metrics`
   - **Переиспользование входной структуры в `GetMetricFromJSON()`** — `result := m` вместо создания новой структуры `&models.Metrics{ID: m.ID, MType: m.MType}`

3. Создание профиля после оптимизации:
```bash
go run cmd/profiler/main.go -out profiles/result.pprof
```

4. Анализ профиля с помощью команд pprof:
```bash
go tool pprof -alloc_space profiles/base.pprof

(pprof) top
(pprof) list GetAll
(pprof) peek GetAll
(pprof) web
```

5. Сравнение профилей:
```bash
go tool pprof -top \
  -diff_base=profiles/base.pprof \
  profiles/result.pprof

go tool pprof -alloc_space -top \
  -diff_base=profiles/base.pprof \
  profiles/result.pprof
```

**Результаты оптимизации:**

```
File: main
Type: alloc_space
Time: 2026-08-13 17:13:41 MSK
Showing nodes accounting for -51.20MB, 62.15% of 82.37MB total
      flat  flat%   sum%        cum   cum%
  -48.21MB 58.53% 58.53%   -48.21MB 58.53%  go-musthave-metrics/internal/repository.(*MemStorage).GetAll
      -4MB  4.86% 63.38%       -4MB  4.86%  go-musthave-metrics/internal/service.(*MetricsService).GetMetricFromJSON
    1.01MB  1.23% 62.15%     1.01MB  1.23%  runtime.mallocgc
         0     0% 62.15%   -48.21MB 58.53%  go-musthave-metrics/internal/service.(*MetricsService).GetAllMetrics (inline)
         0     0% 62.15%   -52.21MB 63.38%  main.main
```

**Итог:**
- `GetAll`: **-48.21MB** (-58.53%)
- `GetMetricFromJSON`: **-4MB** (-4.86%)
- **Суммарное снижение объёма аллокаций: -51.20MB** (-62.15%)


**Инфо:**

```bash
go test -bench=. -benchmem ./internal/repository/... ./internal/service/... ./internal/middleware/...

go test -bench=. -cpuprofile=cpu.out ./internal/repository/...

go test -bench=. -memprofile=mem.out ./internal/repository/...
```
