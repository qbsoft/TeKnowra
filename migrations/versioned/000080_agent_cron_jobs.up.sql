-- Scheduled agent tasks ("cron jobs").
--
-- Users create these conversationally through the `cronjob` built-in tool:
-- "every morning at 9, check which customers need a payment reminder".
-- The scheduler fires them and an asynq worker runs them.
--
-- The scheduler runs jobs; it does not deliver their output anywhere. If a job
-- should notify someone, that is the job's own work, done through whatever
-- tool it calls. What the scheduler owes the user is a readable record of what
-- happened, which is last_output / last_status / last_error.
--
-- One column encodes a guardrail rather than business data:
--
--   pinned_model    Provider/model snapshot taken at creation. If the global
--                   default model changes afterwards the run fails closed and
--                   alerts, rather than silently switching an unattended job
--                   onto a different (possibly much pricier) model.
CREATE TABLE IF NOT EXISTS agent_cron_jobs (
    id                 VARCHAR(36)  PRIMARY KEY,
    tenant_id          BIGINT       NOT NULL,
    creator_user_id    VARCHAR(64)  NOT NULL,
    agent_id           VARCHAR(36)  NOT NULL,

    name               VARCHAR(200) NOT NULL DEFAULT '',
    -- once | interval | cron
    schedule_kind      VARCHAR(16)  NOT NULL,
    -- Normalised robfig 6-field expression; natural language is converted on write.
    schedule_expr      VARCHAR(128) NOT NULL,
    next_run_at        TIMESTAMPTZ,

    prompt             TEXT         NOT NULL,
    -- agent | no_agent
    mode               VARCHAR(16)  NOT NULL DEFAULT 'agent',
    enabled_toolsets   JSONB,

    pinned_model       JSONB,

    -- NULL = run forever. Decremented only on a SUCCESSFUL run, so a run of
    -- network blips cannot silently burn a user's "run it 5 times" budget.
    repeat_left        INTEGER,

    enabled            BOOLEAN      NOT NULL DEFAULT TRUE,
    paused             BOOLEAN      NOT NULL DEFAULT FALSE,

    last_status        VARCHAR(16)  NOT NULL DEFAULT '',
    last_error         TEXT         NOT NULL DEFAULT '',
    -- What the run produced. A job that fires at 3am needs somewhere its
    -- result can be read afterwards.
    last_output        TEXT         NOT NULL DEFAULT '',
    last_run_at        TIMESTAMPTZ,
    -- Consecutive failures. A review nudge is sent once when this crosses the
    -- threshold, instead of nagging on every failed run.
    failure_streak     INTEGER      NOT NULL DEFAULT 0,

    -- Set by the worker when it starts, cleared when it finishes. Prevents a
    -- slow job from overlapping itself. A sweeper force-clears stale claims.
    -- NOT NULL with an empty default on purpose: the claim is taken and
    -- released with `running_claim_by = ''` predicates, and NULL would never
    -- match them — the guard would silently never engage.
    running_claim_by   VARCHAR(128) NOT NULL DEFAULT '',
    running_claim_at   TIMESTAMPTZ,

    created_at         TIMESTAMPTZ  NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at         TIMESTAMPTZ  NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at         TIMESTAMPTZ
);

-- The scheduler's hot path: "which jobs are due?"
CREATE INDEX IF NOT EXISTS idx_agent_cron_jobs_due
    ON agent_cron_jobs (next_run_at)
    WHERE deleted_at IS NULL AND enabled AND NOT paused;

-- Quota checks ("how many jobs does this user already have?") and the
-- management list view.
CREATE INDEX IF NOT EXISTS idx_agent_cron_jobs_owner
    ON agent_cron_jobs (tenant_id, creator_user_id)
    WHERE deleted_at IS NULL;
