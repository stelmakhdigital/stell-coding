# github.com/stelmakhdigital/stell-coding

Продуктовый CLI (`stell`) и встраиваемый SDK поверх `github.com/stelmakhdigital/stell-agent` и `github.com/stelmakhdigital/stell-tui`.

Зависит от `github.com/stelmakhdigital/stell-ai`, `github.com/stelmakhdigital/stell-agent`, `github.com/stelmakhdigital/stell-tui`.

## Возможности

- Интерактивный TUI: чат, оверлеи, tool-карточки, темы
- Print-режим (`-p`) и JSONL RPC (`--mode rpc`)
- Embeddable SDK: `CreateSession` / `Prompt` / steer / follow-up
- Конфиг `~/.stell/agent`, project `.stell/`, модели и auth
- Skills, prompts, packages, subprocess-расширения
- Компактирование контекста, retry, proxy (`STELL_PROXY_*`)

## Карта директорий

| Путь | Назначение |
|------|------------|
| `cmd/stell/` | бинарник CLI |
| `sdk/` | публичный Go API для встраивания |
| `internal/agent/` | продуктовый Agent и Service |
| `internal/tui/` | интерактивный терминальный UI |
| `internal/rpc/` | JSONL RPC-сервер |
| `internal/config/` | settings, models, auth |
| `internal/extensions/` | subprocess-расширения и grants |
| `internal/discovery/`, `skills/`, `prompts/`, `catalog/` | ресурсы агента |
| `internal/packages/` | установка/обновление пакетов |
| `internal/themes/` | темы UI |
| `internal/update/` | проверка версий и self-update |
| `internal/contextbuild/`, `workspace/`, `trust/`, `telemetry/`, `version/` | вспомогательные слои |

## SDK

```go
import "github.com/stelmakhdigital/stell-coding/sdk"

sess, err := sdk.CreateSession("/path/to/workspace")
if err != nil {
	panic(err)
}
events, err := sess.Prompt(ctx, "hello")
_ = events
```

## CLI

```bash
go run ./cmd/stell
go run ./cmd/stell -p "summarize this repo"
go run ./cmd/stell --mode rpc
```
