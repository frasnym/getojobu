# Getojobu Job Matching Pipeline

## Implementation Reference: CV/JD Extraction, Compatibility Evaluation, Scoring, and Reporting

**Status:** Proposed v1 implementation\
**Audience:** Getojobu backend implementation\
**Primary stack:** Go, PostgreSQL, reasoning LLM\
**Optional future component:** Jev 1.13

------------------------------------------------------------------------

## 1. Purpose

This document describes a practical v1 architecture for evaluating how
well a candidate matches a job description (JD).

The design intentionally avoids building a large deterministic
skill-matching engine. A reasoning LLM handles document understanding
and semantic evaluation. Go handles predictable calculations and
validation. PostgreSQL stores reusable extracted profiles, scoring
configurations, evaluations, and reports.

The v1 pipeline has four steps:

1.  **Extract and classify** candidate CV and job description.
2.  **Evaluate compatibility** using a reasoning LLM.
3.  **Calculate score and eligibility** using Go.
4.  **Generate the final report** using a reasoning LLM.

Jev 1.13 is not required for v1. It can later be tested as an
alternative evaluator in Step 2.

### Design principles

-   Let the LLM understand the meaning of CVs and job descriptions.
-   Let Go perform arithmetic, validation, and deterministic application
    logic.
-   Do not treat missing CV information as proof that a candidate lacks
    a skill.
-   Keep mandatory eligibility checks separate from the compatibility
    percentage.
-   Preserve evidence and source references so results are explainable.
-   Cache CV and JD extraction; do not repeat extraction for every
    match.
-   Version prompts, model settings, and scoring configurations.

------------------------------------------------------------------------

## 2. End-to-end architecture

``` mermaid
flowchart TD
    A["User uploads CV"] --> B["CV Extraction - LLM"]
    C["User submits JD"] --> D["JD Extraction - LLM"]
    B --> E[("candidate_profiles")]
    D --> F[("job_profiles")]
    F --> G["Determine job family"]
    G --> H[("scoring_configs")]
    E --> I["Compatibility Evaluation - LLM"]
    F --> I
    H --> I
    I --> J[("matching_evaluations")]
    J --> K["Go Scoring and Validation"]
    H --> K
    K --> L[("matching_results")]
    E --> M["Report Generation - LLM"]
    F --> M
    J --> M
    L --> M
    M --> N[("match_reports")]
```

### Data flow

  ------------------------------------------------------------------------
  Step              Input             Output             Component
  ----------------- ----------------- ------------------ -----------------
  1A. Analyze CV    Original CV text  Normalized         Reasoning LLM
                                      candidate profile  

  1B. Analyze job   Original JD text  Normalized job     Reasoning LLM
                                      profile and job    
                                      family             

  2\. Evaluate      Candidate         Category scores,   Reasoning LLM
                    profile, job      evidence, gaps,    
                    profile,          mandatory checks   
                    evaluation config                    

  3\. Calculate     Evaluation        Weighted score,    Go
                    output, scoring   eligibility status 
                    config                               

  4\. Report        Profiles,         Candidate-facing   Reasoning LLM
                    evaluation,       report             
                    calculated result                    
  ------------------------------------------------------------------------

Step 1A and 1B should be independent operations. A candidate profile can
be reused across many jobs, and a job profile can be reused across many
candidates.

------------------------------------------------------------------------

## 3. Placeholder reference: where prompt variables come from

The placeholders in prompts are variables populated by the Getojobu
backend. They are not special LLM syntax.

  ---------------------------------------------------------------------------
  Placeholder                 Meaning                 Source
  --------------------------- ----------------------- -----------------------
  `{{candidate_cv}}`          Original candidate CV   User upload, after text
                              text                    extraction

  `{{job_description}}`       Original job            User input or imported
                              description text        job

  `{{candidate_profile}}`     Normalized candidate    Step 1A output, stored
                              information             in PostgreSQL

  `{{job_profile}}`           Normalized job          Step 1B output, stored
                              information             in PostgreSQL

  `{{evaluation_config}}`     Categories, weights,    PostgreSQL scoring
                              and evaluation guidance configuration
                              for the job family      

  `{{evaluation_results}}`    LLM category            Step 2 output
                              evaluations and         
                              requirement assessments 

  `{{scoring_results}}`       Overall score and       Step 3 Go output
                              category score          
                              breakdown               

  `{{eligibility_results}}`   Aggregated mandatory    Step 3 Go output
                              requirement status      
  ---------------------------------------------------------------------------

### Example prompt assembly

The backend loads the values and renders the prompt template before
calling the model.

``` go
promptData := map[string]any{
    "candidate_profile": candidateProfile,
    "job_profile":       jobProfile,
    "evaluation_config": evaluationConfig,
}
```

Use the prompt templating approach already adopted by the application.
Validate that all required variables are present before making an LLM
request.

Do not pass a Go map containing arbitrary user-controlled keys into a
prompt template without controlling the template and expected fields.

------------------------------------------------------------------------

# 4. Step 1A --- Analyze candidate CV

**Purpose:** Convert the candidate's CV into a reusable, structured
profile.

Run this when a candidate uploads a CV or explicitly requests a profile
refresh. Store the result and associate it with the CV version used to
generate it.

### Input placeholder

  Variable             Example source
  -------------------- -------------------------------------------
  `{{candidate_cv}}`   Extracted text from the uploaded PDF/DOCX

### Prompt: `extract-cv-v1`

``` text
# Role

You are a professional recruitment data extraction system.

Your task is to extract and normalize factual candidate information from a CV.

You must extract facts and supporting evidence. You must NOT evaluate compatibility with a job or invent missing information.

# Input

## Candidate CV

{{candidate_cv}}

# Instructions

## 1. Candidate information

Extract the following when available:
- Professional experience and employment dates.
- Job titles and employers.
- Responsibilities.
- Projects and achievements.
- Skills and technologies.
- Education.
- Certifications.
- Languages.
- Licenses and qualifications.
- Industry or domain experience.

Do not infer sensitive personal characteristics. Do not use age, gender, ethnicity, religion, disability, marital status, or other protected or irrelevant personal characteristics in a candidate assessment.

## 2. Employment history and years of experience

Extract each employment period separately.

Calculate total professional experience only when the dates support a reasonable calculation. Account for overlapping employment periods so that overlapping months are not counted twice.

If dates are incomplete or ambiguous, return null for the total rather than guessing.

Do not assume all professional experience is relevant to every future job.

## 3. Skills

For each skill:
- Preserve the original term.
- Provide a normalized name where reasonably clear.
- Identify the evidence type.
- Include the source location or a short supporting excerpt.

Use these evidence types:
- demonstrated: supported by a project, responsibility, or achievement.
- self_reported: listed as a skill but not supported by work details.
- inferred: indirectly suggested by evidence; use sparingly.
- unknown: cannot be established from the CV.

Normalize obvious aliases:
- Go, Golang -> golang
- Postgres, PostgreSQL -> postgresql
- Node.js, NodeJS -> nodejs
- MS Excel, Microsoft Excel -> microsoft_excel

Do not normalize distinct products into one skill:
- AWS is not GCP.
- Kafka is not RabbitMQ.
- React is not Vue.
- MySQL is not PostgreSQL.

## 4. Responsibilities and achievements

Preserve what the candidate actually did.

Do not turn participation into ownership. For example, "worked on a payment service" does not prove "designed the payment architecture."

Keep measurable results only when stated in the CV.

## 5. Missing information

Use null or an empty array when information is unavailable, as appropriate.

Missing information does not mean the candidate lacks the qualification.

# Output

Return valid JSON matching the application's candidate profile schema.

Do not include Markdown, commentary, job matching scores, recommendations, or hiring decisions.
```

### Example input CV

This is illustrative test data, not a claim that every detail is present
in a real CV.

``` text
Experience:

Mid-Level Backend Engineer — Amartha
April 2023 - Present
- Works on backend services using Golang.
- Works on transaction processing and message broker integration.
- Works on wallet and agent services.

Backend Engineer — Sinarmas Mining
January 2021 - January 2022

Full-Stack Engineer — Eklanku
October 2017 - December 2020

Skills:
Golang, Node.js, Python, PostgreSQL, MySQL, Redis,
Kafka, RabbitMQ, gRPC, microservices, GCP,
Prometheus, Grafana.
```

### Example output (abbreviated)

``` json
{
  "years_experience": 6,
  "skills": [
    {
      "original_name": "Golang",
      "normalized_name": "golang",
      "evidence_type": "self_reported",
      "source": "CV skills section"
    },
    {
      "original_name": "Kafka",
      "normalized_name": "kafka",
      "evidence_type": "demonstrated",
      "source": "Amartha experience: message broker integration"
    }
  ],
  "experience": [
    {
      "company": "Amartha",
      "role": "Mid-Level Backend Engineer",
      "start_date": "2023-04",
      "end_date": null,
      "responsibilities": [
        "Backend services using Golang",
        "Transaction processing",
        "Message broker integration",
        "Wallet and agent services"
      ]
    }
  ]
}
```

The abbreviated output is for illustration. Define and enforce a
complete JSON Schema in the application.

------------------------------------------------------------------------

# 5. Step 1B --- Analyze job description

**Purpose:** Extract job requirements, classify the job family, and
identify seniority and mandatory conditions.

Run this when a job is submitted or imported. Store the normalized
result against the source JD and its version.

### Input placeholder

  -----------------------------------------------------------------------
  Variable                            Example source
  ----------------------------------- -----------------------------------
  `{{job_description}}`               Text pasted by the user or
                                      extracted from a job listing

  -----------------------------------------------------------------------

### Prompt: `extract-job-v1`

``` text
# Role

You are a professional job description analysis system.

Your task is to extract and normalize information from a job description.

You must identify the job family, seniority, responsibilities, and requirements.

You must NOT evaluate a candidate or calculate compatibility scores.

# Input

## Job Description

{{job_description}}

# Instructions

## 1. Job information

Extract:
- Job title.
- Employer, if stated.
- Location.
- Work arrangement (remote, hybrid, onsite, or unknown).
- Employment type, if stated.
- Seniority level.
- Minimum years of experience, if explicitly required.

Use null or unknown when information is not available.

## 2. Job family classification

Choose the most appropriate family.

Initial supported families:
- technology
- administration
- customer_operations
- transportation
- sales
- marketing
- finance
- human_resources
- engineering
- healthcare
- hospitality
- education
- other

Use `other` if none fits and provide a short explanation.

Do not classify based on the title alone. Consider the actual responsibilities and requirements.

## 3. Requirements

Extract individual requirements, including:
- Skills and competencies.
- Experience.
- Education.
- Certifications.
- Licenses.
- Language proficiency.
- Work authorization.
- Other qualifications.

For each requirement, preserve the original wording and provide:
- A normalized name, where appropriate.
- A category.
- An importance level.
- A short source excerpt.

Use these importance levels:
- required: clearly mandatory.
- preferred: explicitly desirable but not mandatory.
- optional: an additional advantage.
- unknown: importance cannot be determined.

Do not assume every bullet under a "Requirements" heading is mandatory.

## 4. Responsibilities

Extract the actual work the person is expected to perform.

Represent responsibilities as clear, separate items. Do not turn them into skills unless the JD explicitly describes them as qualifications.

## 5. Seniority

Use one of:
- entry
- junior
- mid
- senior
- lead
- manager
- executive
- unknown

Use the job's stated level and responsibility scope. Do not infer seniority from years alone.

## 6. Normalization

Normalize obvious aliases where appropriate.

Examples:
- Go, Golang -> golang
- Postgres, PostgreSQL -> postgresql
- Node.js, NodeJS -> nodejs
- MS Excel, Microsoft Excel -> microsoft_excel

Do not treat related but different products as equivalent.

## 7. Accuracy

Do not invent requirements that are not in the JD.

If a requirement is ambiguous, preserve the original wording and mark its importance as unknown.

# Output

Return valid JSON matching the application's job profile schema.

Do not include Markdown, commentary, candidate assessments, compatibility scores, or recommendations.
```

### Example input JD

``` text
Position: Senior Backend Engineer

Requirements:
- 5+ years of backend development experience.
- Strong Golang experience.
- PostgreSQL and database optimization.
- Experience with Kafka.
- Experience building distributed systems.
- AWS and Kubernetes are preferred.

Responsibilities:
- Design scalable backend services.
- Build event-driven architectures.
- Optimize database performance.
- Collaborate with cross-functional teams.
```

### Example output (abbreviated)

``` json
{
  "title": "Senior Backend Engineer",
  "family": "technology",
  "seniority": "senior",
  "minimum_years_experience": 5,
  "requirements": [
    {
      "original_name": "Golang",
      "normalized_name": "golang",
      "category": "technical_skills",
      "importance": "required"
    },
    {
      "original_name": "AWS",
      "normalized_name": "aws",
      "category": "technical_skills",
      "importance": "preferred"
    }
  ],
  "responsibilities": [
    "Design scalable backend services",
    "Build event-driven architectures",
    "Optimize database performance",
    "Collaborate with cross-functional teams"
  ]
}
```

------------------------------------------------------------------------

# 6. Scoring configuration

The scoring configuration is application-owned data. It is not generated
by the LLM for every match.

The job family extracted in Step 1B determines which configuration to
load.

### Example: technology

``` json
{
  "family": "technology",
  "version": 1,
  "categories": [
    {
      "name": "technical_skills",
      "weight": 0.35,
      "description": "Required and preferred technical skills and professional capabilities."
    },
    {
      "name": "responsibilities",
      "weight": 0.25,
      "description": "Alignment between previous work and the responsibilities of the job."
    },
    {
      "name": "experience",
      "weight": 0.20,
      "description": "Relevant professional experience, not just total years."
    },
    {
      "name": "seniority",
      "weight": 0.15,
      "description": "Evidence of ownership, autonomy, decision-making, and leadership where relevant."
    },
    {
      "name": "domain",
      "weight": 0.05,
      "description": "Relevance of industry and domain experience."
    }
  ]
}
```

### Example: customer operations

``` json
{
  "family": "customer_operations",
  "version": 1,
  "categories": [
    {
      "name": "skills",
      "weight": 0.20,
      "description": "Communication, listening, problem-solving, and customer service skills."
    },
    {
      "name": "responsibilities",
      "weight": 0.25,
      "description": "Alignment with customer support, complaint handling, and service responsibilities."
    },
    {
      "name": "experience",
      "weight": 0.15,
      "description": "Relevant professional and customer-facing experience."
    },
    {
      "name": "seniority",
      "weight": 0.10,
      "description": "Independence, accountability, and complexity of previous work."
    },
    {
      "name": "job_specific",
      "weight": 0.25,
      "description": "Job-specific requirements such as CRM tools, communication channels, language, and complaint handling."
    },
    {
      "name": "domain",
      "weight": 0.05,
      "description": "Relevant industry experience."
    }
  ]
}
```

### Other illustrative family configurations

These are hypotheses for initial testing, not validated industry
standards. Review them against real job descriptions and candidate
judgments.

  ----------------------------------------------------------------------------------------
  Category                     Technology   Administration       Customer   Transportation
                                                               operations 
  ------------------------ -------------- ---------------- -------------- ----------------
  Technical/professional              35%              25%            20%              20%
  skills                                                                  

  Responsibilities                    25%              25%            25%              20%

  Experience                          20%              15%            15%              15%

  Seniority                           15%              10%            10%               5%

  Domain                               5%               5%             5%               5%

  Job-specific                        ---              20%            25%              35%

  **Total**                      **100%**         **100%**       **100%**         **100%**
  ----------------------------------------------------------------------------------------

For a family that uses a `job_specific` category, include it in the
configuration and evaluation schema. Do not use an empty category merely
to force every family into the same dimensions.

### Configuration ownership

Store configurations in PostgreSQL or versioned application
configuration.

Suggested fields: - `family` - `version` - `categories` (JSONB or
normalized child rows) - `is_active` - `created_at` - `updated_at`

A match should retain the exact scoring configuration version used.
Changing weights later must not silently change the meaning of
previously generated reports.

------------------------------------------------------------------------

# 7. Step 2 --- Evaluate compatibility

**Purpose:** Compare the normalized candidate profile with the
normalized job profile.

### Input placeholders

  -----------------------------------------------------------------------
  Placeholder                         Exact source
  ----------------------------------- -----------------------------------
  `{{candidate_profile}}`             Step 1A output loaded from
                                      `candidate_profiles`

  `{{job_profile}}`                   Step 1B output loaded from
                                      `job_profiles`

  `{{evaluation_config}}`             Active scoring configuration for
                                      `job_profile.family`
  -----------------------------------------------------------------------

### Example of `evaluation_config`

The backend loads the technology configuration shown in Section 6 and
supplies it to the prompt.

It is not another LLM output.

### Prompt: `evaluate-match-v1`

``` text
# Role

You are an experienced recruitment evaluator.

Evaluate how well a candidate matches a specific job description using structured candidate and job information.

Provide evidence-based assessments, not generic judgments.

# Input

## Candidate Profile

{{candidate_profile}}

## Job Profile

{{job_profile}}

## Evaluation Configuration

{{evaluation_config}}

The evaluation configuration defines:
- Job family.
- Evaluation categories.
- Category weights.
- Category descriptions.

# Instructions

## 1. Evaluate configured categories

Evaluate every category provided in the evaluation configuration.

For each category:
- Assign a score from 0 to 100.
- Explain the assessment.
- Identify matching evidence.
- Identify gaps.
- Distinguish direct experience from transferable experience.
- Identify missing or uncertain information.

Do not create categories, omit categories, or change weights.

Weights are provided as context. Do not calculate the overall weighted score.

## 2. Skills and professional capabilities

Evaluate:
- Exact skill matches.
- Equivalent terminology.
- Related but different technologies.
- Practical experience.
- Depth of experience.
- Evidence from previous work.

Do not consider two different technologies equivalent just because they belong to the same category.

For non-technical positions, evaluate the relevant professional competencies.

## 3. Experience

Evaluate:
- Total professional experience.
- Relevant experience.
- Experience in similar positions.
- Complexity of previous responsibilities.
- Relevance of previous work.

Do not evaluate experience using years alone.

## 4. Responsibilities

Compare previous responsibilities and achievements with the job's responsibilities.

Evaluate:
- Similarity of tasks.
- Complexity.
- Scope of responsibility.
- Relevant achievements.
- Transferable experience.

## 5. Seniority

Evaluate evidence of:
- Independence.
- Ownership.
- Decision-making.
- Problem-solving.
- Leadership, if relevant.
- Responsibility and accountability.

Do not rely solely on previous job titles.

## 6. Domain

Evaluate the relevance of the candidate's industry or domain experience.

Do not automatically penalize a candidate for experience in another industry when their capabilities are transferable.

## 7. Job-specific requirements

Evaluate the additional criteria defined for the job family.

Examples:
- Driving license and vehicle experience.
- Customer communication.
- Microsoft Excel.
- Language proficiency.
- Professional certifications.

## 8. Mandatory requirements

Evaluate each job requirement marked `required` independently.

Use exactly one status:
- met: candidate evidence supports the requirement.
- not_met: available evidence explicitly establishes that the candidate does not satisfy it.
- uncertain: insufficient evidence to determine whether the candidate satisfies it.

Do not treat missing CV information as `not_met`.

Do not treat preferred or optional requirements as mandatory.

## 9. Scoring guidance

Use these general score interpretations:
- 90–100: Excellent alignment.
- 75–89: Strong alignment.
- 60–74: Moderate alignment.
- 40–59: Limited alignment.
- 0–39: Poor alignment.

These ranges are guidance, not automatic scoring rules.

Scores must reflect actual evidence and relevance.

Do not inflate scores to make the candidate appear more suitable.

## 10. Evidence and fairness

For each significant assessment, reference supporting candidate evidence and relevant job requirements.

Never:
- Invent candidate experience.
- Assume missing skills.
- Treat preferred requirements as mandatory.
- Penalize candidates for unrelated requirements.
- Use protected or irrelevant personal characteristics in the assessment.

# Output

Return valid JSON matching the application's evaluation schema.

For every configured category, return:
- category
- score
- reason
- strengths
- gaps
- evidence

For every mandatory requirement, return:
- requirement
- status
- reason
- evidence

Do not calculate the overall weighted score.
Do not generate the final candidate-facing report.
```

### Example input data

The backend sends the complete normalized candidate profile, job
profile, and configuration. The following is a shortened illustration:

``` json
{
  "candidate_profile": {
    "years_experience": 6,
    "skills": [
      "golang",
      "postgresql",
      "kafka",
      "rabbitmq",
      "microservices",
      "gcp"
    ],
    "experience": [
      {
        "company": "Amartha",
        "role": "Mid-Level Backend Engineer",
        "responsibilities": [
          "Golang backend development",
          "Transaction processing",
          "Message broker integration",
          "Wallet and agent services"
        ]
      }
    ]
  },
  "job_profile": {
    "title": "Senior Backend Engineer",
    "family": "technology",
    "seniority": "senior",
    "minimum_years_experience": 5,
    "requirements": [
      {
        "name": "Golang",
        "importance": "required"
      },
      {
        "name": "PostgreSQL",
        "importance": "required"
      },
      {
        "name": "Kafka",
        "importance": "required"
      },
      {
        "name": "AWS",
        "importance": "preferred"
      },
      {
        "name": "Kubernetes",
        "importance": "preferred"
      }
    ]
  },
  "evaluation_config": {
    "family": "technology",
    "version": 1,
    "categories": [
      {"name": "technical_skills", "weight": 0.35},
      {"name": "responsibilities", "weight": 0.25},
      {"name": "experience", "weight": 0.20},
      {"name": "seniority", "weight": 0.15},
      {"name": "domain", "weight": 0.05}
    ]
  }
}
```

### Example output (abbreviated)

``` json
{
  "evaluations": [
    {
      "category": "technical_skills",
      "score": 90,
      "reason": "Strong evidence of Golang, PostgreSQL, and Kafka experience.",
      "strengths": ["Golang", "PostgreSQL", "Kafka"],
      "gaps": ["AWS not established", "Kubernetes not established"],
      "evidence": [
        {
          "candidate_evidence": "Golang backend development",
          "job_requirement": "Strong Golang experience",
          "relationship": "direct"
        }
      ]
    },
    {
      "category": "responsibilities",
      "score": 80,
      "reason": "Transaction processing and message broker integration are relevant to event-driven backend systems.",
      "strengths": ["Transaction processing", "Message broker integration"],
      "gaps": ["Scale and architecture ownership need more evidence"],
      "evidence": []
    },
    {
      "category": "experience",
      "score": 100,
      "reason": "Approximately six years of professional experience against five required.",
      "strengths": ["Exceeds stated minimum"],
      "gaps": [],
      "evidence": []
    },
    {
      "category": "seniority",
      "score": 65,
      "reason": "Current title is mid-level; evidence of senior-level ownership is limited.",
      "strengths": [],
      "gaps": ["Architecture ownership not established"],
      "evidence": []
    },
    {
      "category": "domain",
      "score": 90,
      "reason": "Financial transaction processing is relevant to fintech.",
      "strengths": ["Financial services experience"],
      "gaps": [],
      "evidence": []
    }
  ],
  "mandatory_requirements": [
    {
      "requirement": "5+ years experience",
      "status": "met",
      "reason": "Candidate profile indicates approximately six years.",
      "evidence": ["Employment history"]
    },
    {
      "requirement": "Golang",
      "status": "met",
      "reason": "Golang backend development is listed in the candidate's experience.",
      "evidence": ["Amartha experience"]
    },
    {
      "requirement": "PostgreSQL",
      "status": "met",
      "reason": "PostgreSQL is listed in the candidate's skills.",
      "evidence": ["CV skills section"]
    },
    {
      "requirement": "Kafka",
      "status": "met",
      "reason": "Kafka/message broker experience is documented.",
      "evidence": ["Amartha experience"]
    }
  ]
}
```

The numbers above are illustrative. They are not validated benchmark
results. Use a labeled test set to calibrate scoring consistency.

------------------------------------------------------------------------

# 8. Step 3 --- Go scoring and eligibility

**Purpose:** Calculate a reproducible weighted score and aggregate
mandatory requirement statuses.

No LLM prompt is needed for this step.

### Inputs

  Input                              Source
  ---------------------------------- -----------------------------------
  Evaluation categories and scores   Step 2 output
  Category weights                   Same configuration used in Step 2
  Mandatory requirement statuses     Step 2 output

### Weighted score formula

For each configured category:

`weighted_category_score = category_score * category_weight`

Then:

`overall_score = sum(weighted_category_score)`

Example:

  Category             Score     Weight   Contribution
  ------------------ ------- ---------- --------------
  Technical skills        90       0.35          31.50
  Responsibilities        80       0.25          20.00
  Experience             100       0.20          20.00
  Seniority               65       0.15           9.75
  Domain                  90       0.05           4.50
  **Total**                    **1.00**      **85.75**

### Example Go implementation

``` go
package scoring

import (
    "fmt"
)

type CategoryConfig struct {
    Name   string  `json:"name"`
    Weight float64 `json:"weight"`
}

type CategoryEvaluation struct {
    Category string  `json:"category"`
    Score    float64 `json:"score"`
}

type MandatoryRequirement struct {
    Requirement string `json:"requirement"`
    Status      string `json:"status"`
}

type EligibilityStatus string

const (
    Eligible         EligibilityStatus = "eligible"
    NotEligible      EligibilityStatus = "not_eligible"
    NeedsVerification EligibilityStatus = "needs_verification"
)

type Result struct {
    OverallScore     float64           `json:"overall_score"`
    EligibilityStatus EligibilityStatus `json:"eligibility_status"`
}

func Calculate(
    config []CategoryConfig,
    evaluations []CategoryEvaluation,
    requirements []MandatoryRequirement,
) (Result, error) {
    scores := make(map[string]float64)

    for _, item := range evaluations {
        if item.Score < 0 || item.Score > 100 {
            return Result{}, fmt.Errorf(
                "invalid score for %s", item.Category,
            )
        }

        if _, exists := scores[item.Category]; exists {
            return Result{}, fmt.Errorf(
                "duplicate category: %s", item.Category,
            )
        }

        scores[item.Category] = item.Score
    }

    var total, totalWeight float64
    configured := make(map[string]bool)

    for _, category := range config {
        if category.Weight < 0 {
            return Result{}, fmt.Errorf(
                "invalid weight for %s", category.Name,
            )
        }

        if configured[category.Name] {
            return Result{}, fmt.Errorf(
                "duplicate configured category: %s",
                category.Name,
            )
        }
        configured[category.Name] = true

        score, exists := scores[category.Name]
        if !exists {
            return Result{}, fmt.Errorf(
                "missing evaluation: %s", category.Name,
            )
        }

        total += score * category.Weight
        totalWeight += category.Weight
    }

    if len(scores) != len(config) {
        return Result{}, fmt.Errorf(
            "unexpected evaluation categories",
        )
    }

    if totalWeight < 0.999 || totalWeight > 1.001 {
        return Result{}, fmt.Errorf(
            "weights must sum to 1, got %f", totalWeight,
        )
    }

    eligibility := Eligible

    for _, requirement := range requirements {
        switch requirement.Status {
        case "met":
            // No change.
        case "not_met":
            eligibility = NotEligible
        case "uncertain":
            if eligibility != NotEligible {
                eligibility = NeedsVerification
            }
        default:
            return Result{}, fmt.Errorf(
                "invalid requirement status: %s",
                requirement.Status,
            )
        }
    }

    return Result{
        OverallScore:      total / totalWeight,
        EligibilityStatus: eligibility,
    }, nil
}
```

### Important implementation notes

-   Reject missing and duplicate category evaluations.
-   Reject unexpected categories.
-   Validate that scores are between 0 and 100.
-   Validate that weights sum to 1.
-   Validate mandatory requirement statuses.
-   Do not silently skip invalid data.
-   Keep eligibility independent of the compatibility score.

The example uses floating-point arithmetic for readability. For
persisted scores, choose and document a rounding policy (for example,
round to two decimal places).

### Eligibility aggregation

  Mandatory checks                         Eligibility result
  ---------------------------------------- ----------------------
  All are `met`                            `eligible`
  At least one is `not_met`                `not_eligible`
  No `not_met`, at least one `uncertain`   `needs_verification`

If there are no mandatory requirements, define the behavior explicitly.
A reasonable default is `eligible` with an empty requirement list.

A missing CV detail should normally result in `uncertain`, not
`not_met`. For regulated or safety-critical qualifications, the product
may require verification before presenting the candidate as eligible.

### Example output

``` json
{
  "overall_score": 85.75,
  "eligibility_status": "eligible",
  "category_scores": {
    "technical_skills": 90,
    "responsibilities": 80,
    "experience": 100,
    "seniority": 65,
    "domain": 90
  }
}
```

------------------------------------------------------------------------

# 9. Step 4 --- Generate final report

**Purpose:** Turn the structured evaluation into a clear, useful report
for the candidate.

The report model must not recalculate scores or change evaluation
conclusions.

### Input placeholders

  Placeholder                 Exact source
  --------------------------- ----------------------------------
  `{{candidate_profile}}`     Step 1A stored candidate profile
  `{{job_profile}}`           Step 1B stored job profile
  `{{evaluation_results}}`    Step 2 output
  `{{scoring_results}}`       Step 3 Go output
  `{{eligibility_results}}`   Step 3 eligibility output

`eligibility_results` may be passed as a separate object for clarity,
even if it is also included in `scoring_results`.

### Prompt: `generate-report-v1`

``` text
# Role

You are a professional career advisor helping candidates understand their compatibility with job opportunities.

Generate a clear, honest, and actionable job compatibility report.

# Input

## Candidate Profile

{{candidate_profile}}

## Job Information

{{job_profile}}

## Evaluation Results

{{evaluation_results}}

## Scoring Results

{{scoring_results}}

## Eligibility Results

{{eligibility_results}}

# Instructions

## 1. Overall assessment

Present:
- Job title.
- Overall compatibility score.
- Recommendation.
- Eligibility status.

Use these recommendation labels:

- strong_match: Strong overall alignment.
- worth_applying: Good alignment with some gaps.
- possible_match: Some relevant qualifications, but significant gaps.
- low_match: Limited alignment.

Treat these labels as product presentation guidance, not objective hiring outcomes.

Do not automatically classify a candidate as ineligible solely because information is missing.

## 2. Strengths

Identify the candidate's strongest matching qualifications.

Explain why each qualification is relevant to this specific job.

## 3. Gaps

Identify:
- Confirmed missing required qualifications.
- Preferred skills not established in the CV.
- Skills supported only by transferable experience.
- Requirements needing additional verification.

Distinguish confirmed gaps from missing CV evidence.

## 4. Experience assessment

Explain how previous employment, responsibilities, and achievements relate to the target position.

Identify areas where the candidate should provide stronger evidence.

## 5. Recommendations

Provide practical suggestions:
- CV improvements.
- Relevant experience to highlight.
- Skills worth learning.
- Potential interview preparation topics.

Recommendations must be realistic and relevant to the job.

## 6. Accuracy

Never:
- Invent qualifications.
- Exaggerate experience.
- Change evaluation scores.
- Present uncertain information as fact.
- Hide unmet mandatory requirements.
- Recommend that a candidate ignore legal, licensing, or work-authorization requirements.

# Output

Return valid JSON matching the application's report schema.

Include:
- job_title
- overall_score
- recommendation
- eligibility_status
- summary
- strengths
- gaps
- experience_assessment
- cv_improvements
- learning_recommendations
- interview_preparation

Keep explanations concise, specific, and useful to the candidate.
```

### Example output

``` json
{
  "job_title": "Senior Backend Engineer",
  "overall_score": 85.75,
  "recommendation": "worth_applying",
  "eligibility_status": "eligible",
  "summary": "Strong backend engineering alignment with relevant financial transaction experience.",
  "strengths": [
    "Golang, PostgreSQL, and Kafka experience",
    "Approximately six years of professional experience",
    "Relevant transaction processing work"
  ],
  "gaps": [
    {
      "requirement": "AWS",
      "status": "not_found_in_cv",
      "suggestion": "Highlight relevant GCP experience without claiming AWS experience."
    },
    {
      "requirement": "Kubernetes",
      "status": "not_found_in_cv",
      "suggestion": "Add relevant Kubernetes experience if you have it."
    }
  ],
  "experience_assessment": "Transaction processing and message broker experience are relevant. More evidence of architecture ownership would strengthen the application.",
  "cv_improvements": [
    "Explain the transaction processing work and your contribution.",
    "Highlight Kafka integration and its business impact.",
    "Describe architectural decisions and technical ownership."
  ],
  "learning_recommendations": [
    "Explore AWS fundamentals if AWS is important to your target roles."
  ],
  "interview_preparation": [
    "Prepare to explain your message broker architecture.",
    "Prepare examples of database optimization."
  ]
}
```

------------------------------------------------------------------------

# 10. Example: how other job families use the same pipeline

The prompts remain generic. Only the extracted job profile and scoring
configuration change.

## A. Administrative Staff

Example JD:

``` text
Position: Administrative Staff

Requirements:
- Good Microsoft Excel skills.
- Accurate data entry.
- Good written communication.
- Attention to detail.
- Previous administrative experience preferred.

Responsibilities:
- Maintain company records.
- Prepare reports.
- Manage documents.
- Support daily office operations.
```

Step 1B should classify this as `administration`.

The backend loads the administration scoring configuration. Step 2
evaluates administrative skills, responsibilities, experience,
seniority, domain, and job-specific criteria.

## B. Customer Service Representative

Example JD:

``` text
Position: Customer Service Representative

Requirements:
- Strong communication skills.
- Ability to handle customer complaints.
- Good problem-solving skills.
- Experience using CRM systems preferred.
- Good written communication.

Responsibilities:
- Respond to customer inquiries.
- Resolve customer complaints.
- Communicate through phone and email.
- Record customer interactions.
```

Step 1B should classify this as `customer_operations`.

The evaluation should consider communication, customer handling,
problem-solving, CRM experience, and relevant responsibilities.

## C. Delivery Driver

Example JD:

``` text
Position: Delivery Driver

Requirements:
- Valid driving license for the required vehicle class.
- At least 2 years of driving experience.
- Familiarity with navigation applications.
- Good knowledge of local routes preferred.

Responsibilities:
- Deliver packages safely.
- Follow assigned routes.
- Maintain vehicle condition.
- Record delivery status.
```

Step 1B should classify this as `transportation`.

The evaluation should consider: - License and vehicle-class
requirements. - Driving experience. - Navigation skills. - Delivery
responsibilities. - Relevant experience.

A required license must be checked separately from the weighted
compatibility score. Missing evidence of a license should be
`uncertain`, not automatically `not_met`.

------------------------------------------------------------------------

# 11. Suggested database structure

This is a starting point. Adapt it to the existing Getojobu schema
rather than copying it blindly.

  ------------------------------------------------------------------------
  Table                    Purpose                 Important data
  ------------------------ ----------------------- -----------------------
  `candidate_profiles`     Normalized CV           Candidate ID, CV
                                                   version, profile JSON,
                                                   prompt version

  `job_profiles`           Normalized JD           Job ID, family, profile
                                                   JSON, prompt version

  `scoring_configs`        Family weights          Family, version,
                                                   categories, active
                                                   status

  `matching_evaluations`   Step 2 results          Candidate profile
                                                   version, job profile
                                                   version, config
                                                   version, evaluation
                                                   JSON

  `matching_results`       Step 3 results          Overall score,
                                                   eligibility, category
                                                   scores

  `match_reports`          Step 4 report           Report JSON, prompt
                                                   version
  ------------------------------------------------------------------------

### Recommended version tracking

For each match, preserve: - Candidate profile version. - Job profile
version. - Extraction prompt versions. - Evaluation prompt version. -
Report prompt version. - Model/provider identifier. - Scoring
configuration version. - Creation timestamp.

This makes it possible to explain and reproduce how a report was
produced.

------------------------------------------------------------------------

# 12. Suggested Go project organization

``` text
apps/api/
└── internal/
    ├── modules/
    │   ├── profile/
    │   │   ├── extraction.go
    │   │   └── service.go
    │   │
    │   ├── job/
    │   │   ├── extraction.go
    │   │   └── service.go
    │   │
    │   └── matching/
    │       ├── evaluation.go
    │       ├── scoring.go
    │       ├── eligibility.go
    │       ├── report.go
    │       └── service.go
    │
    └── ai/
        ├── client.go
        └── prompts/
            ├── extract-cv-v1.txt
            ├── extract-job-v1.txt
            ├── evaluate-match-v1.txt
            └── generate-report-v1.txt
```

### Suggested operations

-   `AnalyzeCV`: Extract and store a candidate profile.
-   `AnalyzeJob`: Extract and store a job profile.
-   `EvaluateMatch`: Evaluate a candidate against a job.
-   `CalculateScore`: Calculate score and eligibility in Go.
-   `GenerateReport`: Generate the candidate-facing report.

The API endpoints can be designed around your existing routing and
resource conventions.

------------------------------------------------------------------------

# 13. Implementation checklist

## Extraction

-   [ ] Extract text from supported CV formats.
-   [ ] Implement CV extraction prompt.
-   [ ] Implement JD extraction prompt.
-   [ ] Define strict JSON schemas for both outputs.
-   [ ] Validate model output before persistence.
-   [ ] Preserve source evidence.
-   [ ] Store prompt and model versions.
-   [ ] Reuse CV profiles across job matches.

## Configuration

-   [ ] Define initial job families.
-   [ ] Create scoring configurations.
-   [ ] Validate that weights sum to 1.
-   [ ] Version scoring configurations.
-   [ ] Define behavior for unknown job families.

## Evaluation

-   [ ] Implement generic evaluation prompt.
-   [ ] Pass candidate profile, job profile, and scoring configuration.
-   [ ] Require every configured category in the response.
-   [ ] Validate scores and evidence.
-   [ ] Evaluate mandatory requirements independently.
-   [ ] Store the complete evaluation result.

## Scoring

-   [ ] Implement weighted score calculation.
-   [ ] Validate duplicate, missing, and unexpected categories.
-   [ ] Implement eligibility aggregation.
-   [ ] Define rounding policy.
-   [ ] Keep score and eligibility separate.

## Reporting

-   [ ] Implement report generation prompt.
-   [ ] Prevent the report model from changing scores.
-   [ ] Validate recommendation labels.
-   [ ] Store the report and prompt version.

## Quality and evaluation

-   [ ] Create a small labeled test dataset.
-   [ ] Test technology, administration, customer operations, and
    transportation.
-   [ ] Review incorrect or unsupported model judgments.
-   [ ] Test missing CV information.
-   [ ] Test required versus preferred requirements.
-   [ ] Compare model outputs across prompt versions.
-   [ ] Establish a baseline before experimenting with Jev 1.13.

------------------------------------------------------------------------

# 14. Important caveats before production

1.  **Scores are estimates, not objective probabilities.** A score of 85
    does not mean an 85% chance of getting hired.
2.  **Weights are hypotheses.** The initial family weights need
    validation against real job descriptions and human judgments.
3.  **LLM scores can vary.** Use structured output, low-variance
    settings where available, versioned prompts, and a repeatable
    evaluation dataset.
4.  **Evidence matters.** Store the evidence supporting each category
    score. A score without traceable evidence is difficult to debug.
5.  **CVs and JDs are untrusted input.** Treat their content as data,
    not instructions. The prompt should explicitly tell the model to
    ignore instructions embedded in supplied documents.
6.  **Privacy matters.** Minimize stored personal information, define
    retention rules, and avoid using protected characteristics in
    matching.
7.  **Eligibility is separate.** Work authorization, professional
    licenses, and legally required qualifications should not be
    compensated for by a high compatibility score.
8.  **Do not introduce Jev prematurely.** First establish a baseline
    with a reasoning LLM. Then compare Jev on the same labeled examples
    to see whether it improves accuracy, cost, or latency.

------------------------------------------------------------------------

## Final recommendation

For Getojobu v1:

-   Use a reasoning LLM for extraction, classification, and
    compatibility evaluation.
-   Use PostgreSQL to store normalized profiles and family-specific
    scoring configurations.
-   Use Go for weighted calculations, validation, and eligibility
    aggregation.
-   Use a reasoning LLM to generate the final report.
-   Keep Jev 1.13 as a future experiment, not a dependency.

The central design principle is:

**LLM understands the documents. Go calculates the score. The final LLM
explains the result.**

Build the extraction and evaluation pipeline first, test it against real
job descriptions, and only then optimize the scoring model.
