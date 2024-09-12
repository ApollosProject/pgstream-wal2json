// SPDX-License-Identifier: Apache-2.0

package integration

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ApollosProject/pgstream-wal2json/internal/log/zerolog"
	pglib "github.com/ApollosProject/pgstream-wal2json/internal/postgres"
	kafkalib "github.com/ApollosProject/pgstream-wal2json/pkg/kafka"
	loglib "github.com/ApollosProject/pgstream-wal2json/pkg/log"
	schemalogpg "github.com/ApollosProject/pgstream-wal2json/pkg/schemalog/postgres"
	"github.com/ApollosProject/pgstream-wal2json/pkg/stream"
	"github.com/ApollosProject/pgstream-wal2json/pkg/tls"
	"github.com/ApollosProject/pgstream-wal2json/pkg/wal"
	kafkacheckpoint "github.com/ApollosProject/pgstream-wal2json/pkg/wal/checkpointer/kafka"
	kafkaprocessor "github.com/ApollosProject/pgstream-wal2json/pkg/wal/processor/kafka"
	"github.com/ApollosProject/pgstream-wal2json/pkg/wal/processor/search/store"
	"github.com/ApollosProject/pgstream-wal2json/pkg/wal/processor/translator"
	"github.com/ApollosProject/pgstream-wal2json/pkg/wal/processor/webhook"
	"github.com/ApollosProject/pgstream-wal2json/pkg/wal/processor/webhook/notifier"
	pgreplication "github.com/ApollosProject/pgstream-wal2json/pkg/wal/replication/postgres"
	"github.com/stretchr/testify/require"
)

var (
	pgurl            string
	kafkaBrokers     []string
	opensearchURL    string
	elasticsearchURL string
)

type mockProcessor struct {
	eventChan chan *wal.Event
}

func (mp *mockProcessor) process(ctx context.Context, event *wal.Event) error {
	mp.eventChan <- event
	return nil
}

func (mp *mockProcessor) close() {
	close(mp.eventChan)
}

type mockWebhookServer struct {
	*httptest.Server
	dataChan chan *wal.Data
}

func newMockWebhookServer() *mockWebhookServer {
	mw := &mockWebhookServer{
		dataChan: make(chan *wal.Data),
	}
	mw.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		payload := webhook.Payload{}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		mw.dataChan <- payload.Data
	}))
	return mw
}

func (m *mockWebhookServer) close() {
	close(m.dataChan)
	m.Server.Close()
}

func runStream(t *testing.T, ctx context.Context, cfg *stream.Config) {
	// start the configured stream listener/processor
	go func() {
		err := stream.Run(ctx, testLogger(), cfg, nil)
		require.NoError(t, err)
	}()
}

func execQuery(t *testing.T, ctx context.Context, query string) {
	conn, err := pglib.NewConn(ctx, pgurl)
	require.NoError(t, err)

	_, err = conn.Exec(ctx, query)
	require.NoError(t, err)
}

func testLogger() loglib.Logger {
	return zerolog.NewStdLogger(zerolog.NewLogger(&zerolog.Config{
		LogLevel: "trace",
	}))
}

func testPostgresListenerCfg() stream.ListenerConfig {
	return stream.ListenerConfig{
		Postgres: &stream.PostgresListenerConfig{
			Replication: pgreplication.Config{
				PostgresURL: pgurl,
			},
		},
	}
}

func testKafkaListenerCfg() stream.ListenerConfig {
	return stream.ListenerConfig{
		Kafka: &stream.KafkaListenerConfig{
			Reader: kafkalib.ReaderConfig{
				Conn:            testKafkaCfg(),
				ConsumerGroupID: "integration-test-group",
			},
			Checkpointer: kafkacheckpoint.Config{},
		},
	}
}

func testKafkaProcessorCfg() stream.ProcessorConfig {
	return stream.ProcessorConfig{
		Kafka: &stream.KafkaProcessorConfig{
			Writer: &kafkaprocessor.Config{
				Kafka: testKafkaCfg(),
			},
		},
		Translator: &translator.Config{
			Store: schemalogpg.Config{
				URL: pgurl,
			},
		},
	}
}

func testSearchProcessorCfg(storeCfg store.Config) stream.ProcessorConfig {
	return stream.ProcessorConfig{
		Search: &stream.SearchProcessorConfig{
			Store: storeCfg,
		},
		Translator: &translator.Config{
			Store: schemalogpg.Config{
				URL: pgurl,
			},
		},
	}
}

func testWebhookProcessorCfg() stream.ProcessorConfig {
	return stream.ProcessorConfig{
		Webhook: &stream.WebhookProcessorConfig{
			Notifier: notifier.Config{},
			SubscriptionStore: stream.WebhookSubscriptionStoreConfig{
				URL: pgurl,
			},
		},
		Translator: &translator.Config{
			Store: schemalogpg.Config{
				URL: pgurl,
			},
		},
	}
}

func testKafkaCfg() kafkalib.ConnConfig {
	return kafkalib.ConnConfig{
		Servers: kafkaBrokers,
		Topic: kafkalib.TopicConfig{
			Name:       "integration-tests",
			AutoCreate: true,
		},
		TLS: tls.Config{
			Enabled: false,
		},
	}
}
