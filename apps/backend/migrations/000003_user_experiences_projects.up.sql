-- +goose Up
CREATE TABLE user_experiences (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,

    experience_type VARCHAR(32) NOT NULL,

    organization VARCHAR(128),
    role_title VARCHAR(128),
    location VARCHAR(128),
    description TEXT,

    started_at DATE,
    ended_at DATE,
    is_current BOOLEAN NOT NULL DEFAULT FALSE,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT uq_user_experiences_id_user UNIQUE (id, user_id)
);

CREATE TABLE user_projects (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,

    user_experience_id UUID,

    name VARCHAR(255) NOT NULL CHECK (BTRIM(name) <> ''),

    description TEXT,
    responsibilities TEXT,
    achievements TEXT,

    started_at DATE,
    ended_at DATE,
    is_current BOOLEAN NOT NULL DEFAULT FALSE,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT uq_user_projects_id_user UNIQUE (id, user_id),

    CONSTRAINT fk_user_projects_experience_owner
        FOREIGN KEY (user_experience_id, user_id)
        REFERENCES user_experiences (id, user_id)
        ON DELETE RESTRICT
);

CREATE TABLE user_project_skills (
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    user_project_id UUID NOT NULL,
    user_skill_id UUID NOT NULL,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    PRIMARY KEY (user_project_id, user_skill_id),

    CONSTRAINT fk_project_skills_project_owner
        FOREIGN KEY (user_project_id, user_id)
        REFERENCES user_projects (id, user_id)
        ON DELETE RESTRICT,

    CONSTRAINT fk_project_skills_skill_owner
        FOREIGN KEY (user_skill_id, user_id)
        REFERENCES user_skills (id, user_id)
        ON DELETE RESTRICT
);