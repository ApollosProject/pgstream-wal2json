// SPDX-License-Identifier: Apache-2.0

package search

import (
	"context"
	"fmt"
	"testing"

	"github.com/ApollosProject/pgstream-wal2json/pkg/backoff"
	loglib "github.com/ApollosProject/pgstream-wal2json/pkg/log"
	"github.com/stretchr/testify/require"
)

func TestStoreRetrier_SendDocuments(t *testing.T) {
	t.Parallel()

	testDocs := []Document{
		*newTestDocument(withID("1")),
		*newTestDocument(withID("2")),
		*newTestDocument(withID("3")),
	}

	failedDocs := func(severity Severity) []DocumentError {
		return []DocumentError{
			{
				Document: *newTestDocument(withID("1")),
				Severity: severity,
				Error:    errTest.Error(),
			},
		}
	}

	tests := []struct {
		name            string
		store           *mockStore
		backoffProvider backoff.Provider

		wantFailedDocs []DocumentError
		wantErr        error
	}{
		{
			name: "ok",
			store: &mockStore{
				sendDocumentsFn: func(ctx context.Context, i uint, docs []Document) ([]DocumentError, error) {
					require.Equal(t, testDocs, docs)
					return nil, nil
				},
			},
			wantFailedDocs: []DocumentError{},
			wantErr:        nil,
		},
		{
			name: "ok - transient error",
			store: &mockStore{
				sendDocumentsFn: func(ctx context.Context, i uint, docs []Document) ([]DocumentError, error) {
					switch i {
					case 1:
						require.Equal(t, testDocs, docs)
						return nil, errTest
					case 2:
						require.Equal(t, testDocs, docs)
						return nil, nil
					default:
						return nil, fmt.Errorf("sendDocumentsFn: unexpected call %d", i)
					}
				},
			},
			wantFailedDocs: []DocumentError{},
			wantErr:        nil,
		},
		{
			name: "ok - failed and dropped documents",
			store: &mockStore{
				sendDocumentsFn: func(ctx context.Context, i uint, docs []Document) ([]DocumentError, error) {
					switch i {
					case 1:
						require.Equal(t, testDocs, docs)
						return append(failedDocs(SeverityDataLoss), failedDocs(SeverityRetriable)...), nil
					case 2, 3:
						require.Equal(t, []Document{*newTestDocument(withID("1"))}, docs)
						return failedDocs(SeverityRetriable), nil
					default:
						return nil, fmt.Errorf("sendDocumentsFn: unexpected call %d", i)
					}
				},
			},
			wantFailedDocs: append(failedDocs(SeverityRetriable), failedDocs(SeverityDataLoss)...),
			wantErr:        nil,
		},
		{
			name: "ok - all failed documents dropped",
			store: &mockStore{
				sendDocumentsFn: func(ctx context.Context, i uint, docs []Document) ([]DocumentError, error) {
					switch i {
					case 1:
						require.Equal(t, testDocs, docs)
						return failedDocs(SeverityDataLoss), nil
					default:
						return nil, fmt.Errorf("sendDocumentsFn: unexpected call %d", i)
					}
				},
			},
			wantFailedDocs: failedDocs(SeverityDataLoss),
			wantErr:        nil,
		},
		{
			name: "ok - some failed documents",
			store: &mockStore{
				sendDocumentsFn: func(ctx context.Context, i uint, docs []Document) ([]DocumentError, error) {
					switch i {
					case 1:
						require.Equal(t, testDocs, docs)
						return failedDocs(SeverityRetriable), nil
					case 2, 3:
						require.Equal(t, []Document{*newTestDocument(withID("1"))}, docs)
						return failedDocs(SeverityRetriable), nil
					default:
						return nil, fmt.Errorf("sendDocumentsFn: unexpected call %d", i)
					}
				},
			},
			wantFailedDocs: failedDocs(SeverityRetriable),
			wantErr:        nil,
		},
		{
			name: "error - store error",
			store: &mockStore{
				sendDocumentsFn: func(ctx context.Context, i uint, docs []Document) ([]DocumentError, error) {
					return nil, errTest
				},
			},
			wantFailedDocs: nil,
			wantErr:        errTest,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			retrier := StoreRetrier{
				inner:           tc.store,
				logger:          loglib.NewNoopLogger(),
				backoffProvider: newMockBackoffProvider(),
			}

			failedDocs, err := retrier.SendDocuments(context.Background(), testDocs)
			require.ErrorIs(t, err, tc.wantErr)
			require.Equal(t, tc.wantFailedDocs, failedDocs)
		})
	}
}

// mock backoff provider runs the operation for up to 2 times until it succeeds
// or returns error
func newMockBackoffProvider() backoff.Provider {
	return func(ctx context.Context) backoff.Backoff {
		return backoff.NewConstantBackoff(ctx, &backoff.ConstantConfig{
			Interval:   0,
			MaxRetries: 2,
		})
	}
}
