DELETE FROM integration_providers WHERE slug = 'twilio';

ALTER TABLE transactions DROP COLUMN IF EXISTS call_id;

DROP TABLE IF EXISTS call_preloads;
DROP TABLE IF EXISTS rtb_pings;
DROP TABLE IF EXISTS call_suppression;
DROP TABLE IF EXISTS call_legs;
DROP TABLE IF EXISTS calls;
DROP TABLE IF EXISTS participation_call_targets;
DROP TABLE IF EXISTS contract_call_settings;

ALTER TABLE contract_compensations DROP CONSTRAINT contract_compensations_trigger_check;
ALTER TABLE contract_compensations ADD CONSTRAINT contract_compensations_trigger_check
  CHECK (trigger IN ('per_lead', 'buyer_stage', 'manual'));

DROP INDEX IF EXISTS idx_routing_sources_tracking_number;
ALTER TABLE routing_sources
  DROP COLUMN IF EXISTS require_preload,
  DROP COLUMN IF EXISTS payload_enabled,
  DROP COLUMN IF EXISTS twilio_sid,
  DROP COLUMN IF EXISTS tracking_number,
  DROP COLUMN IF EXISTS integration_connection_id;
