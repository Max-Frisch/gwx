-- ============================================================================
-- gwx — unit production queue: an unlimited chain of ship/defense batches.
--
-- The old `uq_queue_shipyard` partial unique index allowed only one active
-- ship/defense order per planet. GalaxyWars lets the player append as many
-- batches as they like: the queue is "unlimited" and each new order is
-- scheduled to start when the previous one finishes. Drop the index; the
-- back-to-back scheduling is enforced in Go (QueueStore.StartUnits) while the
-- planet row is locked.
-- ============================================================================

DROP INDEX IF EXISTS uq_queue_shipyard;
