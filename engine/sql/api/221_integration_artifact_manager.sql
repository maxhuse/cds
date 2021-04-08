-- +migrate Up
ALTER TABLE "integration_model" ADD COLUMN artifact_manager boolean DEFAULT false;
ALTER TABLE workflow_node_run_job ADD COLUMN integration_plugins JSONB;

-- +migrate Down
ALTER TABLE "integration_model" DROP COLUMN artifact_manager;
ALTER TABLE workflow_node_run_job DROP COLUMN integration_plugins;
