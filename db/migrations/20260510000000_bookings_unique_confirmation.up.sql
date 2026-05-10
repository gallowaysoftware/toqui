-- Close the dedup-race window in the booking ingestion path.
--
-- Background: the IngestBooking path runs SELECT-by-(user, trip, code) →
-- INSERT, which is non-atomic. Two concurrent webhook deliveries (Resend
-- retries, user forwarding the same email twice) both saw "no row" and
-- both succeeded INSERT, defeating the dedup feature the merge-PR
-- shipped. Application-side advisory locks were considered and rejected
-- because they don't survive multi-instance Cloud Run scaleout.
--
-- A partial unique index over (user_id, trip_id, confirmation_code) is
-- the correct database-side guarantee. The partial filter excludes:
--   * unattached bookings (trip_id IS NULL) — they have no merge-target
--     anchor and the dedup feature does not apply to them;
--   * empty confirmation codes — manual/paste bookings without a code
--     legitimately can repeat (different user inputs of the same data).
--
-- On a 23505 unique-violation, the service layer re-runs the SELECT-
-- merge path so the second-arriving request gets the same merged record
-- the first one created.
CREATE UNIQUE INDEX IF NOT EXISTS bookings_user_trip_confirmation_unique
  ON bookings (user_id, trip_id, confirmation_code)
  WHERE confirmation_code IS NOT NULL
    AND confirmation_code != ''
    AND trip_id IS NOT NULL;
