-- +migrate Up
DROP TABLE IF EXISTS "workflow_node_run_job_logs";
DROP TABLE IF EXISTS "requirement_service_logs";

-- +migrate Down
CREATE TABLE IF NOT EXISTS "workflow_node_run_job_logs" (
    id BIGSERIAL PRIMARY KEY,
    workflow_node_run_job_id BIGINT,
    workflow_node_run_id BIGINT,
    start TIMESTAMP WITH TIME ZONE,
    last_modified TIMESTAMP WITH TIME ZONE,
    done TIMESTAMP WITH TIME ZONE,
    step_order BIGINT,
    "value" BYTEA
);

SELECT create_foreign_key_idx_cascade('FK_WORKFLOW_NODE_RUN_JOBS_WORKFLOW_NODE_RUN', 'workflow_node_run_job_logs', 'workflow_node_run', 'workflow_node_run_id', 'id');
select create_index('workflow_node_run_job_logs', 'IDX_WORKFLOW_LOG_STEP', 'workflow_node_run_job_id,step_order');

CREATE TABLE IF NOT EXISTS "requirement_service_logs" (
    id BIGSERIAL PRIMARY KEY,
    workflow_node_run_job_id BIGINT,
    workflow_node_run_id BIGINT,
    requirement_service_name TEXT,
    start TIMESTAMP WITH TIME ZONE,
    last_modified TIMESTAMP WITH TIME ZONE,
    "value" BYTEA
);

SELECT create_foreign_key_idx_cascade('FK_REQUIREMENT_SERVICE_LOGS_WORKFLOW_NODE_RUN', 'requirement_service_logs', 'workflow_node_run', 'workflow_node_run_id', 'id');

