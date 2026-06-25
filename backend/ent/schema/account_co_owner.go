package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// AccountCoOwner records admin-assigned additional owners of a contributed
// account. The owner set of an account is its primary owner_user_id plus all
// co-owners recorded here. Single-owner and multi-owner accounts coexist in the
// pool. Co-owners bypass contribution protection on accounts they co-own and
// share contribution rewards evenly with the other owners.
type AccountCoOwner struct {
	ent.Schema
}

func (AccountCoOwner) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "account_co_owners"},
	}
}

func (AccountCoOwner) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("account_id"),
		field.Int64("user_id"),
		// created_by is the admin user who assigned this co-owner (nullable).
		field.Int64("created_by").
			Optional().
			Nillable(),
		field.Time("created_at").
			Immutable().
			Default(time.Now).
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
	}
}

func (AccountCoOwner) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("account_id", "user_id").
			Unique(),
		index.Fields("user_id"),
	}
}
