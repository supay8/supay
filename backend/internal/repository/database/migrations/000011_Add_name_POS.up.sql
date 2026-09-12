BEGIN;

ALTER TABLE points_of_sale
ADD COLUMN name varchar(100);

UPDATE points_of_sale
SET name = CONCAT('Punto de Venta ', codigo_punto_venta)
WHERE name IS NULL;

ALTER TABLE points_of_sale
ALTER COLUMN name SET NOT NULL;

COMMIT;