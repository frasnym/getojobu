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

.PHONY: all install run test test-integration lint fmt clean

all: install

install:
	$(PIP) install -e .

run:
	$(PY) -m worker.main

test:
	$(PY) -m pytest

test-integration:
	docker compose -f ../../docker-compose.integration.yml up -d --wait
	TEMPORAL_HOST=temporal TEMPORAL_NAMESPACE=getojobu TEMPORAL_TASK_QUEUE_JOB_ANALYZE=job-evaluation \
	  API_BASE_URL=http://api:8080 API_KEY=internal-worker-key \
      OMNIROUTE_BASE_URL=http://omniroute:20128 OMNIROUTE_API_KEY=internal-omniroute-key \
      OMNIROUTE_MODEL=use-combo \
      $(PY) -m pytest tests/ -v --timeout=120
	docker compose -f ../../docker-compose.integration.yml down

lint:
	$(RUFF) check .

fmt:
	$(RUFF) format .

clean:
	rm -rf .venv build dist *.egg-info
