ALTER TABLE certificates
    DROP COLUMN IF EXISTS point_of_sale_id,
    DROP COLUMN IF EXISTS encrypted_blob,
    DROP COLUMN IF EXISTS encrypted_password,
    DROP COLUMN IF EXISTS serial_number;