# GetOJobu Python Temporal Worker Makefile
#
# Usage:
#   make install     install dependencies
#   make run         run the Temporal worker (needs DATABASE_URL, TEMPORAL_*)
#   make test        run all tests
#   make lint        run ruff
#   make fmt         run ruff format

PY   ?= python
PIP  ?= pip
RUFF ?= ruff

.PHONY: all install run test lint fmt clean

all: install

install:
	$(PIP) install -e .

run:
	$(PY) -m worker.main

test:
	$(PY) -m pytest

lint:
	$(RUFF) check .

fmt:
	$(RUFF) format .

clean:
	rm -rf .venv build dist *.egg-info
