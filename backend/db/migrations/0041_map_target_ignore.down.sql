DELETE FROM routing_source_field_map WHERE target_type = 'ignore';

ALTER TABLE routing_source_field_map DROP CONSTRAINT routing_source_field_map_check;
ALTER TABLE routing_source_field_map ADD CONSTRAINT routing_source_field_map_check CHECK (
    (target_type = 'builtin' AND builtin_field IS NOT NULL AND custom_field_id IS NULL)
    OR (target_type = 'custom' AND custom_field_id IS NOT NULL AND builtin_field IS NULL)
);
