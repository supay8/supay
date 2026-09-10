-- No eliminar reservas sin recepción: se perdería la protección frente a
-- reenvíos de operaciones cuyo resultado fiscal todavía es desconocido.
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM sent_packages WHERE codigo_recepcion = '') THEN
        RAISE EXCEPTION 'no se puede revertir mientras existan reservas sin recepción SIAT';
    END IF;
END $$;

DROP TABLE sent_package_invoices;
DROP INDEX idx_sent_packages_recepcion;
ALTER TABLE sent_packages
    ADD CONSTRAINT sent_packages_codigo_recepcion_key UNIQUE (codigo_recepcion),
    DROP COLUMN modalidad,
    DROP COLUMN layout,
    DROP COLUMN cufd,
    DROP COLUMN cuis,
    DROP COLUMN cufd_id;
