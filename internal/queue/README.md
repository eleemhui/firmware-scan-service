# internal/queue/

RabbitMQ publisher and consumer wrappers.

## Publisher (`publisher.go`)

`NewPublisher(amqpURL, queueName)` connects to RabbitMQ, opens a channel, and declares the queue (durable, idempotent). Returns a `*Publisher`.

`Publisher.Publish(ctx, body)` publishes a message as `application/json` with `DeliveryMode: Persistent` so messages survive a broker restart. A mutex guards the channel because `amqp.Channel` is not goroutine-safe.

Used by: **API** (publish new scan jobs) and **worker** (re-publish stale scans from the watchdog).

## Consumer (`consumer.go`)

`NewConsumer(amqpURL, queueName)` connects and declares the same queue. Sets `prefetch=1` so each worker instance receives one message at a time, distributing load evenly across scaled workers.

`Consumer.Consume(ctx, handler)` starts consuming. For each message:
- Calls `handler(body)`
- **Acks** on success
- **Nacks with `requeue=false`** on error — routes the message to the dead-letter queue rather than looping

## Queue configuration

| Property | Value |
|----------|-------|
| Name | `firmware_scan_jobs` (configurable via `QUEUE_NAME`) |
| Durable | yes — survives broker restart |
| Message persistence | `Persistent` delivery mode |
| Prefetch | 1 per consumer |
| Failed messages | nacked, not requeued (dead-letter queue) |
