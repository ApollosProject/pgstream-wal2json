// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"
	"encoding/json"
	"fmt"

	pglib "github.com/ApollosProject/pgstream-wal2json/internal/postgres"
	loglib "github.com/ApollosProject/pgstream-wal2json/pkg/log"
	"github.com/ApollosProject/pgstream-wal2json/pkg/wal/processor/webhook/subscription"
)

type Store struct {
	conn   pglib.Querier
	logger loglib.Logger
}

type Option func(*Store)

const (
	subscriptionsTableName = "webhook_subscriptions"
	pgstreamSchema         = "pgstream"
)

func NewSubscriptionStore(ctx context.Context, url string, opts ...Option) (*Store, error) {
	pgpool, err := pglib.NewConnPool(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("create postgres connection pool: %w", err)
	}
	ss := &Store{
		conn: pgpool,
	}

	for _, opt := range opts {
		opt(ss)
	}

	// create subscriptions table if it doesn't exist
	if err := ss.createTable(ctx); err != nil {
		return nil, fmt.Errorf("creating subscriptions table: %w", err)
	}

	return ss, nil
}

func WithLogger(l loglib.Logger) Option {
	return func(ss *Store) {
		ss.logger = loglib.NewLogger(l).WithFields(loglib.Fields{
			loglib.ServiceField: "webhook_subscription_store",
		})
	}
}

func (s *Store) CreateSubscription(ctx context.Context, subscription *subscription.Subscription) error {
	headers, err := headersParam(subscription.Headers)
	if err != nil {
		return err
	}
	// Re-subscribing the same url/schema/table replaces headers, same as event_types.
	query := fmt.Sprintf(`
	INSERT INTO %s(url, schema_name, table_name, event_types, headers) VALUES($1, $2, $3, $4, $5::jsonb)
	ON CONFLICT (url,schema_name,table_name) DO UPDATE SET event_types = EXCLUDED.event_types, headers = EXCLUDED.headers;`, subscriptionsTable())
	_, err = s.conn.Exec(ctx, query, subscription.URL, subscription.Schema, subscription.Table, subscription.EventTypes, headers)
	return err
}

func (s *Store) DeleteSubscription(ctx context.Context, subscription *subscription.Subscription) error {
	query := fmt.Sprintf(`DELETE FROM %s WHERE url=$1 AND schema_name=$2 AND table=$3;`, subscriptionsTable())
	_, err := s.conn.Exec(ctx, query, subscription.URL, subscription.Schema, subscription.Table)
	return err
}

func (s *Store) GetSubscriptions(ctx context.Context, action, schema, table string) ([]*subscription.Subscription, error) {
	query, params := s.buildGetQuery(action, schema, table)
	s.logger.Trace("getting subscriptions", loglib.Fields{
		"query":  query,
		"params": params,
	})
	rows, err := s.conn.Query(ctx, query, params...)
	if err != nil {
		return nil, fmt.Errorf("querying subscriptions table: %w", err)
	}
	defer rows.Close()

	subscriptions := []*subscription.Subscription{}
	for rows.Next() {
		subscription := &subscription.Subscription{}
		if err := rows.Scan(&subscription.URL, &subscription.Schema, &subscription.Table, &subscription.EventTypes, &subscription.Headers); err != nil {
			return nil, fmt.Errorf("scanning subscription row: %w", err)
		}

		subscriptions = append(subscriptions, subscription)
	}

	return subscriptions, nil
}

func (s *Store) createTable(ctx context.Context) error {
	query := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s(
	url TEXT,
	schema_name TEXT,
	table_name TEXT,
	event_types TEXT[],
	headers JSONB,
	PRIMARY KEY(url,schema_name,table_name))`, subscriptionsTable())
	if _, err := s.conn.Exec(ctx, query); err != nil {
		return err
	}
	// webhook_subscriptions is created here, not by the schema-log migrator.
	// Existing installs need the column added in place.
	alter := fmt.Sprintf(`ALTER TABLE %s ADD COLUMN IF NOT EXISTS headers JSONB`, subscriptionsTable())
	_, err := s.conn.Exec(ctx, alter)
	return err
}

func (s *Store) buildGetQuery(action, schema, table string) (string, []any) {
	query := fmt.Sprintf(`SELECT url, schema_name, table_name, event_types, headers FROM %s`, subscriptionsTable())

	separator := func(params []any) string {
		if len(params) == 0 {
			return "WHERE"
		}
		return "AND"
	}
	var params []any
	if schema != "" {
		query = fmt.Sprintf("%s %s (schema_name=$%d OR schema_name='')", query, separator(params), len(params)+1)
		params = append(params, schema)
	}
	if table != "" {
		query = fmt.Sprintf("%s %s (table_name=$%d OR table_name='')", query, separator(params), len(params)+1)
		params = append(params, table)
	}
	if action != "" {
		query = fmt.Sprintf("%s %s ($%d=ANY(event_types) OR event_types IS NULL)", query, separator(params), len(params)+1)
		params = append(params, action)
	}

	return fmt.Sprintf("%s LIMIT 1000", query), params
}

func subscriptionsTable() string {
	return fmt.Sprintf("%s.%s", pgstreamSchema, subscriptionsTableName)
}

// headersParam encodes headers as a JSON string for a jsonb cast.
// Nil and empty maps are stored as NULL so older rows and header-less
// subscriptions stay equivalent.
func headersParam(headers map[string]string) (any, error) {
	if len(headers) == 0 {
		return nil, nil
	}
	b, err := json.Marshal(headers)
	if err != nil {
		return nil, fmt.Errorf("encoding subscription headers: %w", err)
	}
	return string(b), nil
}
