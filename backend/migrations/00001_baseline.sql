-- +goose Up
-- Verify transactional migration bookkeeping without creating domain objects.
SELECT 1;

-- +goose Down
-- Baseline has no application objects to remove.
SELECT 1;
