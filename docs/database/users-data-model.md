# Users Data Model

## 1. Purpose

The `users` table stores the application's internal account record for **go-get-job**.

Authentication is handled by **Clerk**. PostgreSQL stores the internal user identity used by application data, authorization, and foreign keys.

Clerk is responsible for:

- Registration and login
- Passwords and OAuth/social login
- Email verification
- Sessions and authentication identity

The application database is responsible for:

- Internal user identity
- Application role and authorization state
- User-owned application data and relationships
- Account lifecycle
- A local snapshot of basic user information needed by the UI

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

## 3. Users table

| Column                    | Type          | Nullable | Description                                    |
| ------------------------- | ------------- | -------: | ---------------------------------------------- |
| `id`                      | `UUID`        |       No | Internal primary key                           |
| `clerk_user_id`           | `TEXT`        |       No | Unique Clerk user identifier                   |
| `email`                   | `TEXT`        |      Yes | Email snapshot synchronized from Clerk         |
| `full_name`               | `TEXT`        |      Yes | Full name snapshot synchronized from Clerk     |
| `role`                    | `user_role`   |       No | Application role; defaults to `user`           |
| `onboarding_completed_at` | `TIMESTAMPTZ` |      Yes | When the required career profile was completed |
| `created_at`              | `TIMESTAMPTZ` |       No | Local record creation time                     |
| `updated_at`              | `TIMESTAMPTZ` |       No | Last local record update                       |
| `deleted_at`              | `TIMESTAMPTZ` |      Yes | Account deactivation timestamp                 |

### PostgreSQL migration

Assumes PostgreSQL supports `gen_random_uuid()` and the project has a shared `trigger_updated_at()` function.

```sql
CREATE TYPE user_role AS ENUM (
    'user',
    'admin'
);

CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    clerk_user_id TEXT NOT NULL UNIQUE,
    email TEXT,
    full_name TEXT,

    role user_role NOT NULL DEFAULT 'user',

    onboarding_completed_at TIMESTAMPTZ,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE INDEX idx_users_email ON users (email);

CREATE TRIGGER trg_users_updated_at
    BEFORE UPDATE ON users
    FOR EACH ROW EXECUTE FUNCTION trigger_updated_at();
```

### Field decisions

- `email` and `full_name` are local snapshots synchronized from Clerk. Clerk remains authoritative for authentication and verified email status.
- Store only `full_name`; do not split it into `first_name` and `last_name` in the global user record. Name conventions vary. Collect application-specific first and last names only when a particular job application form requires them.
- Do not store `display_name` or `image_url`.
- Do not persist `timezone` initially. The frontend can detect the browser timezone and pass it to the API when date/time presentation or scheduling requires it.
- The UI language is English only, so there is no need for a `locale` column.
- Do not add subscription plan or credit balance fields to `users`; those belong in billing tables.
- `deleted_at` is a recommended account lifecycle field. The final retention and erasure policy must be defined separately.

Email is intentionally nullable and indexed but not unique. Use `clerk_user_id` as the identity key. Do not merge accounts based only on matching email addresses.

## 4. Roles and admin account policy

The application has two mutually exclusive roles initially:

| Role    | Access                                        |
| ------- | --------------------------------------------- |
| `user`  | User-facing application                       |
| `admin` | Admin dashboard and administrative operations |

An admin account is a separate account created with a different email address from a regular user account. An `admin` account does **not** implicitly receive user-facing application access.

Authorization rules:

1. Every newly registered account receives `user` by default.
2. Admin accounts must be provisioned or promoted through trusted backend/admin operations.
3. Never accept role changes from ordinary frontend requests.
4. Enforce authorization in Go API middleware/handlers. Hiding UI elements is not authorization.
5. Keep Clerk authentication separate from application roles. PostgreSQL is the source of truth for the application's role.
6. Do not automatically merge accounts that share an email address.

The separate-email requirement is an account provisioning policy. The local `email` column remains non-unique; `clerk_user_id` is the database identity constraint. If a future policy requires database-enforced unique emails, define normalization and verified-email semantics first.

A two-value enum is sufficient for the initial release. Introduce a role/permission mapping table only if the product needs multiple independently assignable roles or granular permissions.

## 5. Career profile and onboarding

A career profile is mandatory for regular users before they can use the core job-matching experience.

`onboarding_completed_at` records the first time the user completes the required career profile:

- `NULL`: required career profile has not yet been completed.
- Non-`NULL`: the required career profile was completed at this timestamp.

The Go API should set this timestamp after validating that all required career-profile fields are present and saved successfully. It must not be set directly by the frontend.

Treat this timestamp as a record of completion, not as the sole source of truth for current profile validity. If profile requirements change or the profile later becomes incomplete, validate the actual career-profile data before running dependent workflows.

Admin accounts do not need to complete a career profile merely to access the admin dashboard.

## 6. User creation and Clerk synchronization

Use Clerk webhooks to synchronize identity changes into the application database.

| Clerk event    | Local behavior                                                                        |
| -------------- | ------------------------------------------------------------------------------------- |
| `user.created` | Create local user with default role `user`; synchronize available email and full name |
| `user.updated` | Synchronize email and full name snapshots                                             |
| `user.deleted` | Start the account deletion/deactivation policy                                        |

Webhook processing must be idempotent because Clerk may retry delivery.

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
Create local users row by clerk_user_id
    |
    v
Create required account-level records
    |
    v
Grant one-time welcome credits (if enabled by billing policy)
```

The user must not be able to grant themselves welcome credits through an ordinary API endpoint. The grant must be created by trusted backend logic with a unique idempotency key, for example:

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

Never trust a client-supplied `user_id` to determine the authenticated user. Never allow this fallback path to assign an admin role.

## 7. Account deletion and retention

Recommended initial behavior:

- Mark the local user as deleted/deactivated using `deleted_at`.
- Revoke or invalidate access through Clerk.
- Prevent new product operations for the deleted account.
- Preserve financial records and application history according to product retention and legal obligations.
- Define a separate data-erasure/anonymization process where required.

Do not cascade-delete billing transactions, invoices/orders, or application history merely because the Clerk identity was deleted.

The exact retention, anonymization, and hard-deletion policy remains a product/legal decision and should be confirmed before launch.

## 8. Tenant isolation

Every user-owned table should reference `users.id`, not `clerk_user_id`.

Example:

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

## 9. Out of scope

- Password storage
- Session/token storage
- OAuth provider tables managed by Clerk
- First/last name in the global user record
- Display name or profile image URL
- Persisted timezone and locale
- Multiple roles per account
