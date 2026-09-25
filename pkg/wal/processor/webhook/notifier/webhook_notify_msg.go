// SPDX-License-Identifier: Apache-2.0

package notifier

import (
	"fmt"

	"github.com/ApollosProject/pgstream-wal2json/pkg/wal"
	"github.com/ApollosProject/pgstream-wal2json/pkg/wal/processor/webhook"
	"github.com/ApollosProject/pgstream-wal2json/pkg/wal/processor/webhook/subscription"
)

type notifyTarget struct {
	url     string
	headers map[string]string
}

type notifyMsg struct {
	targets        []notifyTarget
	payload        []byte
	commitPosition wal.CommitPosition
}

type serialiser func(any) ([]byte, error)

func newNotifyMsg(event *wal.Event, subscriptions []*subscription.Subscription, serialiser serialiser) (*notifyMsg, error) {
	var payload []byte
	targets := make([]notifyTarget, 0, len(subscriptions))
	if len(subscriptions) > 0 {
		var err error
		payload, err = serialiser(&webhook.Payload{Data: event.Data})
		if err != nil {
			return nil, fmt.Errorf("serialising webhook payload: %w", err)
		}

		for _, s := range subscriptions {
			targets = append(targets, notifyTarget{url: s.URL, headers: s.Headers})
		}
	}

	return &notifyMsg{
		targets:        targets,
		payload:        payload,
		commitPosition: event.CommitPosition,
	}, nil
}

func (m *notifyMsg) urls() []string {
	urls := make([]string, len(m.targets))
	for i, target := range m.targets {
		urls[i] = target.url
	}
	return urls
}

func (m *notifyMsg) size() int {
	size := len(m.payload)
	for _, target := range m.targets {
		size += len(target.url)
		for k, v := range target.headers {
			size += len(k) + len(v)
		}
	}
	return size
}
