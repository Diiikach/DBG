.PHONY: build run up down logs clean

build:
\tdocker compose build

run:
\tdocker compose up

up:
\tdocker compose up -d

down:
\tdocker compose down

logs:
\tdocker compose logs -f

clean:
\tdocker compose down -v --remove-orphans
