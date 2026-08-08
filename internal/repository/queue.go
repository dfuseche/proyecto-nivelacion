package repository

import (
	"encoding/json"
	"fmt"
	"log"

	amqp "github.com/rabbitmq/amqp091-go"

	"okf-converter/internal/domain"
)

type Queue struct {
	conn      *amqp.Connection
	channel   *amqp.Channel
	queueName string
}

func NewQueue(url, queueName string) (*Queue, error) {
	conn, err := amqp.Dial(url)
	if err != nil {
		return nil, fmt.Errorf("error conectando a RabbitMQ: %w", err)
	}

	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("error abriendo canal de RabbitMQ: %w", err)
	}

	_, err = ch.QueueDeclare(
		queueName, // name
		true,      // durable
		false,     // delete when unused
		false,     // exclusive
		false,     // no-wait
		nil,       // arguments
	)
	if err != nil {
		ch.Close()
		conn.Close()
		return nil, fmt.Errorf("error declarando cola '%s': %w", queueName, err)
	}

	log.Printf("[QUEUE] Conexión a RabbitMQ establecida exitosamente en la cola '%s'", queueName)
	return &Queue{
		conn:      conn,
		channel:   ch,
		queueName: queueName,
	}, nil
}

func (q *Queue) PublishJob(payload domain.JobPayload) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("error serializando payload de trabajo: %w", err)
	}

	err = q.channel.Publish(
		"",          // exchange
		q.queueName, // routing key
		false,       // mandatory
		false,       // immediate
		amqp.Publishing{
			DeliveryMode: amqp.Persistent,
			ContentType:  "application/json",
			Body:         body,
		},
	)
	if err != nil {
		return fmt.Errorf("error publicando trabajo en RabbitMQ: %w", err)
	}

	log.Printf("[QUEUE] Trabajo publicado en la cola: %s", payload.JobID)
	return nil
}

func (q *Queue) ConsumeJobs() (<-chan amqp.Delivery, error) {
	err := q.channel.Qos(
		1,     // prefetch count
		0,     // prefetch size
		false, // global
	)
	if err != nil {
		return nil, fmt.Errorf("error configurando QoS de RabbitMQ: %w", err)
	}

	msgs, err := q.channel.Consume(
		q.queueName, // queue
		"",          // consumer
		false,       // auto-ack (desactivado para control explícito de ACK)
		false,       // exclusive
		false,       // no-local
		false,       // no-wait
		nil,         // args
	)
	if err != nil {
		return nil, fmt.Errorf("error iniciando consumidor de RabbitMQ: %w", err)
	}

	return msgs, nil
}

func (q *Queue) Close() {
	if q.channel != nil {
		q.channel.Close()
	}
	if q.conn != nil {
		q.conn.Close()
	}
}
