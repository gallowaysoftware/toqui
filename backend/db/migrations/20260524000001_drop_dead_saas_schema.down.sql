-- No-op down migration.
--
-- This migration drops dead SaaS schema left behind by the OSS pivot.
-- The data is gone; recreating empty tables wouldn't restore anything
-- meaningful. If a user truly needs to roll back, they should restore
-- from a pre-migration backup rather than re-running CREATE TABLE.
SELECT 1;
