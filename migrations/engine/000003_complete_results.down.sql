-- Restoring the pre-upgrade backup is required: node events cannot be represented
-- by the old outbox uniqueness constraint. Deliberately refuse a lossy downgrade.
SELECT * FROM complete_results_downgrade_requires_backup_restore;
