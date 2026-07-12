DROP TABLE IF EXISTS thread_channel_routing;
DROP TABLE IF EXISTS channel_connections;
DROP TABLE IF EXISTS broadcast_jobs;
DROP TABLE IF EXISTS connect_requests;
DROP TABLE IF EXISTS message_attachments;
DROP TABLE IF EXISTS thread_members;
DROP TABLE IF EXISTS messages;
DROP TABLE IF EXISTS threads;
DROP FUNCTION IF EXISTS set_thread_purge();
DROP FUNCTION IF EXISTS update_thread_last_message();
DROP TYPE IF EXISTS ext_delivery_status;
DROP TYPE IF EXISTS thread_status;
DROP TYPE IF EXISTS thread_context;
DROP TYPE IF EXISTS thread_type;
-- notification_type enum values cannot be removed in PostgreSQL
