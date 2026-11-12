# JobSpy Reference Response and Schema

This document details the schema, fields, and behavior of `python-jobspy` as utilized by the Python Temporal worker (`JobScrapeWorkflow` in `workers/python/workflows/job_scrape.py` and `deliveries/consumer/activities/job_scrape.py`).

A static JSON fixture is also maintained at [jobspy_reference.json](file:///Users/nyomanfrastyawan/Documents/dev/frasnym/homelab-01/apps/getojobu/workers/python/tests/fixtures/jobspy_reference.json).

---

## 1. Core Behavior: `fetch_description`

`jobspy.scrape_jobs()` has a critical parameter:
```python
def scrape_jobs(
    ...
    fetch_description: bool | None = False,
    description_format: str | DescriptionFormat | None = "markdown",
) -> pd.DataFrame:
```

- **Default (`fetch_description=False`):**
  JobSpy only queries the job board search/listing index (titles, companies, locations, dates, links). It **skips** making individual detail requests.
  - On **Glassdoor** and **LinkedIn**, `description` is returned as `NaN` / `None`.
  - On **Indeed**, short card snippets might appear, but full descriptions are not fetched.
- **Enabled (`fetch_description=True`):**
  JobSpy follows up each search card with a GET request to the job posting page or details endpoint to retrieve the full body text.
  - `description` contains the plain text or markdown of the job requirements and responsibilities.

> [!IMPORTANT]
> Because `JobAnalyzeWorkflow` and the Go backend reject jobs with empty descriptions (`has no description to analyze`), `fetch_description=True` must be explicitly passed to `jobspy.scrape_jobs()`.

---

## 2. JobSpy Output Schema

JobSpy returns a `pandas.DataFrame`. When converted via `to_dict("records")`, each record contains the following keys:

| Field | Type | Description |
|---|---|---|
| `id` | `str` | Prefixed job ID (e.g. `gd-1010053526738` or `li-4112345678`) |
| `site` | `str` | Board identifier (`glassdoor`, `linkedin`, `indeed`, `zip_recruiter`) |
| `job_url` | `str` | URL to the listing on the job board |
| `job_url_direct` | `str \| null` | Direct applicant/company URL if provided |
| `title` | `str` | Job title (e.g. `Systems Automation Engineer I`) |
| `company` | `str` | Employer name (e.g. `RoviSys`) |
| `location` | `str` | Location string (e.g. `Ireland` or `Dublin, County Dublin, Ireland`) |
| `date_posted` | `str \| null` | ISO date `YYYY-MM-DD` |
| `job_type` | `str \| null` | `fulltime`, `contract`, `parttime`, `internship` |
| `is_remote` | `bool` | Remote status flag |
| `description` | `str \| null` | Full job description text (requires `fetch_description=True`) |
| `salary_source` | `str \| null` | `direct_data` or `description` |
| `interval` | `str \| null` | `yearly`, `monthly`, `hourly` |
| `min_amount` | `float \| null` | Lower bound of compensation |
| `max_amount` | `float \| null` | Upper bound of compensation |
| `currency` | `str \| null` | Currency code (`USD`, `EUR`, etc.) |
| `job_level` | `str \| null` | Experience tier (e.g. `entry_level`, `mid_senior`) |
| `job_function` | `str \| null` | Department / function |
| `company_url` | `str \| null` | Board profile URL for company |
| `company_url_direct` | `str \| null` | Company homepage |
| `company_addresses` | `str \| null` | Headquarters address |
| `company_num_employees` | `str \| null` | Company size range |
| `skills` | `list[str] \| null` | Extracted skill tags (where available) |

---

## 3. Reference JSON Sample

Example captured from Glassdoor for `Systems Automation Engineer I @ RoviSys` in Ireland:

```json
{
  "id": "gd-1010053526738",
  "site": "glassdoor",
  "job_url": "https://www.glassdoor.com/job-listing/j?jl=1010053526738",
  "job_url_direct": null,
  "title": "Systems Automation Engineer I",
  "company": "RoviSys",
  "location": "Ireland",
  "date_posted": "2026-11-12",
  "job_type": "fulltime",
  "salary_source": null,
  "interval": null,
  "min_amount": null,
  "max_amount": null,
  "currency": null,
  "is_remote": false,
  "job_level": null,
  "job_function": null,
  "listing_type": "organic",
  "emails": null,
  "description": "Overview: About RoviSys: RoviSys is an independent system integrator with year-over-year growth in Ireland, Europe, and world-wide. RoviSys provides a variety of information and automation services in many industries globally. RoviSys Ireland formed in 2022, with the Dublin Office open in May 2024. Dublin is part of RoviSys Europe group and currently operates closely with our European HQ located in Utrecht in The Nederland’s. RoviSys operates in 15+ markets areas and this leads to varied work experience with huge potential to develop and grow. About the opportunity: Systems Automation Engineer I is an entry level position responsible for designing and implementation of automation and process control systems for Life Science, Data Centre, Medical Devices, Food and Beverage and many other industrial sectors that RoviSys Europe operate in. Responsibilities: Key Responsibilities include: Assist in design and development of automation software tailored to customer project requirements. Work across multiple control systems and SCADA packages. Create and maintain design documentation. Assist in customer demonstrations and acceptance testing. Conduct electrical panel testing and Factory Acceptance Testing (FAT). Support site commissioning activities, including Site Acceptance Testing (SAT), Installation Qualification (IQ), and Operational Qualification (OQ). Assist with developing training documentation/presentation on operations and maintenance of system. Team & Work Environment: Typically, part of a project team of 2–5 engineers, led by an experienced colleague. Work will be split between the assigned RoviSys office and customer sites. Travel may be required depending on project needs. Qualifications: Experience Required: Graduate – 2 years’ experience Bachelor of Science (Level 8) in Engineering or Engineering Technology, Computer Science and/or equivalent qualification",
  "company_industry": "Machinery Manufacturing",
  "company_url": "https://www.glassdoor.com/Overview/W-EI_IE150292.htm",
  "company_logo": "https://media.glassdoor.com/sql/150292/rovisys-squarelogo.png",
  "company_url_direct": "https://www.rovisys.com",
  "company_addresses": "Aurora, OH",
  "company_num_employees": "1001 to 5000 Employees",
  "company_revenue": "Unknown / Non-Applicable",
  "company_description": "Since our founding, we’ve stood by simple principles. These have focused our vision and guided our success. Today, clients around the globe know RoviSys as a trustworthy partner that makes businesses stronger, more sustainable, and more profitable.",
  "skills": null,
  "experience_range": null,
  "company_rating": 3.8,
  "company_reviews_count": null,
  "vacancy_count": null,
  "work_from_home_type": null
}
```

---

## 4. Pipeline Mapping & Sanitization

1. **`_sanitize_for_json(val)`:**
   `pandas` produces `float('nan')` for missing numerical/string fields and `pd.NaT` for missing timestamps. Temporal's JSON payload converter cannot serialize `float('nan')` without converting it. `_sanitize_for_json` recursively converts `NaN` and `NaT` to `None`.
2. **`enrich_jobs` Mapping:**
   - `slug`: `build_job_slug(title, company, location)` -> e.g. `systems-automation-engineer-i-rovisys-ireland`.
   - `source`: `site` (e.g. `glassdoor`).
   - `source_url`: `job_url_direct` or fallback to `job_url`.
   - `description`: `row.get("description")`.
   - `posted_date`: Filtered by recency (`cut = now - 24h`) and formatted as `YYYY-MM-DD`.
3. **`ingest_jobs`:**
   Sends the enriched payload to Go backend `POST /v1/jobs/bulk`.
4. **`queue_analysis`:**
   Sends newly created slugs with non-empty descriptions to `POST /v1/jobs/bulk-queue-analyze`. Jobs without descriptions must not be queued because the Go backend rejects them with `invalid status transition: job "..." has no description to analyze`.
