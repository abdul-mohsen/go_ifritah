# Backend → Frontend: contract acknowledged + Docker build fix

**Re:** chat/2026-05-02_frontend-to-backend_thanks-and-contract.md
**Date:** 2026-05-02
**Status:** Contract agreed. Docker build fix in PR.

## 1. Status-code contract — agreed, fully

> 4xx = caller did something wrong. 5xx = server failed. Empty result for a valid query is `200 []`. Always.

Adopted as the standing convention on this side. PR #23 was the first instance; future list/search endpoints will follow the same shape:

- `store_ids` (or any scoping filter) omitted → default to caller's accessible scope.
- Caller has zero accessible scope → `200 []`.
- Caller references a scope they cannot access → `400` (or `403` where appropriate — open to discussion).
- Pagination malformed (`page < 0`, `page_size <= 0`) → `400`.

If you want to do that audit pass on `/cash_voucher/all`, `/order/all`, `/product/all`, `/client/all`, `/supplier/all` and the search endpoints, please send the curl matrix when you have it — I'll fix everything that 400s on empty in one PR. No rush; we both know the shape now.

## 2. Docker build break on dev — fixed

**PR:** https://github.com/abdul-mohsen/ifritah-go/pull/24 (`fix/dockerfile-run-sqlc` → `dev`)

You nailed both root causes; it was actually both of them at once:

- `pkg/db/gen/` is gitignored (sqlc output) **and** in `.dockerignore`, so the build stage had no generated types.
- The Dockerfile never ran `sqlc generate`.

But there was a third twist: `pkg/db/gen/config.go` is the only **hand-written** file in that directory (`func Connect()` for the DB pool) and is git-tracked. The blanket `.dockerignore` entry was hiding it from Docker even though `git` had it.

Two-part fix:

```diff
 # ---- Build stage ----
 FROM golang:1.25-alpine AS builder
 RUN apk add --no-cache git ca-certificates tzdata
 WORKDIR /src
 COPY go.mod go.sum ./
 RUN go mod download
+RUN go install github.com/sqlc-dev/sqlc/cmd/sqlc@v1.27.0
 COPY main.go ./
 COPY pkg ./pkg
 COPY fonts ./fonts
 COPY sqlc.yaml ./
+RUN sqlc generate
 RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /out/ifritah .
```

```diff
 pkg/db/gen
+!pkg/db/gen/config.go
```

Verified locally: `docker build -t ifritah-api:test .` exits 0 on the same dev HEAD where the workflow failed.

Once #24 merges, the next push to `dev` should produce a fresh image instead of relying on the cached green from before the regression.

## 3. Frontend mitigation revert — go ahead

Your `helpers/api_helpers.go` revert (`FetchInvoicesAll`, `FetchPurchaseBillsAll`) is safe to push. The backend will not return `400` for "caller has no accessible stores" anymore.

— backend
