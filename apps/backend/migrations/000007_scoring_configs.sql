-- +goose Up
CREATE TABLE IF NOT EXISTS scoring_configs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    family VARCHAR(64) NOT NULL,
    version INT NOT NULL DEFAULT 1,
    categories JSONB NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_scoring_configs_active_family 
ON scoring_configs(family) WHERE is_active = true;

CREATE UNIQUE INDEX IF NOT EXISTS idx_scoring_configs_family_version 
ON scoring_configs(family, version);

-- Seed initial v1 configurations
INSERT INTO scoring_configs (family, version, is_active, categories) VALUES
('technology', 1, true, '[
  {"name": "technical_skills", "weight": 0.35, "description": "Required and preferred technical skills and professional capabilities."},
  {"name": "responsibilities", "weight": 0.25, "description": "Alignment between previous work and the responsibilities of the job."},
  {"name": "experience", "weight": 0.20, "description": "Relevant professional experience, not just total years."},
  {"name": "seniority", "weight": 0.15, "description": "Evidence of ownership, autonomy, decision-making, and leadership where relevant."},
  {"name": "domain", "weight": 0.05, "description": "Relevance of industry and domain experience."}
]'::jsonb),
('administration', 1, true, '[
  {"name": "skills", "weight": 0.25, "description": "Administrative competencies, written communication, and data handling."},
  {"name": "responsibilities", "weight": 0.25, "description": "Alignment with office management and administrative support duties."},
  {"name": "experience", "weight": 0.15, "description": "Relevant administrative and clerical work experience."},
  {"name": "seniority", "weight": 0.10, "description": "Independence and organization in day-to-day work."},
  {"name": "job_specific", "weight": 0.20, "description": "Specific tools such as Microsoft Excel, document systems, and records management."},
  {"name": "domain", "weight": 0.05, "description": "Industry relevance."}
]'::jsonb),
('customer_operations', 1, true, '[
  {"name": "skills", "weight": 0.20, "description": "Communication, listening, problem-solving, and customer service skills."},
  {"name": "responsibilities", "weight": 0.25, "description": "Alignment with customer support, complaint handling, and service responsibilities."},
  {"name": "experience", "weight": 0.15, "description": "Relevant professional and customer-facing experience."},
  {"name": "seniority", "weight": 0.10, "description": "Independence, accountability, and complexity of previous work."},
  {"name": "job_specific", "weight": 0.25, "description": "Job-specific requirements such as CRM tools, communication channels, language, and complaint handling."},
  {"name": "domain", "weight": 0.05, "description": "Relevant industry experience."}
]'::jsonb),
('transportation', 1, true, '[
  {"name": "skills", "weight": 0.20, "description": "Vehicle operation, navigation skills, route planning, and safe driving practice."},
  {"name": "responsibilities", "weight": 0.20, "description": "Alignment with delivery schedules, package handling, and vehicle upkeep."},
  {"name": "experience", "weight": 0.15, "description": "Years of driving and commercial delivery experience."},
  {"name": "seniority", "weight": 0.05, "description": "Reliability and track record."},
  {"name": "job_specific", "weight": 0.35, "description": "Required driving licenses, vehicle class endorsements, and local route familiarity."},
  {"name": "domain", "weight": 0.05, "description": "Specific delivery or transportation sector experience."}
]'::jsonb)
ON CONFLICT DO NOTHING;

-- +goose Down
DROP TABLE IF EXISTS scoring_configs;
