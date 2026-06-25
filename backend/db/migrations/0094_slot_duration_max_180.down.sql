ALTER TABLE buyer_appointment_slots DROP CONSTRAINT buyer_appointment_slots_duration_check;
ALTER TABLE buyer_appointment_slots ADD CONSTRAINT buyer_appointment_slots_duration_check
    CHECK (duration_min >= 15 AND duration_min <= 240);

ALTER TABLE contract_appointment_slots DROP CONSTRAINT contract_appointment_slots_duration_check;
ALTER TABLE contract_appointment_slots ADD CONSTRAINT contract_appointment_slots_duration_check
    CHECK (duration_min_override IS NULL OR (duration_min_override >= 15 AND duration_min_override <= 240));
