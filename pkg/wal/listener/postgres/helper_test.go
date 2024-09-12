// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"
	"time"

	"github.com/ApollosProject/pgstream-wal2json/pkg/wal/replication"
	replicationmocks "github.com/ApollosProject/pgstream-wal2json/pkg/wal/replication/mocks"
)

const (
	testLSN    = replication.LSN(7773397064)
	testLSNStr = "1/CF54A048"
)

func newMockReplicationHandler() *replicationmocks.Handler {
	return &replicationmocks.Handler{
		StartReplicationFn: func(context.Context) error { return nil },
		GetLSNParserFn:     func() replication.LSNParser { return newMockLSNParser() },
		SyncLSNFn:          func(ctx context.Context, lsn replication.LSN) error { return nil },
		ReceiveMessageFn: func(ctx context.Context, i uint64) (*replication.Message, error) {
			return newMockMessage(), nil
		},
	}
}

func newMockMessage() *replication.Message {
	return &replication.Message{
		LSN:            testLSN,
		Data:           []byte("test-data"),
		ReplyRequested: false,
		ServerTime:     time.Now(),
	}
}

func newMockKeepAliveMessage(replyRequested bool) *replication.Message {
	return &replication.Message{
		LSN:            testLSN,
		ReplyRequested: replyRequested,
	}
}

func newMockLSNParser() *replicationmocks.LSNParser {
	return &replicationmocks.LSNParser{
		ToStringFn:   func(replication.LSN) string { return testLSNStr },
		FromStringFn: func(s string) (replication.LSN, error) { return testLSN, nil },
	}
}
