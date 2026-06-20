CREATE TABLE custom_field_folders (
    id          BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    public_id   UUID NOT NULL DEFAULT gen_random_uuid() UNIQUE,
    account_id  BIGINT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    name        TEXT NOT NULL,
    position    INT NOT NULL DEFAULT 0,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_cff_account ON custom_field_folders(account_id);

ALTER TABLE custom_fields
    ADD COLUMN folder_id BIGINT REFERENCES custom_field_folders(id) ON DELETE SET NULL;
CREATE INDEX idx_cf_folder ON custom_fields(folder_id);
