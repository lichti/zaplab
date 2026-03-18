# ─── Tools ────────────────────────────────────────────────────────────────────
GO             := go
DOCKER_COMPOSE := docker compose

# ─── Version ──────────────────────────────────────────────────────────────────
# Resolved from the nearest git tag (e.g. v1.0.0-beta.1).
# Falls back to "dev" when no tags exist or outside a git repo.
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
SUFFIX  ?=

# Final version string
FULL_VERSION := $(VERSION)$(SUFFIX)

# ─── Binary names ─────────────────────────────────────────────────────────────
BIN_DIR := bin
BINARY  := $(BIN_DIR)/zaplab_$(shell $(GO) env GOOS)_$(shell $(GO) env GOARCH)$(SUFFIX)
SYMLINK := $(BIN_DIR)/zaplab$(SUFFIX)

# ─── Data directory ────────────────────────────────────────────────────────────
# Override via env: ZAPLAB_DATA_DIR=/custom/path make run
# or via flag:      make run DATA_DIR=/custom/path
DATA_DIR ?= $(or $(ZAPLAB_DATA_DIR),$(HOME)/.zaplab)

# ──────────────────────────────────────────────────────────────────────────────
.DEFAULT_GOAL := build
.PHONY: fmt vet deps-download build link build-run run tag tag-push git-init \
        build-img run-docker down clean clean-docker ps logs \
        shell help css

# ─── Go ───────────────────────────────────────────────────────────────────────

fmt:
	$(GO) fmt ./...

vet: fmt
	$(GO) vet ./...

deps-download:
	$(GO) mod download

## css: generate Tailwind CSS (JIT, minified) → pb_public/css/tailwind.css
css:
	node_modules/.bin/tailwindcss -i tailwind.input.css -o pb_public/css/tailwind.css --minify

## build: fmt + vet + compile → bin/zaplab_<OS>_<ARCH>
build: vet deps-download
	mkdir -p $(BIN_DIR)
	$(GO) build -ldflags "-X main.Version=$(FULL_VERSION)" -o $(BINARY) .

## link: create a symlink without the platform suffix
link:
	ln -sf $(notdir $(BINARY)) $(SYMLINK)

## build-run: build then run locally
build-run: build run

## run: execute the compiled binary (port 8090)
run:
	ZAPLAB_DATA_DIR="$(DATA_DIR)" ./$(BINARY) serve --http 0.0.0.0:8090

## tag: create an annotated git tag — usage: make tag TAG=v1.0.0-beta.1
tag:
	@test -n "$(TAG)" || (echo "Usage: make tag TAG=v1.0.0-beta.1" && exit 1)
	git tag -a $(TAG) -m "Release $(TAG)"
	@echo "Tag $(TAG) created. Push with: git push origin $(TAG)"

## tag-push: create and push a git tag — usage: make tag-push TAG=v1.0.0-beta.1
tag-push:
	@test -n "$(TAG)" || (echo "Usage: make tag-push TAG=v1.0.0-beta.1" && exit 1)
	git tag -a $(TAG) -m "Release $(TAG)"
	git push origin $(TAG)

## git-init: initialize git repo, set remote, and push initial commit
git-init:
	git init
	git add .
	git commit -m "feat: initial release"
	git remote add origin git@github.com:lichti/zaplab.git
	git branch -M main
	git push -u origin main

## clean: remove compiled binaries
clean:
	rm -rf ./$(BIN_DIR)

# ─── Docker ───────────────────────────────────────────────────────────────────

## build-img: build the engine Docker image
build-img:
	$(DOCKER_COMPOSE) build engine

## run-docker: start all services in background
run-docker:
	$(DOCKER_COMPOSE) up -d

## down: stop all services
down:
	$(DOCKER_COMPOSE) down

## clean-docker: stop + remove volumes, images and orphans
clean-docker:
	$(DOCKER_COMPOSE) down --volumes --rmi all --remove-orphans

## ps: show container status
ps:
	$(DOCKER_COMPOSE) ps

## logs: follow container logs
logs:
	$(DOCKER_COMPOSE) logs -f

# ─── Shells ───────────────────────────────────────────────────────────────────

## shell: open a bash shell in the engine container
shell:
	$(DOCKER_COMPOSE) exec -it engine bash

# ─── Help ─────────────────────────────────────────────────────────────────────

## help: list documented targets
help:
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/## /  /'
