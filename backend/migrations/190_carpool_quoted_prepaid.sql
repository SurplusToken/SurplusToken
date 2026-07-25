-- Carpool seat-fee reconciliation (席位费找齐): keep the amount the member was
-- actually quoted when boarding, separate from the launch-time locked amount.
--
-- Before this migration, prepaid_amount_cny was written at join time (seat fee
-- split by the member count *at that moment*) and then silently overwritten at
-- launch time (split by the final member count). Settlement derived the seat-fee
-- prepaid part from that same overwritten column, so seat_fee_delta_cny was
-- identically zero -- the "找齐" branch of the settlement was dead code even
-- though real money had changed hands at the join-time quote.
--
--   quoted_prepaid_amount_cny  -> what the member was told to pay when boarding
--                                 (seat_fee/N_at_join + usage_pool x declared/limit)
--   prepaid_amount_cny         -> launch-time locked ledger value (unchanged)
--   seat fee delta             -> quoted seat-fee part - seat_fee/N_at_launch
--
-- Existing rows are backfilled with the locked value, which degrades to the
-- previous behavior (delta 0) instead of inventing a refund that never happened.
ALTER TABLE carpool_members
    ADD COLUMN IF NOT EXISTS quoted_prepaid_amount_cny DECIMAL(20, 2) NULL;

UPDATE carpool_members
SET quoted_prepaid_amount_cny = prepaid_amount_cny
WHERE quoted_prepaid_amount_cny IS NULL
  AND prepaid_amount_cny IS NOT NULL;
