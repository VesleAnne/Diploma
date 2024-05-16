package broker

import (
	"github.com/streadway/amqp"
	"log"
	"os"
	"time"
)

var (
	connectRabbitMQ  *amqp.Connection
	channelRabbitMQ  *amqp.Channel
	rabbitCloseError chan *amqp.Error
	amqpServerURL    string
	queueNameIn      string
	queueNameOut     string
	consumer         messageConsumer
)

const (
	AmqpServerUrlDefault = "amqp://guest:guest@message-broker:5672/"
	QueueNameInDefault   = "hello"
	QueueNameOutDefault  = "out"
)

type messageConsumer func(string)

// Try to connect to the RabbitMQ server as
// long as it takes to establish a connection
// Used into sender
func ConnectToRabbitMQ(uri string) *amqp.Connection {
	for {
		conn, err := amqp.Dial(uri)

		if err == nil {
			log.Println("RabbitMQ connection established")
			return conn
		}

		logError("Connection to rabbitmq failed. Retrying... ", err)
		time.Sleep(10 * time.Second)
	}
}

// Next methods used into consumer
func Consume(consumer messageConsumer) {
	//log.Println("Registering consumer...")
	deliveries, err := registerQueueConsumer()
	logError("Registering consumer failed", err)
	//log.Println("Consumer registered! Processing messages...")
	executeMessageConsumer(err, consumer, deliveries, false)
}

func Reconnector() {
	for {
		err := <-rabbitCloseError
		if err != nil {
			log.Println("Reconnecting after connection closed:", err)
		}

		Connect()
		recoverConsumer()
	}
}

func IsEnvNotExist(param string) bool {
	if param == "" {
		return true
	}
	return false
}

func InitAMQP() {
	// RabbitMQ server URL.
	amqpServerURL = os.Getenv("AMQP_SERVER_URL")
	if IsEnvNotExist(amqpServerURL) {
		amqpServerURL = AmqpServerUrlDefault
	}
	// RabbitMQ Sender queue.
	queueNameIn = os.Getenv("QUEUE_NAME_IN")
	if IsEnvNotExist(queueNameIn) {
		queueNameIn = QueueNameInDefault
	}
	// RabbitMQ Consumer queue .
	queueNameOut = os.Getenv("QUEUE_NAME_OUT")
	if IsEnvNotExist(queueNameOut) {
		queueNameOut = QueueNameOutDefault
	}
}

func Connect() {
	for {
		conn, err := amqp.Dial(amqpServerURL)
		if err == nil {
			connectRabbitMQ = conn
			rabbitCloseError = make(chan *amqp.Error)
			connectRabbitMQ.NotifyClose(rabbitCloseError)

			log.Println("RabbitMQ connection established")

			openChannel()
			declareQueue()

			return
		}

		logError("Connection to rabbitmq failed. Retrying... ", err)
		time.Sleep(10 * time.Second)
	}
}

func declareQueue() {
	_, err := channelRabbitMQ.QueueDeclare(
		queueNameOut, // name
		true,         // durable
		false,        // delete when unused
		false,        // exclusive
		false,        // no-wait
		nil,          // arguments
	)
	logError("Queue declaration failed", err)
}

func openChannel() {
	channel, err := connectRabbitMQ.Channel()
	logError("Opening channel failed", err)
	channelRabbitMQ = channel
}

func registerQueueConsumer() (<-chan amqp.Delivery, error) {
	msgs, err := channelRabbitMQ.Consume(
		queueNameOut, // queue
		"",           // messageConsumer
		true,         // auto-ack
		false,        // exclusive
		false,        // no-local
		false,        // no-wait
		nil,          // args
	)
	logError("Consuming messages from queue failed", err)
	return msgs, err
}

func executeMessageConsumer(err error, newConsumer messageConsumer, deliveries <-chan amqp.Delivery, isRecovery bool) {
	if err == nil {
		if !isRecovery {
			consumer = newConsumer
		}
		go func() {
			for delivery := range deliveries {
				consumer(string(delivery.Body[:]))
			}
		}()
	}
}

func recoverConsumer() {
	//log.Println("Recovering consumer...")
	msgs, err := registerQueueConsumer()
	logError("Recovering consumer failed", err)
	//log.Println("Consumer recovered! Continuing message processing...")
	executeMessageConsumer(err, consumer, msgs, true)
}

func logError(message string, err error) {
	if err != nil {
		log.Printf("%s: %s", message, err)
	}
}
