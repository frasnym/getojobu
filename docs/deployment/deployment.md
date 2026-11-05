# Deployment Architecture

## 1. Environments

Use separate environments for:
- local development,
- staging,
- production.

Use separate Neon branches/databases and Clerk instances or clearly separated Clerk environments for staging and production. Do not let staging workers write to production data.

## 2. Deployment targets

| Component | Target | Deployment |
|---|---|---|
| Next.js web | Vercel | Git-based deployment |
| Go API | VPS | Docker Compose |
| Python worker | VPS | Docker Compose |
| Browser worker | VPS | Docker Compose |
| Temporal Server | VPS | Docker Compose with persistent storage |
| Temporal UI | VPS private access only | Docker Compose |
| Caddy | VPS | Public HTTPS ingress/reverse proxy |
| Prometheus/Grafana | VPS/private | Docker Compose or existing monitoring stack |
| Neon | Managed | Application PostgreSQL |
| R2 | Managed | Object storage |

Temporal's persistence database is infrastructure for Temporal and must remain separate from Neon application data.

## 3. Network boundaries

Public ingress should be limited to:
- the Go API HTTPS endpoint,
- any explicitly public web endpoints served by Vercel.

Keep the following private:
- Neon credentials and direct DB access,
- Temporal frontend/gRPC port,
- Temporal UI,
- worker ports,
- metrics/admin endpoints,
- browser debugging ports.

Workers connect outbound to Temporal and required external providers. They do not need public inbound ports.

## 4. Compose services

The VPS Compose stack should include at least:
- `api`
- `worker-python`
- `worker-browser`
- `temporal`
- `temporal-ui` (private)
- Temporal persistence database
- `caddy`
- monitoring services as required

Use health checks, restart policies, resource limits, persistent volumes, and explicit networks. Pin image versions; avoid floating `latest` tags in production.

## 5. Configuration and secrets

Inject secrets at runtime through the VPS secret-management approach. Do not commit `.env` files containing values. Separate credentials by environment and component. The browser worker should not receive database migration credentials or broad administrative tokens.

## 6. Release process

1. Build and test images in CI.
2. Run unit and integration tests.
3. Run database migrations as a controlled one-off job.
4. Deploy API and workers.
5. Verify health checks and Temporal task-queue polling.
6. Run a staging end-to-end workflow.
7. Promote the release to production.
8. Verify logs, metrics, and error rates.

Do not run schema migrations independently from every API container startup.

## 7. Backup and recovery

Define Neon backup/restore procedures and test restoration. Back up Temporal persistence separately. Define R2 lifecycle and recovery policy for generated documents. A database backup alone does not restore external browser sessions or workflow side effects; document reconciliation procedures for in-flight applications.
