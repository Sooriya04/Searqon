FROM postgres:15-alpine

# Copy the initialization SQL script into docker-entrypoint-initdb.d
# Postgres automatically runs any .sql files in this directory on first container startup
COPY init.sql /docker-entrypoint-initdb.d/
