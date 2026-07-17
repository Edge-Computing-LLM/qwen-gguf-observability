PYTHON ?= /usr/local/bin/python3.11

.PHONY: check test validate snapshot

check: test
	$(PYTHON) -m py_compile scripts/qwen_observe.py
	git diff --check

test:
	$(PYTHON) -m unittest discover -s tests -v

validate:
	$(PYTHON) scripts/qwen_observe.py validate

snapshot:
	$(PYTHON) scripts/qwen_observe.py snapshot --output evidence/live-snapshot.json
	$(PYTHON) scripts/qwen_observe.py report --input evidence/live-snapshot.json --output evidence/live-report.md
