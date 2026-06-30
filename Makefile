REGISTRY := 192.168.3.90:5000
NAMESPACE := convocate
IMAGES := openbao redis postgresql minio api ui pdv metrics

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

build-api:
	docker build -f docker/api.Dockerfile -t $(REGISTRY)/convocate/api:latest .

build-ui:
	docker build -f docker/ui.Dockerfile -t $(REGISTRY)/convocate/ui:latest .

build-pdv:
	docker build -f docker/pdv.Dockerfile -t $(REGISTRY)/convocate/pdv:latest .

build-metrics:
	docker build -f docker/metrics.Dockerfile -t $(REGISTRY)/convocate/metrics:latest .

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
	kubectl apply -f k8s/openbao/
	kubectl rollout status deployment/openbao -n $(NAMESPACE) --timeout=120s
	kubectl apply -f k8s/redis/
	kubectl apply -f k8s/postgresql/
	kubectl apply -f k8s/minio/
	kubectl rollout status deployment/redis -n $(NAMESPACE) --timeout=120s
	kubectl rollout status deployment/postgresql -n $(NAMESPACE) --timeout=120s
	kubectl rollout status deployment/minio -n $(NAMESPACE) --timeout=120s
	kubectl apply -f k8s/metrics/
	kubectl rollout status daemonset/node-metrics -n $(NAMESPACE) --timeout=120s
	kubectl apply -f k8s/api/
	kubectl apply -f k8s/ui/
	kubectl rollout status deployment/convocate-api -n $(NAMESPACE) --timeout=120s
	kubectl rollout status deployment/convocate-ui -n $(NAMESPACE) --timeout=120s

k8s-verify:
	-kubectl delete job verify-openbao verify-redis verify-postgresql verify-minio verify-pdv -n $(NAMESPACE) 2>/dev/null
	kubectl apply -f k8s/openbao/verify-job.yaml
	kubectl apply -f k8s/redis/verify-job.yaml
	kubectl apply -f k8s/postgresql/verify-job.yaml
	kubectl apply -f k8s/minio/verify-job.yaml
	kubectl apply -f k8s/pdv/verify-job.yaml
	kubectl wait --for=condition=complete -n $(NAMESPACE) \
		job/verify-openbao job/verify-redis job/verify-postgresql job/verify-minio job/verify-pdv \
		--timeout=180s
	@echo "=== Verification Results ==="
	@kubectl logs job/verify-openbao -n $(NAMESPACE) 2>/dev/null | tail -1
	@kubectl logs job/verify-redis -n $(NAMESPACE) 2>/dev/null | tail -1
	@kubectl logs job/verify-postgresql -n $(NAMESPACE) 2>/dev/null | tail -1
	@kubectl logs job/verify-minio -n $(NAMESPACE) 2>/dev/null | tail -1
	@echo "--- PDV ---"
	@kubectl logs job/verify-pdv -n $(NAMESPACE) 2>/dev/null | tail -3
