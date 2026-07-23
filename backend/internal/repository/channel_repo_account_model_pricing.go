package repository

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
)

// ListAccountModelPricingOverrides returns only active, schedulable bindings.
// Joining account_groups prevents a stale rule from applying after an account
// has been removed from the configured group.
func (r *channelRepository) ListAccountModelPricingOverrides(ctx context.Context) ([]service.AccountModelPricingOverride, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT o.id, o.group_id, o.account_id, a.name, o.platform, o.models,
		       o.billing_mode, o.input_price, o.output_price,
		       o.cache_write_price, o.cache_read_price,
		       o.image_input_price, o.image_output_price, o.per_request_price,
		       o.created_at, o.updated_at
		FROM account_model_pricing_overrides o
		JOIN accounts a ON a.id = o.account_id
		JOIN groups g ON g.id = o.group_id
		JOIN account_groups ag ON ag.account_id = o.account_id AND ag.group_id = o.group_id
		JOIN channel_groups cg ON cg.group_id = o.group_id
		JOIN channels c ON c.id = cg.channel_id
		WHERE a.status = 'active' AND a.deleted_at IS NULL
		  AND g.status = 'active' AND g.deleted_at IS NULL
		  AND c.status = 'active'
		ORDER BY o.id`)
	if err != nil {
		return nil, fmt.Errorf("list account model pricing overrides: %w", err)
	}
	defer func() { _ = rows.Close() }()

	overrides := make([]service.AccountModelPricingOverride, 0)
	pricingIDs := make([]int64, 0)
	for rows.Next() {
		var override service.AccountModelPricingOverride
		var modelsJSON []byte
		p := &override.Pricing
		if err := rows.Scan(
			&override.ID, &override.GroupID, &override.AccountID, &override.AccountName,
			&override.Platform, &modelsJSON, &p.BillingMode,
			&p.InputPrice, &p.OutputPrice, &p.CacheWritePrice, &p.CacheReadPrice,
			&p.ImageInputPrice, &p.ImageOutputPrice, &p.PerRequestPrice,
			&override.CreatedAt, &override.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan account model pricing override: %w", err)
		}
		if err := json.Unmarshal(modelsJSON, &p.Models); err != nil {
			return nil, fmt.Errorf("decode account model pricing override %d models: %w", override.ID, err)
		}
		p.ID = override.ID
		p.Platform = override.Platform
		p.CreatedAt = override.CreatedAt
		p.UpdatedAt = override.UpdatedAt
		pricingIDs = append(pricingIDs, override.ID)
		overrides = append(overrides, override)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate account model pricing overrides: %w", err)
	}

	intervals, err := r.batchLoadAccountModelPricingOverrideIntervals(ctx, pricingIDs)
	if err != nil {
		return nil, err
	}
	for i := range overrides {
		overrides[i].Pricing.Intervals = intervals[overrides[i].ID]
	}
	return overrides, nil
}

func (r *channelRepository) batchLoadAccountModelPricingOverrideIntervals(ctx context.Context, pricingIDs []int64) (map[int64][]service.PricingInterval, error) {
	result := make(map[int64][]service.PricingInterval, len(pricingIDs))
	if len(pricingIDs) == 0 {
		return result, nil
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, pricing_id, min_tokens, max_tokens, tier_label,
		       input_price, output_price, cache_write_price, cache_read_price,
		       per_request_price, sort_order, created_at, updated_at
		FROM account_model_pricing_override_intervals
		WHERE pricing_id = ANY($1)
		ORDER BY pricing_id, sort_order, id`, pq.Array(pricingIDs))
	if err != nil {
		return nil, fmt.Errorf("load account model pricing override intervals: %w", err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var interval service.PricingInterval
		if err := rows.Scan(
			&interval.ID, &interval.PricingID, &interval.MinTokens, &interval.MaxTokens,
			&interval.TierLabel, &interval.InputPrice, &interval.OutputPrice,
			&interval.CacheWritePrice, &interval.CacheReadPrice, &interval.PerRequestPrice,
			&interval.SortOrder, &interval.CreatedAt, &interval.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan account model pricing override interval: %w", err)
		}
		result[interval.PricingID] = append(result[interval.PricingID], interval)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate account model pricing override intervals: %w", err)
	}
	return result, nil
}

var _ interface {
	ListAccountModelPricingOverrides(context.Context) ([]service.AccountModelPricingOverride, error)
} = (*channelRepository)(nil)
