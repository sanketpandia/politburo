# Database migrations

Migrations are plain PostgreSQL files applied manually in filename order. The
application does not run migrations and no migration framework is installed.

`000_infinite_schema.sql` is the baseline schema. It contains no application
data and establishes the tables, enums, functions, constraints, indexes, and
triggers inherited from the existing Infinite Experiment database.

Apply the baseline to an empty local rewrite database from `labour-bureau/`:

```sh
docker compose -f docker-compose.dev.yml exec -T db \
  psql -v ON_ERROR_STOP=1 -1 -U ieuser -d politburo_next \
  < ../politburo/migrations/000_infinite_schema.sql
```

Validate that PostgreSQL can parse and apply it cleanly by importing it into a
new empty database. Do not apply `000` directly to an existing populated
database that already has these objects; treat that database as already being
at the baseline and begin subsequent changes with `001_*.sql`.

Each later migration should be transactional where PostgreSQL permits it and
must be tested against both:

1. A fresh database created from `000` followed by all later migrations.
2. A copy of the existing database beginning at the `000` baseline.
