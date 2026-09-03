DROP TRIGGER IF EXISTS reviews_after_insert_aggregate_sync ON reviews;
DROP FUNCTION IF EXISTS review_aggregates_sync();
DROP TABLE IF EXISTS review_aggregates;
DROP TABLE IF EXISTS reviews;
