REGISTRY := 192.168.3.90:5000
NAMESPACE := convocate
IMAGES := openbao redis postgresql minio influxdb prometheus grafana api ui pdv metrics agent fluentbit

.PHONY: all clean lint test build cover deploy

all: build

# ── clean ──────────────────────────────────────────────
clean:
	rm -rf build/
	mkdir -p build/
	-docker rmi $(addprefix $(REGISTRY)/convocate/,$(addsuffix :latest,$(IMAGES))) 2>/dev/null

# ── lint ───────────────────────────────────────────────
lint:
	cd api && gofmt -l . && go vet ./...

# ── test ───────────────────────────────────────────────
test:
	cd api && go test ./...
	cd metrics && go test ./...
	cd agent && go test ./...

# ── build ──────────────────────────────────────────────
build: $(addprefix build-,$(IMAGES))

build-openbao:
	docker build -f docker/openbao.Dockerfile -t $(REGISTRY)/convocate/openbao:latest .

build-redis:
	docker build -f docker/redis.Dockerfile -t $(REGISTRY)/convocate/redis:latest .

build-postgresql:
	docker build -f docker/pg.Dockerfile -t $(REGISTRY)/convocate/postgresql:latest .

build-minio:
	docker build -f docker/minio.Dockerfile -t $(REGISTRY)/convocate/minio:latest .

build-influxdb:
	docker build -f docker/influxdb.Dockerfile -t $(REGISTRY)/convocate/influxdb:latest .

build-prometheus:
	docker build -f docker/prometheus.Dockerfile -t $(REGISTRY)/convocate/prometheus:latest .

build-grafana:
	docker build -f docker/grafana.Dockerfile -t $(REGISTRY)/convocate/grafana:latest .

build-fluentbit:
	docker build -f docker/fluentbit.Dockerfile -t $(REGISTRY)/convocate/fluentbit:latest .

build-api:
	docker build -f docker/api.Dockerfile -t $(REGISTRY)/convocate/api:latest .

build-ui:
	docker build -f docker/ui.Dockerfile -t $(REGISTRY)/convocate/ui:latest .

build-pdv:
	docker build -f docker/pdv.Dockerfile -t $(REGISTRY)/convocate/pdv:latest .

build-metrics:
	docker build -f docker/metrics.Dockerfile -t $(REGISTRY)/convocate/metrics:latest .

build-agent:
	docker build -f docker/agent.Dockerfile -t $(REGISTRY)/convocate/agent:latest .

# ── cover ──────────────────────────────────────────────
cover:
	cd api && go test -coverprofile=build/coverage.out ./... && \
	go tool cover -func=build/coverage.out | tail -1 | awk '{ \
		gsub(/%/, "", $$3); \
		if ($$3+0 < 98) { print "FAIL: coverage " $$3 "% < 98%"; exit 1 } \
		else { print "PASS: coverage " $$3 "%" } \
	}'

# ── push ───────────────────────────────────────────────
push: $(addprefix push-,$(IMAGES))

push-%:
	docker push $(REGISTRY)/convocate/$*:latest

# ── deploy ─────────────────────────────────────────────
deploy: build push k8s-apply k8s-verify
	@echo "Deploy complete."

k8s-apply:
	kubectl apply -f k8s/namespace.yaml
	kubectl apply -f k8s/storage.yaml
	kubectl apply -f k8s/network-policies.yaml
	# Security namespace
	kubectl apply -f k8s/openbao/
	kubectl rollout status deployment/openbao -n security --timeout=120s
	# Data layer namespace
	kubectl apply -f k8s/redis/
	kubectl apply -f k8s/postgresql/
	kubectl apply -f k8s/minio/
	kubectl rollout status deployment/redis -n data-layer --timeout=120s
	kubectl rollout status deployment/postgresql -n data-layer --timeout=120s
	kubectl rollout status deployment/minio -n data-layer --timeout=120s
	# Observability namespace
	kubectl apply -f k8s/influxdb/
	kubectl apply -f k8s/prometheus/
	kubectl apply -f k8s/grafana/
	kubectl rollout status deployment/influxdb -n o11y --timeout=120s
	kubectl rollout status deployment/prometheus -n o11y --timeout=120s
	kubectl rollout status deployment/grafana -n o11y --timeout=120s
	kubectl apply -f k8s/fluentbit/
	kubectl rollout status daemonset/fluent-bit -n o11y --timeout=120s
	kubectl apply -f k8s/metrics/
	kubectl rollout status daemonset/node-metrics -n o11y --timeout=120s
	# Application namespace
	kubectl apply -f k8s/agent/
	kubectl apply -f k8s/api/
	kubectl apply -f k8s/ui/
	kubectl rollout status deployment/convocate-api -n $(NAMESPACE) --timeout=120s
	kubectl rollout status deployment/convocate-ui -n $(NAMESPACE) --timeout=120s

k8s-verify:
	-kubectl delete job verify-openbao -n security 2>/dev/null
	-kubectl delete job verify-redis verify-postgresql verify-minio -n data-layer 2>/dev/null
	-kubectl delete job verify-influxdb -n o11y 2>/dev/null
	-kubectl delete job verify-pdv -n $(NAMESPACE) 2>/dev/null
	kubectl apply -f k8s/openbao/verify-job.yaml
	kubectl apply -f k8s/redis/verify-job.yaml
	kubectl apply -f k8s/postgresql/verify-job.yaml
	kubectl apply -f k8s/minio/verify-job.yaml
	kubectl apply -f k8s/influxdb/verify-job.yaml
	kubectl apply -f k8s/pdv/verify-job.yaml
	kubectl wait --for=condition=complete -n security job/verify-openbao --timeout=180s
	kubectl wait --for=condition=complete -n data-layer job/verify-redis job/verify-postgresql job/verify-minio --timeout=180s
	kubectl wait --for=condition=complete -n o11y job/verify-influxdb --timeout=180s
	kubectl wait --for=condition=complete -n $(NAMESPACE) job/verify-pdv --timeout=180s
	@echo "=== Verification Results ==="
	@kubectl logs job/verify-openbao -n security 2>/dev/null | tail -1
	@kubectl logs job/verify-redis -n data-layer 2>/dev/null | tail -1
	@kubectl logs job/verify-postgresql -n data-layer 2>/dev/null | tail -1
	@kubectl logs job/verify-minio -n data-layer 2>/dev/null | tail -1
	@kubectl logs job/verify-influxdb -n o11y 2>/dev/null | tail -1
	@echo "--- PDV ---"
	@kubectl logs job/verify-pdv -n $(NAMESPACE) 2>/dev/null | tail -3
