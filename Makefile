.PHONY: deps web-deps ingest ml web dev dev-modbus

GO ?= go
PYTHON ?= python3
NPM ?= npm
SIM_SOURCE_CONFIG ?= ingest/config/sources.json
MODBUS_SOURCE_CONFIG ?= ingest/config/sources.modbus.json
ML_API_URL ?= http://127.0.0.1:8000
ML_BATCH_FLUSH_INTERVAL ?= 300ms

deps: web-deps

web-deps:
	cd web && $(NPM) install

ingest:
	cd ingest && SOURCE_CONFIG_PATH=$(patsubst ingest/%,%,$(SIM_SOURCE_CONFIG)) ML_API_URL=$(ML_API_URL) ML_BATCH_FLUSH_INTERVAL=$(ML_BATCH_FLUSH_INTERVAL) $(GO) run .

ml:
	$(PYTHON) ml/app/main.py --serve

web:
	cd web && $(NPM) run dev -- --host

dev:
	@trap 'kill 0' INT TERM EXIT; \
	( cd ingest && SOURCE_CONFIG_PATH=$(patsubst ingest/%,%,$(SIM_SOURCE_CONFIG)) ML_API_URL=$(ML_API_URL) ML_BATCH_FLUSH_INTERVAL=$(ML_BATCH_FLUSH_INTERVAL) $(GO) run . ) & \
	( $(PYTHON) ml/app/main.py --serve ) & \
	( cd web && $(NPM) run dev -- --host ) & \
	wait

dev-modbus:
	@trap 'kill 0' INT TERM EXIT; \
	( cd ingest && SOURCE_CONFIG_PATH=$(patsubst ingest/%,%,$(MODBUS_SOURCE_CONFIG)) ML_API_URL=$(ML_API_URL) ML_BATCH_FLUSH_INTERVAL=$(ML_BATCH_FLUSH_INTERVAL) $(GO) run . ) & \
	( $(PYTHON) ml/app/main.py --serve ) & \
	( cd web && $(NPM) run dev -- --host ) & \
	wait
