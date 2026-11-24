<div align="center">

# 🗄️ Distributed In-Memory Key-Value Storage

**Распределённая Key-Value база данных для кэширования, работающая в оперативной памяти и написанная с нуля на Go**

![Go Version](https://img.shields.io/badge/Go-1.25+-00ADD8?logo=go&logoColor=white)
![Status](https://img.shields.io/badge/status-work%20in%20progress-orange)
![License](https://img.shields.io/badge/license-MIT-blue)

</div>

## 📖 О проекте

Key-Value база данных, вдохновлённая Redis и созданная для обучения: хранение данных в памяти, сетевое взаимодействие по UDP, конкурентность, шардирование, репликация и обнаружение отказов.

## ✨ Возможности

- `SET`, `GET`, `SET NX`, `DEL`
- TTL и автоматическое удаление просроченных ключей
- Получение списка ключей с фильтрацией по регулярным выражениям
- Транзакции
- Lua-скрипты
- Шардирование, репликация, failover и ребалансировка

## 🚀 Требования

Для локальной сборки и запуска проекта необходимо установить:

- **Go** (версия `1.25` или выше)
- **Task** (альтернатива `make` для автоматизации команд) - [Инструкция по установке](https://taskfile.dev/installation/)
- **golangci-lint** - [Инструкция по установке](https://golangci-lint.run/welcome/install/)
- **Buf** (проверка и генерация Protobuf-кода) - [Инструкция по установке](https://buf.build/docs/cli/installation/)

## 🛠 Команды разработки (Taskfile)

Для автоматизации сборки, тестирования, проверки качества и форматирования кода используется `Taskfile.yml`.

### Доступные команды

| Команда | Описание |
| :--- | :--- |
| `task` | Показать справку и список доступных задач |
| `task test` | Запустить все модульные тесты с проверкой гонок (`-race`) |
| `task test:short` | Запустить быстрые тесты без проверки гонок |
| `task test:cover` | Запустить тесты и создать HTML-отчёт о покрытии |
| `task bench` | Запустить Go-бенчмарки |
| `task lint` | Проверить качество кода с помощью `golangci-lint` |
| `task fmt` | Отформатировать код с помощью `gofmt` / `golangci-lint fmt` |
| `task deps` | Загрузить и привести в порядок зависимости Go-модуля |
| `task clean` | Удалить временные файлы сборки и отчёты о покрытии |
| `task proto:lint` | Проверить Protobuf-схемы с помощью Buf |
| `task proto:format` | Отформатировать Protobuf-схемы |
| `task proto:build` | Проверить сборку Protobuf-схем |
| `task proto:generate` | Сгенерировать Go-код из Protobuf-схем |
| `task proto:push` | Опубликовать Protobuf-схемы в Buf Schema Registry |

## 📁 Структура проекта

```text
cmd/        # Точки входа сервера и CLI
internal/   # Хранилище, протокол, TTL, транзакции, скрипты, сервер
docs/       # Документация
```

Подробнее: [`adept/wiki/VISION.md`](adept/wiki/VISION.md)

## 📜 Лицензия

MIT
