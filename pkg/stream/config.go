// SPDX-License-Identifier: Apache-2.0

package stream

import (
	"errors"
	"time"

	"github.com/ApollosProject/pgstream-wal2json/pkg/kafka"
	kafkacheckpoint "github.com/ApollosProject/pgstream-wal2json/pkg/wal/checkpointer/kafka"
	kafkaprocessor "github.com/ApollosProject/pgstream-wal2json/pkg/wal/processor/kafka"
	"github.com/ApollosProject/pgstream-wal2json/pkg/wal/processor/search"
	"github.com/ApollosProject/pgstream-wal2json/pkg/wal/processor/search/store"
	"github.com/ApollosProject/pgstream-wal2json/pkg/wal/processor/translator"
	"github.com/ApollosProject/pgstream-wal2json/pkg/wal/processor/webhook/notifier"
	"github.com/ApollosProject/pgstream-wal2json/pkg/wal/processor/webhook/subscription/server"
	pgreplication "github.com/ApollosProject/pgstream-wal2json/pkg/wal/replication/postgres"
)

type Config struct {
	Listener  ListenerConfig
	Processor ProcessorConfig
}

type ListenerConfig struct {
	Postgres *PostgresListenerConfig
	Kafka    *KafkaListenerConfig
}

type PostgresListenerConfig struct {
	Replication pgreplication.Config
}

type KafkaListenerConfig struct {
	Reader       kafka.ReaderConfig
	Checkpointer kafkacheckpoint.Config
}

type ProcessorConfig struct {
	Kafka      *KafkaProcessorConfig
	Search     *SearchProcessorConfig
	Webhook    *WebhookProcessorConfig
	Translator *translator.Config
}

type KafkaProcessorConfig struct {
	Writer *kafkaprocessor.Config
}

type SearchProcessorConfig struct {
	Indexer search.IndexerConfig
	Store   store.Config
	Retrier search.StoreRetryConfig
}

type WebhookProcessorConfig struct {
	Notifier           notifier.Config
	SubscriptionServer server.Config
	SubscriptionStore  WebhookSubscriptionStoreConfig
}

type WebhookSubscriptionStoreConfig struct {
	URL                  string
	CacheEnabled         bool
	CacheRefreshInterval time.Duration
}

func (c *Config) IsValid() error {
	if c.Listener.Kafka == nil && c.Listener.Postgres == nil {
		return errors.New("need at least one listener configured")
	}

	if c.Processor.Kafka == nil && c.Processor.Search == nil && c.Processor.Webhook == nil {
		return errors.New("need at least one processor configured")
	}

	return nil
}
