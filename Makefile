# Distribucion de Ameriboard en el repositorio Debian compartido.
REPO_DIR ?= /var/www/html/repo
PORT ?= 8087
export REPO_DIR

.DEFAULT_GOAL := help
.PHONY: help dist check-repo newtags release copy-repo index-repo clean-repo push-repo dev-backend dev-front run
.NOTPARALLEL:

help:
	@echo "make dist [VERSION=vX.Y.Z] [REPO_DIR=/ruta] - crear y publicar la distribucion"
	@echo "make release | copy-repo | clean-repo | index-repo | push-repo"
	@echo "make dev-backend | dev-front | run"

# No publica ni crea tags hasta que se solicite explicitamente.
dist: check-repo newtags release copy-repo clean-repo push-repo

check-repo:
	@git -C "$(REPO_DIR)" rev-parse --show-toplevel >/dev/null
	@if [ "$$(git -C "$(REPO_DIR)" branch --show-current)" != main ]; then \
		echo "ERROR: el repositorio Debian debe estar en la rama main." >&2; exit 1; \
	fi
	@if ! git -C "$(REPO_DIR)" diff --cached --quiet; then \
		echo "ERROR: el repositorio Debian tiene cambios preparados; resuelvalos antes de publicar." >&2; exit 1; \
	fi
	@if [ -n "$$(git -C "$(REPO_DIR)" status --porcelain -- Packages Packages.gz Release InRelease Release.gpg ':(glob)ameriboard_*.deb')" ]; then \
		echo "ERROR: hay cambios pendientes en los paquetes o indices del repositorio Debian." >&2; exit 1; \
	fi

newtags:
	@if [ -n "$$(git status --porcelain --untracked-files=all)" ]; then \
		echo "ERROR: hay cambios sin commit; no se creara ni publicara el tag." >&2; \
		git status --short; exit 1; \
	fi
	@NEW_TAG="$(VERSION)"; \
	if [ -z "$$NEW_TAG" ]; then \
		CURRENT_TAG=$$(git tag --list 'v[0-9]*' --sort=-v:refname | grep -E '^v[0-9]+\.[0-9]+\.[0-9]+$$' | head -n 1); \
		[ -n "$$CURRENT_TAG" ] || CURRENT_TAG=v1.0.0; \
		case "$$CURRENT_TAG" in v0.*) CURRENT_TAG=v1.0.0;; esac; \
		NEW_TAG=$$(echo "$$CURRENT_TAG" | awk -F. '{print $$1"."$$2"."$$3+1}'); \
	fi; \
	if ! echo "$$NEW_TAG" | grep -Eq '^v[0-9]+\.[0-9]+\.[0-9]+$$'; then \
		echo "ERROR: la version debe tener el formato vX.Y.Z (recibido: $$NEW_TAG)" >&2; exit 1; \
	fi; \
	if git rev-parse -q --verify "refs/tags/$$NEW_TAG" >/dev/null; then \
		echo "ERROR: el tag $$NEW_TAG ya existe" >&2; exit 1; \
	fi; \
	echo "Publicando la rama actual y el tag $$NEW_TAG..."; \
	git push && git tag "$$NEW_TAG" -m "new tag" && git push origin "refs/tags/$$NEW_TAG"

release:
	goreleaser release --clean

copy-repo:
	@set -- dist/ameriboard_*_linux_*.deb; \
	[ -f "$$1" ] || { echo "ERROR: no se encontraron paquetes Debian de Ameriboard en dist/." >&2; exit 1; }; \
	for package do \
		[ ! -e "$(REPO_DIR)/$${package##*/}" ] || { echo "ERROR: el paquete ya existe en el repositorio: $$package" >&2; exit 1; }; \
	done; \
	cp -- "$$@" "$(REPO_DIR)/"

index-repo:
	cd "$(REPO_DIR)" && ./update-repo.sh

clean-repo:
	bash scripts/clean-rep.sh
	$(MAKE) index-repo

push-repo:
	@if [ "$$(git -C "$(REPO_DIR)" branch --show-current)" != main ]; then \
		echo "ERROR: el repositorio Debian debe estar en la rama main." >&2; exit 1; \
	fi
	@if ! git -C "$(REPO_DIR)" diff --cached --quiet; then \
		echo "ERROR: el repositorio Debian tiene cambios preparados ajenos a esta publicacion." >&2; exit 1; \
	fi
	git -C "$(REPO_DIR)" add --all -- README.md Packages Packages.gz Release InRelease Release.gpg ':(glob)ameriboard_*.deb'
	@git -C "$(REPO_DIR)" diff --cached --quiet || \
		git -C "$(REPO_DIR)" commit -m "Actualizar paquetes de ameriboard - $$(git describe --tags --always)"
	git -C "$(REPO_DIR)" push origin main

dev-backend:
	LISTEN_ADDR=127.0.0.1:$(PORT) go run ./cmd/ameriboard

dev-front:
	npm start

run:
	npm run build
	LISTEN_ADDR=127.0.0.1:$(PORT) go run ./cmd/ameriboard
