// SPDX-License-Identifier: Apache-2.0

package kafka

import (
	"errors"
	"fmt"
	"net"
	"strconv"
	"time"

	"github.com/segmentio/kafka-go"
)

type ConnConfig struct {
	Servers []string
	Topic   TopicConfig
	TLS     *TLSConfig
}

type TopicConfig struct {
	Name              string
	NumPartitions     int
	ReplicationFactor int
	AutoCreate        bool
}

// withConnection creates a connection that can be used by the kafka operation
// passed in the parameters. This ensures the cleanup of all connection resources.
func withConnection(config *ConnConfig, kafkaOperation func(conn *kafka.Conn) error) error {
	dialer, err := buildDialer(config.TLS)
	if err != nil {
		return err
	}

	var conn *kafka.Conn
	for _, server := range config.Servers {
		conn, err = dialer.Dial("tcp", server)
		if err != nil {
			// Try next server in the list
			continue
		}
		defer conn.Close()

		// Successfully connected. Do not try the other servers
		break
	}

	if conn == nil {
		return errors.New("error connecting to kafka, all servers failed")
	}

	controller, err := conn.Controller()
	if err != nil {
		return fmt.Errorf("controller: %w", err)
	}
	var controllerConn *kafka.Conn

	controllerConn, err = dialer.Dial("tcp", net.JoinHostPort(controller.Host, strconv.Itoa(controller.Port)))
	if err != nil {
		return fmt.Errorf("controller connection: %w", err)
	}
	defer controllerConn.Close()

	return kafkaOperation(controllerConn)
}

func buildDialer(tlsConfig *TLSConfig) (*kafka.Dialer, error) {
	timeout := 10 * time.Second

	dialer := &kafka.Dialer{
		Timeout:   timeout,
		DualStack: true,
	}
	if tlsConfig.Enabled {
		var err error
		dialer, err = buildTLSDialer(tlsConfig, timeout)
		if err != nil {
			return nil, fmt.Errorf("building dialer: %w", err)
		}
	}

	return dialer, nil
}
