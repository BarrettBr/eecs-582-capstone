.PHONY: deps web-deps ml-deps ingest ml web dev dev-modbus

GO ?= go
PYTHON ?= python3
NPM ?= npm
VENV_DIR ?= .venv
VENV_PYTHON ?= $(VENV_DIR)/bin/python
ML_PYTHON ?= $(if $(wildcard $(VENV_PYTHON)),$(VENV_PYTHON),$(PYTHON))
SIM_SOURCE_CONFIG ?= ingest/config/sources.json
MODBUS_SOURCE_CONFIG ?= ingest/config/sources.modbus.json
ML_API_URL ?= http://127.0.0.1:8000
ML_BATCH_FLUSH_INTERVAL ?= 300ms

deps: web-deps ml-deps

web-deps:
	cd frontend && $(NPM) install

ml-deps:
	$(PYTHON) -m venv $(VENV_DIR)
	$(VENV_PYTHON) -m pip install -r ml/app/requirements.txt

ingest:
	cd ingest && SOURCE_CONFIG_PATH=$(patsubst ingest/%,%,$(SIM_SOURCE_CONFIG)) ML_API_URL=$(ML_API_URL) ML_BATCH_FLUSH_INTERVAL=$(ML_BATCH_FLUSH_INTERVAL) $(GO) run .

ml:
	$(ML_PYTHON) ml/app/main.py --serve

web:
	cd frontend && $(NPM) run dev -- --host

dev:
	@trap 'kill 0' INT TERM EXIT; \
	( cd ingest && SOURCE_CONFIG_PATH=$(patsubst ingest/%,%,$(SIM_SOURCE_CONFIG)) ML_API_URL=$(ML_API_URL) ML_BATCH_FLUSH_INTERVAL=$(ML_BATCH_FLUSH_INTERVAL) $(GO) run . ) & \
	( $(ML_PYTHON) ml/app/main.py --serve ) & \
	( cd frontend && $(NPM) run dev -- --host ) & \
	wait

dev-modbus:
	@trap 'kill 0' INT TERM EXIT; \
	( cd ingest && SOURCE_CONFIG_PATH=$(patsubst ingest/%,%,$(MODBUS_SOURCE_CONFIG)) ML_API_URL=$(ML_API_URL) ML_BATCH_FLUSH_INTERVAL=$(ML_BATCH_FLUSH_INTERVAL) $(GO) run . ) & \
	( $(ML_PYTHON) ml/app/main.py --serve ) & \
	( cd frontend && $(NPM) run dev -- --host ) & \
	wait
