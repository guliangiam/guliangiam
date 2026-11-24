#!/bin/bash

if [ -z "${driverName:-}" ]; then
  export driverName=postgres
fi
if [ -z "${dataSourceName:-}" ]; then
  export dataSourceName="user=guliangiam password=guliangiam host=db port=5432 sslmode=disable dbname=guliangiam"
fi
if [ -z "${DATABASE_URL:-}" ]; then
  export DATABASE_URL="postgres://guliangiam:guliangiam@db:5432/guliangiam?sslmode=disable"
fi

exec /server
