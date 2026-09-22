SERVICES := video-processor-api video-processor-worker video-processor-auth

.PHONY: build test test-integration lint validate-dashboards up down e2e

build:
	@for s in $(SERVICES); do (cd services/$$s && go build ./...) || exit 1; done

test:
	@for s in $(SERVICES); do (cd services/$$s && go test -race -count=1 -cover ./...) || exit 1; done

test-integration:
	@for s in $(SERVICES); do (cd services/$$s && go test -race -count=1 -tags integration ./...) || exit 1; done

lint:
	@for s in $(SERVICES); do (cd services/$$s && golangci-lint run --config ../../.golangci.yml --build-tags integration ./...) || exit 1; done
	@cd e2e && golangci-lint run --config ../.golangci.yml --build-tags e2e ./...

validate-dashboards:
	python3 scripts/validate-dashboards.py

up:
	docker compose up -d --build --wait

down:
	docker compose down -v

e2e: up
	docker compose --profile e2e run --rm e2e
