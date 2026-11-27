BEGIN;
TRUNCATE TABLE affiliations, prizes, laureates RESTART IDENTITY;
UPDATE dataset_metadata SET last_updated = NULL WHERE id = 1;
COMMIT;
