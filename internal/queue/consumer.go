package queue

import (
	"context"
	"fmt"
	"log"

	amqp "github.com/rabbitmq/amqp091-go"
)

// Consumer wraps an AMQP connection and channel for consuming messages.
type Consumer struct {
	conn    *amqp.Connection
	channel *amqp.Channel
	queue   string
}

func NewConsumer(amqpURL, queueName string) (*Consumer, error) {
	conn, err := amqp.Dial(amqpURL)
	if err != nil {
		return nil, fmt.Errorf("dial amqp: %w", err)
	}

	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("open channel: %w", err)
	}

	// Declare the queue idempotently — same args as publisher.
	_, err = ch.QueueDeclare(
		queueName,
		true,  // durable
		false, // autoDelete
		false, // exclusive
		false, // noWait
		nil,
	)
	if err != nil {
		ch.Close()
		conn.Close()
		return nil, fmt.Errorf("declare queue %q: %w", queueName, err)
	}

	// prefetch=1: pull one message at a time so multiple workers share the load evenly.
	if err := ch.Qos(1, 0, false); err != nil {
		ch.Close()
		conn.Close()
		return nil, fmt.Errorf("set qos: %w", err)
	}

	return &Consumer{conn: conn, channel: ch, queue: queueName}, nil
}

// Consume starts consuming messages and calls handler for each one.
// If handler returns nil the message is acked; otherwise it is nacked with
// requeue=false so it routes to a dead-letter queue rather than looping forever.
func (c *Consumer) Consume(ctx context.Context, handler func([]byte) error) error {
	deliveries, err := c.channel.Consume(
		c.queue,
		"",    // consumer tag — auto-generated
		false, // autoAck — we ack manually
		false, // exclusive
		false, // noLocal
		false, // noWait
		nil,
	)
	if err != nil {
		return fmt.Errorf("consume: %w", err)
	}

	for {
		select {
		case delivery, ok := <-deliveries:
			if !ok {
				return fmt.Errorf("delivery channel closed")
			}

			if err := handler(delivery.Body); err != nil {
				log.Printf("handler error, nacking message: %v", err)
				_ = delivery.Nack(false, false) // requeue=false → DLQ
			} else {
				_ = delivery.Ack(false)
			}

		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (c *Consumer) Close() {
	c.channel.Close()
	c.conn.Close()
}
