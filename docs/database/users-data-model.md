# Users Data Model

## 1. Purpose

The `users` table is the application's internal user identity for **go-get-job**.

Authentication is handled by **Clerk**. PostgreSQL stores the internal user record used by application data and foreign keys.

Clerk is responsible for:
- Registration and login
- Passwords and OAuth/social login
- Email verification
- Sessions and authentication
- Clerk-managed identity

The application database is responsible for:
- Internal user identity
- Application-specific data
- Relationships to career profiles, skills, saved jobs, verdicts, applications, subscriptions, and credits
- Account lifecycle and application preferences

Do not store passwords, session tokens, or Clerk authentication secrets in PostgreSQL.

## 2. Identity strategy

Use an internal UUID as the primary key and store the Clerk user ID as a unique external identity.

```text
Clerk user
    |
    | clerk_user_id
    v
users
    |
    | users.id (internal UUID)
    +---- user_profiles
    +---- user_skills
    +---- job_verdicts
    +---- job_applications
    +---- subscriptions
    +---- credit_accounts
```

Why use an internal UUID?

- Application tables are not coupled to a specific authentication provider.
- Clerk IDs remain integration identifiers rather than database primary keys.
- Replacing or adding an identity provider later does not require changing every foreign key.
- All user-owned records use one consistent internal identifier.

Use UUIDs for `users.id`, as already assumed by the career and billing data models. Other tables may use `BIGINT GENERATED ALWAYS AS IDENTITY` for their own primary keys.

## 3. Proposed `users` table

| Column | Type | Nullable | Description |
|---|---|---:|---|
| `id` | `UUID` | No | Internal primary key |
| `clerk_user_id` | `TEXT` | No | Unique Clerk user identifier |
| `email` | `TEXT` | No | Primary email snapshot synchronized from Clerk |
| `created_at` | `TIMESTAMPTZ` | No | Local record creation time |
| `updated_at` | `TIMESTAMPTZ` | No | Last local record update |
| `deleted_at` | `TIMESTAMPTZ` | Yes | Soft deletion/account deactivation timestamp |

### Notes

- `email` is a synchronized snapshot, not the authentication source of truth. Clerk remains authoritative for verified email addresses and authentication.
- Do not store a user name here initially. Names and authentication-facing profile details can remain in Clerk. Add application-specific display/profile fields only when the product needs them.
- `deleted_at` is a recommended account lifecycle field, not a confirmed product policy. It supports deactivation and retention workflows without immediately destroying application history.
- Do not add `plan_id` or `credit_balance` to `users`. Subscription and credit accounting belong in their own tables.
- Do not add `is_active` if `deleted_at` already represents local account deactivation; avoid two overlapping lifecycle flags.

## 4. PostgreSQL migration

Assumes PostgreSQL supports `gen_random_uuid()` and the project has a shared `trigger_updated_at()` function.

```sql
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    clerk_user_id TEXT NOT NULL UNIQUE,
    email TEXT NOT NULL,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE INDEX idx_users_email
    ON users (email);

CREATE TRIGGER trg_users_updated_at
    BEFORE UPDATE ON users
    FOR EACH ROW EXECUTE FUNCTION trigger_updated_at();
```

If email lookup is case-insensitive, normalize email consistently in the application or use a case-insensitive unique/index strategy. Do not make email unique unless the product explicitly requires one local account per email; Clerk identity is keyed by `clerk_user_id`.

## 5. User creation and synchronization

Use Clerk webhooks to synchronize identity changes into the application database.

| Clerk event | Local behavior |
|---|---|
| `user.created` | Create local user and initialize required application records |
| `user.updated` | Synchronize the primary email snapshot |
| `user.deleted` | Start the account deletion/deactivation policy |

Webhook processing must be idempotent. Clerk may retry delivery.

### Registration flow

```text
User registers
    |
    v
Clerk creates identity
    |
    v
Clerk sends user.created webhook
    |
    v
Go API verifies webhook signature
    |
    v
Upsert local users row by clerk_user_id
    |
    v
Create credit_accounts row
    |
    v
Grant one-time welcome credits
```

The user should not be able to grant themselves welcome credits by calling an ordinary API endpoint. The grant must be created by trusted backend logic with a unique idempotency key, for example:

```text
welcome:{internal_user_id}
```

### Webhook reliability and first-request fallback

Do not assume the webhook always arrives before the user's first API request.

For an authenticated request:
1. Verify the Clerk session/token using the supported Clerk verification method.
2. Resolve the local user by `clerk_user_id`.
3. If the local row does not exist, create/synchronize it through a safe idempotent path.
4. Continue only after the internal user record exists.

Never trust a client-supplied `user_id` to determine the authenticated user.

## 6. Account deletion and retention

Recommended initial behavior:

- Mark the local user as deleted/deactivated using `deleted_at`.
- Revoke or invalidate access through Clerk.
- Prevent new product operations for the deleted account.
- Preserve financial records and application history according to the product's retention and legal obligations.
- Define a separate data-erasure/anonymization process where required.

Do not cascade-delete billing transactions, invoices/orders, or application history merely because the Clerk identity was deleted.

The exact retention, anonymization, and hard-deletion policy remains a product/legal decision and should be confirmed before launch.

## 7. Tenant isolation

Every user-owned table should reference `users.id`, not `clerk_user_id`.

Examples:

```sql
user_id UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT
```

For all user-owned API operations:
- Derive the internal user ID from the verified Clerk identity.
- Scope reads, updates, and deletes by that internal user ID.
- Do not rely on opaque UUIDs as authorization.
- Use database constraints as defense in depth.
- Keep shared job records separate from user-owned verdicts, saved jobs, and applications.

Neon does not automatically provide Supabase-style row-level security behavior. Enforce authorization in the Go API and consider PostgreSQL RLS separately if the architecture later calls for it.

## 8. Out of scope

- Password storage
- Session/token storage
- OAuth provider tables managed by Clerk
- Local role/permission system (unless product authorization requires one)
- User names and full profile data
- Subscription status or plan fields
- Credit balances
- Hard deletion and detailed retention automation

## 9. Decisions and open questions

### Current architecture decisions

- Clerk is the authentication provider.
- PostgreSQL has an internal `users` record.
- `users.id` is an internal UUID primary key.
- `clerk_user_id` is unique.
- Application foreign keys use `users.id`.
- Passwords and session management stay in Clerk.

### Recommended but not yet explicitly confirmed

- Keep a synchronized primary `email` snapshot.
- Keep `deleted_at` for soft deactivation.
- Keep names and authentication profile details in Clerk initially.
- Create the local user idempotently from Clerk webhooks, with a first-request fallback.

### To decide before production

- Whether local email must be unique.
- Account deletion, anonymization, and retention periods.
- Whether users can change their primary email and how quickly the snapshot must synchronize.
