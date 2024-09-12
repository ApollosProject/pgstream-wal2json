// SPDX-License-Identifier: Apache-2.0

package webhook

import "github.com/ApollosProject/pgstream-wal2json/pkg/wal"

type Payload struct {
	Data *wal.Data
}
