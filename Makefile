# GetOJobu Web (Next.js) Makefile
#
# Web deploys to Vercel via git-based deployment (see docs/deployment/deployment.md).
#
# Usage:
#   make web-install    install Node dependencies in apps/web
#   make web-build      build the Next.js app for production
#   make web-typecheck  run TypeScript type checking
#   make web-lint       run Next.js ESLint
#   make web-test       run the vitest suite
#   make web-check      run typecheck, lint, and test
#   make web-deploy-prod build, run checks, then deploy to Vercel (vercel CLI required)
#   make web-clean      remove Next.js build output

WEB_DIR   ?= apps/web
NPM       ?= npm
NODE      ?= node
VERCEL    ?= vercel

.PHONY: web-all web-install web-build web-typecheck web-lint web-test web-check web-deploy-prod web-clean

web-all: web-build

web-install:
	cd $(WEB_DIR) && $(NPM) ci

web-build:
	cd $(WEB_DIR) && $(NPM) run build

web-typecheck:
	cd $(WEB_DIR) && $(NPM) run typecheck

web-lint:
	cd $(WEB_DIR) && $(NPM) run lint

web-test:
	cd $(WEB_DIR) && $(NPM) run test

web-check: web-typecheck web-lint web-test

web-deploy-prod: web-check web-build
	cd $(WEB_DIR) && $(VERCEL) --prod

web-clean:
	cd $(WEB_DIR) && $(NPM) run build 2>/dev/null; rm -rf $(WEB_DIR)/.next $(WEB_DIR)/out
