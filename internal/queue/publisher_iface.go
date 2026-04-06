package queue

import "context"

// MessagePublisher is the interface that wraps Publish.
// Using an interface allows service tests to inject a mock without a real AMQP broker.
type MessagePublisher interface {
	Publish(ctx context.Context, body []byte) error
}