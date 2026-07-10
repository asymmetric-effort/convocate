REGISTRY := 192.168.3.90:5000
NAMESPACE := convocate
IMAGES := openbao redis postgresql minio influxdb prometheus grafana api ui pdv metrics agent fluentbit
IMAGE_TAG := sha-$(shell git rev-parse --short HEAD)
SEMVER := $(or $(shell git describe --tags --abbrev=0 2>/dev/null | awk -F. '{printf "%s.%s.%d", $$1, $$2, $$3+1}'),v2.0.1)

.PHONY: all clean lint test test-metrics build cover deploy tag registry-cleanup setup

all: build

# ── setup ──────────────────────────────────────────────
setup:
	./scripts/install-hooks.sh

# ── tag ───────────────────────────────────────────────
tag:
	@echo "IMAGE_TAG=$(IMAGE_TAG)"
	@echo "SEMVER=$(SEMVER)"

# ── clean ──────────────────────────────────────────────
clean:
	rm -rf build/
	mkdir -p build/
	-docker rmi $(addprefix $(REGISTRY)/convocate/,$(addsuffix :latest,$(IMAGES))) 2>/dev/null
	-docker rmi $(addprefix $(REGISTRY)/convocate/,$(addsuffix :$(IMAGE_TAG),$(IMAGES))) 2>/dev/null
	-docker rmi $(addprefix $(REGISTRY)/convocate/,$(addsuffix :$(SEMVER),$(IMAGES))) 2>/dev/null

# ── lint ───────────────────────────────────────────────
lint:
	cd api && gofmt -l . && go vet ./...

# ── test ───────────────────────────────────────────────
test:
	cd api && go test ./...
	cd metrics && go test ./...
	cd agent && go test ./...

# ── test-metrics ──────────────────────────────────────────
test-metrics:
	./scripts/test-metrics.sh

# ── build ──────────────────────────────────────────────
build: $(addprefix build-,$(IMAGES))

build-openbao:
	docker build -f docker/openbao.Dockerfile \
		-t $(REGISTRY)/convocate/openbao:$(IMAGE_TAG) \
		-t $(REGISTRY)/convocate/openbao:latest \
		-t $(REGISTRY)/convocate/openbao:$(SEMVER) .

build-redis:
	docker build -f docker/redis.Dockerfile \
		-t $(REGISTRY)/convocate/redis:$(IMAGE_TAG) \
		-t $(REGISTRY)/convocate/redis:latest \
		-t $(REGISTRY)/convocate/redis:$(SEMVER) .

build-postgresql:
	docker build -f docker/pg.Dockerfile \
		-t $(REGISTRY)/convocate/postgresql:$(IMAGE_TAG) \
		-t $(REGISTRY)/convocate/postgresql:latest \
		-t $(REGISTRY)/convocate/postgresql:$(SEMVER) .

build-minio:
	docker build -f docker/minio.Dockerfile \
		-t $(REGISTRY)/convocate/minio:$(IMAGE_TAG) \
		-t $(REGISTRY)/convocate/minio:latest \
		-t $(REGISTRY)/convocate/minio:$(SEMVER) .

build-influxdb:
	docker build -f docker/influxdb.Dockerfile \
		-t $(REGISTRY)/convocate/influxdb:$(IMAGE_TAG) \
		-t $(REGISTRY)/convocate/influxdb:latest \
		-t $(REGISTRY)/convocate/influxdb:$(SEMVER) .

build-prometheus:
	docker build -f docker/prometheus.Dockerfile \
		-t $(REGISTRY)/convocate/prometheus:$(IMAGE_TAG) \
		-t $(REGISTRY)/convocate/prometheus:latest \
		-t $(REGISTRY)/convocate/prometheus:$(SEMVER) .

build-grafana:
	docker build -f docker/grafana.Dockerfile \
		-t $(REGISTRY)/convocate/grafana:$(IMAGE_TAG) \
		-t $(REGISTRY)/convocate/grafana:latest \
		-t $(REGISTRY)/convocate/grafana:$(SEMVER) .

build-fluentbit:
	docker build -f docker/fluentbit.Dockerfile \
		-t $(REGISTRY)/convocate/fluentbit:$(IMAGE_TAG) \
		-t $(REGISTRY)/convocate/fluentbit:latest \
		-t $(REGISTRY)/convocate/fluentbit:$(SEMVER) .

build-api:
	docker build -f docker/api.Dockerfile \
		-t $(REGISTRY)/convocate/api:$(IMAGE_TAG) \
		-t $(REGISTRY)/convocate/api:latest \
		-t $(REGISTRY)/convocate/api:$(SEMVER) .

build-ui:
	docker build -f docker/ui.Dockerfile \
		-t $(REGISTRY)/convocate/ui:$(IMAGE_TAG) \
		-t $(REGISTRY)/convocate/ui:latest \
		-t $(REGISTRY)/convocate/ui:$(SEMVER) .

build-pdv:
	docker build -f docker/pdv.Dockerfile \
		-t $(REGISTRY)/convocate/pdv:$(IMAGE_TAG) \
		-t $(REGISTRY)/convocate/pdv:latest \
		-t $(REGISTRY)/convocate/pdv:$(SEMVER) .

build-metrics:
	docker build -f docker/metrics.Dockerfile \
		-t $(REGISTRY)/convocate/metrics:$(IMAGE_TAG) \
		-t $(REGISTRY)/convocate/metrics:latest \
		-t $(REGISTRY)/convocate/metrics:$(SEMVER) .

build-agent:
	docker build -f docker/agent.Dockerfile \
		-t $(REGISTRY)/convocate/agent:$(IMAGE_TAG) \
		-t $(REGISTRY)/convocate/agent:latest \
		-t $(REGISTRY)/convocate/agent:$(SEMVER) .

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
	docker push $(REGISTRY)/convocate/$*:$(IMAGE_TAG)
	docker push $(REGISTRY)/convocate/$*:latest
	docker push $(REGISTRY)/convocate/$*:$(SEMVER)

# ── registry-cleanup ──────────────────────────────────
registry-cleanup:
	@for img in $(IMAGES); do \
		echo "=== $$img ==="; \
		tags=$$(curl -s http://$(REGISTRY)/v2/convocate/$$img/tags/list | jq -r '.tags // [] | .[]' 2>/dev/null | sort -V); \
		count=$$(echo "$$tags" | grep -c . 2>/dev/null || echo 0); \
		if [ "$$count" -le 20 ]; then \
			echo "  $$count tags, nothing to prune"; \
			continue; \
		fi; \
		old=$$(echo "$$tags" | head -n -20); \
		for t in $$old; do \
			digest=$$(curl -s -H "Accept: application/vnd.docker.distribution.manifest.v2+json" \
				-I "http://$(REGISTRY)/v2/convocate/$$img/manifests/$$t" 2>/dev/null \
				| grep -i docker-content-digest | awk '{print $$2}' | tr -d '\r'); \
			if [ -n "$$digest" ]; then \
				echo "  deleting $$t ($$digest)"; \
				curl -s -X DELETE "http://$(REGISTRY)/v2/convocate/$$img/manifests/$$digest" >/dev/null; \
			fi; \
		done; \
	done

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
	kubectl apply -f k8s/ginger/
	kubectl apply -f k8s/grafana/
	kubectl rollout status deployment/influxdb -n o11y --timeout=120s
	kubectl rollout status deployment/prometheus -n o11y --timeout=120s
	kubectl rollout status deployment/ginger -n o11y --timeout=120s
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
	# Synthetic monitoring
	kubectl apply -f k8s/pdv/synthetic-cronjob.yaml

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
