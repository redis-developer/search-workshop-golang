.PHONY: help doctor deps redis-start redis-stop prep run reindex reindex-matrix eval study study-deps verify clean

# Directory of the Python optimizer project (Lab 8).
OPTIMIZER_DIR := optimizer

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-16s\033[0m %s\n", $$1, $$2}'

doctor: ## Check workshop prerequisites
	@ok=1; \
	command -v go >/dev/null && echo "  ok  go $$(go version | cut -d' ' -f3)" || { echo "MISS  go (1.25+): https://go.dev/dl/"; ok=0; }; \
	command -v docker >/dev/null && echo "  ok  docker" || { echo "MISS  docker: https://docs.docker.com/get-docker/"; ok=0; }; \
	if [ -n "$$ONNXRUNTIME_LIB_PATH" ] && [ -e "$$ONNXRUNTIME_LIB_PATH" ]; then echo "  ok  onnxruntime ($$ONNXRUNTIME_LIB_PATH)"; \
	elif ls /opt/homebrew/lib/libonnxruntime*.dylib /usr/local/lib/libonnxruntime*.dylib /usr/lib/libonnxruntime*.so /usr/lib/*/libonnxruntime*.so 2>/dev/null | head -1 | grep -q .; then echo "  ok  onnxruntime"; \
	else echo "MISS  onnxruntime: brew install onnxruntime | apt install libonnxruntime (or set ONNXRUNTIME_LIB_PATH)"; ok=0; fi; \
	command -v uv >/dev/null && echo "  ok  uv" || echo "  --  uv (only needed for Lab 8): https://docs.astral.sh/uv/"; \
	[ $$ok -eq 1 ] && echo "All required tools present." || { echo "Fix the MISS items above."; exit 1; }

deps: ## Download Go module dependencies
	go mod tidy

redis-start: ## Start the workshop Redis container
	docker compose up -d redis

redis-stop: ## Stop the workshop Redis container
	docker compose down

prep: ## Download WANDS and build the deterministic workshop sample
	go run ./cmd/prep

run: ## Start the search service (API + UI on the same port)
	go run ./cmd/searchd

reindex: ## Drop and rebuild the index with the current config.yaml, then exit
	go run ./cmd/searchd -reindex-only

reindex-matrix: ## Build the Lab 8 index grid (algorithms x models) for the optimizer study
	go run ./cmd/searchd -reindex-only -algorithm flat
	go run ./cmd/searchd -reindex-only -algorithm hnsw
	go run ./cmd/searchd -reindex-only -algorithm svs-vamana
	go run ./cmd/searchd -reindex-only -algorithm flat -model sentence-transformers/all-mpnet-base-v2
	go run ./cmd/searchd -reindex-only -algorithm hnsw -model sentence-transformers/all-mpnet-base-v2

eval: ## Score the current config against the WANDS judgments (nDCG@10, Recall@25)
	go run ./cmd/eval

study-deps: ## One-time setup of the Python optimizer project (Lab 8)
	cd $(OPTIMIZER_DIR) && uv sync

study: ## Run the Retrieval Optimizer search study over the Go-built indexes
	cd $(OPTIMIZER_DIR) && uv run python study.py

verify: ## Verify the checkpoint for a lab: make verify LAB=3
	@test -n "$(LAB)" || { echo "usage: make verify LAB=<n>"; exit 1; }
	./labs/verify.sh $(LAB)

clean: ## Remove generated data (keeps raw WANDS downloads)
	rm -rf data/corpus.jsonl data/queries.tsv data/qrels.txt
