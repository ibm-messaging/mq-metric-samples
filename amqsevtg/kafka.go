/*
 * This is an example of a Go program to format IBM MQ Event messages.
 * It is essentially a rewrite of the product's sample program amqsevta.c
 *
 * Functions in this file handle exporting records to a Kafka topic.
 */

/*
Copyright (c) IBM Corporation 2025, 2026

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the license.

	Contributors:
	  Mark Taylor - Initial Contribution
*/
package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/IBM/sarama"
)

var (
	producer sarama.SyncProducer
)

func initKafka() error {
	config := sarama.NewConfig()
	config.Producer.Return.Successes = true
	config.Producer.Return.Errors = true

	brokers := strings.Split(cf.KafkaBrokers, ",")
	for i := range brokers {
		brokers[i] = strings.TrimSpace(brokers[i])
	}

	fmt.Fprintf(statusStream, "Connecting to Kafka broker(s): %s, topic: %s\n", strings.Join(brokers, ","), cf.KafkaTopic)

	var err error
	producer, err = sarama.NewSyncProducer(brokers, config)
	if err != nil {
		return fmt.Errorf("failed to create Kafka producer: %w", err)
	}

	return nil
}

// Called at the end of the program to close the Kafka producer
func shutdownKafka() {
	if producer != nil {
		fmt.Fprintf(statusStream, "Shutting down Kafka producer\n")
		if err := producer.Close(); err != nil {
			fmt.Fprintf(statusStream, "Error closing Kafka producer: %v\n", err)
		}
	}
}

// Write the event message to the Kafka topic
func kafkaProduce(event Event) {
	var b []byte
	var err error

	if cf.compact {
		b, err = json.Marshal(event)
	} else {
		b, err = json.MarshalIndent(event, "", "  ")
	}

	if err != nil {
		fmt.Fprintf(statusStream, "Error marshaling event: %v\n", err)
		return
	}

	msg := &sarama.ProducerMessage{
		Topic: cf.KafkaTopic,
		Value: sarama.ByteEncoder(b),
	}

	if event.EventSource.ObjectName != "" {
		msg.Key = sarama.StringEncoder(event.EventSource.ObjectName)
	} else if event.EventSource.QueueManager != "" {
		msg.Key = sarama.StringEncoder(event.EventSource.QueueManager)
	}

	if event.EventCreation.Epoch > 0 {
		msg.Timestamp = time.Unix(event.EventCreation.Epoch, 0)
	}

	_, _, err = producer.SendMessage(msg)
	if err != nil {
		fmt.Fprintf(statusStream, "Failed to send message to Kafka: %v\n", err)
	}
}
