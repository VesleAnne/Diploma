package main

import (
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/streadway/amqp"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"log"
	"ner/api"
	"ner/api/broker"
	"ner/api/models"
	"os"
	"strconv"
)

var (
	connectRabbitMQ  *amqp.Connection
	channelRabbitMQ  *amqp.Channel
	rabbitCloseError chan *amqp.Error
)

const (
	AmqpServerUrlDefault = "amqp://guest:guest@message-broker:5672/"
	QueueNameInDefault   = "hello"
	DbUrlDefault         = "postgresql://postgres:postgres@postgres:5432?sslmode=disable"
	ChunkSizeDefault     = "100"
)

func main() {
	// Define RabbitMQ server URL.
	amqpServerURL := os.Getenv("AMQP_SERVER_URL")
	if broker.IsEnvNotExist(amqpServerURL) {
		amqpServerURL = AmqpServerUrlDefault
	}
	// Define RabbitMQ queue.
	queueNameIn := os.Getenv("QUEUE_NAME_IN")
	if broker.IsEnvNotExist(queueNameIn) {
		queueNameIn = QueueNameInDefault
	}
	// Define Postgres server URL.
	dbURL := os.Getenv("POSTGRES_URL")
	if broker.IsEnvNotExist(dbURL) {
		dbURL = DbUrlDefault
	}
	// Define chunk size for sending into broker.
	ChunkSize := os.Getenv("CHUNK_SIZE")
	if broker.IsEnvNotExist(ChunkSize) {
		log.Println("ChunkSize: used default size 100")
		ChunkSize = ChunkSizeDefault
	}
	// Create int variable from env string
	ChunkSizeInt, err := strconv.Atoi(ChunkSize)
	if err != nil {
		log.Printf("ChunkSize %s is not a number. Default value used: 100", ChunkSize)
		ChunkSizeInt, err = strconv.Atoi(ChunkSizeDefault)
		if err != nil {
			log.Fatalln(err)
		}
	}

	db, err := gorm.Open(postgres.Open(dbURL), &gorm.Config{})
	if err != nil {
		log.Fatalln(err)
	}

	if db.AutoMigrate(&models.Tokens{}, &models.Result{}); err != nil {
		log.Fatalln(err)
	}

	// Set db and Queue for using into routes
	r := &api.Repository{
		DB:        db,
		Queue:     queueNameIn,
		ChunkSize: ChunkSizeInt,
	}

	// create the rabbitmq error channel
	rabbitCloseError = make(chan *amqp.Error)

	// run the callback in a separate thread
	//go rabbitConnector(queueNameIn, amqpServerURL, channelRabbitMQ)
	go func() {
		var rabbitErr *amqp.Error

		for {
			rabbitErr = <-rabbitCloseError
			if rabbitErr != nil {
				connectRabbitMQ = broker.ConnectToRabbitMQ(amqpServerURL)
				rabbitCloseError = make(chan *amqp.Error)
				connectRabbitMQ.NotifyClose(rabbitCloseError)

				// Let's start by opening a channel to our RabbitMQ
				// instance over the connection we have already
				// established.
				//broker.OpenChannel(connectRabbitMQ, channelRabbitMQ)
				channelRabbitMQ, err = connectRabbitMQ.Channel()
				if err != nil {
					log.Println("rabbitmq channel opening failed: ", err)
				}
				defer channelRabbitMQ.Close()

				// Set ChannelRabbitMQ for using into routes
				r.ChannelRabbitMQ = channelRabbitMQ

				// With the instance and declare Queues that we can
				// publish and subscribe to.
				//broker.DeclareQueue(queueNameIn, channelRabbitMQ)
				_, err = channelRabbitMQ.QueueDeclare(
					queueNameIn, // queue name
					true,        // durable
					false,       // auto delete
					false,       // exclusive
					false,       // no wait
					nil,         // arguments
				)
				if err != nil {
					log.Println("queue declaration failed:", err)
				}
			}
		}
	}()
	// establish the rabbitmq connection by sending
	// an error and thus calling the error callback
	rabbitCloseError <- amqp.ErrClosed

	// Create a new Fiber instance.
	app := fiber.New(fiber.Config{
		BodyLimit: 10 * 1024 * 1024,
	})

	// Add middleware.
	app.Use(
		logger.New(), // add simple logger
	)

	// Register api routes
	r.SetupRoutes(app)

	// Start Fiber API server.
	log.Fatal(app.Listen(":3000"))
}
