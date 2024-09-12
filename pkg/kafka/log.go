// SPDX-License-Identifier: Apache-2.0

package kafka

import (
	"fmt"

	loglib "github.com/ApollosProject/pgstream-wal2json/pkg/log"
	"github.com/segmentio/kafka-go"
)

func makeLogger(logFn func(msg string, fields ...loglib.Fields)) kafka.LoggerFunc {
	return func(msg string, args ...interface{}) {
		logFn(fmt.Sprintf(msg, args...), nil)
	}
}

func makeErrLogger(logFn func(err error, msg string, fields ...loglib.Fields)) kafka.LoggerFunc {
	return func(msg string, args ...interface{}) {
		logFn(nil, fmt.Sprintf(msg, args...), nil)
	}
}
