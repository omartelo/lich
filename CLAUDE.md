# skipo

## O que é

`skipo` é um **harness pessoal** — desktop app em [Wails v3](https://v3alpha.wails.io/) (Go backend + React/TypeScript frontend). Construído com base na experiência e no gosto do autor por outros harnesses do mercado; não é um produto genérico, é uma ferramenta feita sob medida.

## Stack

- **Backend**: Go 1.25, Wails v3 (`v3.0.0-alpha2.116`). Services expostos ao frontend via `application.NewService` (ver `main.go`).
- **Frontend**: React 18 + TypeScript + Vite. Bindings Go→TS gerados pelo Wails.
- **Build/tasks**: [Task](https://taskfile.dev) — `task dev`, `task build`, `task run`.

## Comandos

```bash
task dev      # modo dev (hot reload, Vite na porta 9245)
task build    # build de produção
task run      # roda o binário
```

Frontend isolado: `cd frontend && npm run build` (roda `tsc` + `vite build`).

---

## Hard Invariants

Regras inegociáveis. Violação = trabalho não está concluído.

1. **Idioma do código em inglês.** Todo código, identificadores, comentários, mensagens de commit e nomes de arquivo em inglês. Comunicação com o autor pode ser em pt-BR; o código, não.

2. **Cobertura de testes ≥ 80%.** Backend (Go) e frontend (React/TS). Sem teste, a feature não está pronta. Rode a suíte antes de marcar qualquer task concluída; falhou, corrige antes.

3. **Clean code.**
   - Funções pequenas e focadas (< 50 linhas), uma responsabilidade.
   - Arquivos coesos (200–400 linhas típico, 800 máximo). Muitos arquivos pequenos > poucos grandes.
   - Sem aninhamento profundo (> 4 níveis) — use early returns.
   - Nomes descritivos; código se explica sozinho, comentário só para o *porquê*.
   - Erros tratados explicitamente, nunca engolidos em silêncio.
   - Sem valores mágicos hardcoded — constantes ou config.
   - Sem segredos no código-fonte.
