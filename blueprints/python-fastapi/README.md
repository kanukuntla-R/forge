# python-fastapi

FastAPI starter with async support, uv dependency management, Ruff linting, and feature toggles.

## Usage

```bash
forge new python-fastapi my-api
```

Feature flags (all default false):

| Flag | What it adds |
|---|---|
| `with_database` | SQLAlchemy async + PostgreSQL + Alembic migrations |
| `with_auth` | JWT auth + bcrypt (implicitly enables database infrastructure) |
| `with_docker` | Dockerfile + docker-compose.yml + .dockerignore |
| `with_openai` | openai + anthropic SDKs + /chat endpoint |
| `with_type_check` | mypy configuration |

> **Auth note**: Enabling `with_auth` automatically creates the database connection layer
> (`database.py`, `models/user.py`, `schemas/user.py`, Alembic config) because auth requires
> user records. The explicit `/users` CRUD endpoint is only added with `with_database=true`.

## Examples

Minimal API:
```bash
forge new python-fastapi my-api --yes --no-hooks
```

With database and auth:
```bash
forge new python-fastapi my-api \
  --var with_database=true \
  --var with_auth=true \
  --yes --no-hooks
```

Full stack:
```bash
forge new python-fastapi my-api \
  --var with_database=true \
  --var with_auth=true \
  --var with_docker=true \
  --var with_openai=true \
  --var with_type_check=true \
  --yes --no-hooks
```

## Stack

- Python 3.12
- FastAPI + Uvicorn (ASGI)
- uv (dependency and virtual-environment management)
- pytest + pytest-asyncio + httpx (async testing)
- Ruff (linting and import sorting)
