-- db/mysql/schema.sql
-- Skema gabungan (init MySQL load semua migration)

SOURCE /docker-entrypoint-initdb.d/migrations/0001_init.sql;
SOURCE /docker-entrypoint-initdb.d/migrations/0002_indexes.sql;
SOURCE /docker-entrypoint-initdb.d/migrations/0003_sample_data.sql;
