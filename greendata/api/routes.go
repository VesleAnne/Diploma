package api

import (
	"encoding/json"
	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/streadway/amqp"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"log"
	"ner/api/models"
	"ner/api/util"
	"net/http"
)

type Repository struct {
	DB              *gorm.DB
	ChannelRabbitMQ *amqp.Channel
	Queue           string
	ChunkSize       int
}

var ChannelMQ *amqp.Channel

func createSID(sid *string) error {
	// sid generation if json does not contain it
	uuid, err := uuid.NewUUID()
	if err != nil {
		log.Println("Error: ", err)
		return err
	}
	*sid = uuid.String()
	return nil
}

func (r *Repository) publishMessage(message amqp.Publishing) error {
	// Attempt to publish a message to the queue.
	if err := r.ChannelRabbitMQ.Publish(
		"",      // exchange
		r.Queue, // queue name
		false,   // mandatory
		false,   // immediate
		message, // message to publish
	); err != nil {
		//log.Println("RabbitMQ publish error:: ", err)
		return err
	}
	return nil
}

func (r *Repository) CreateResult(context *fiber.Ctx) error {
	if !util.IsJSON(string(context.Body())) {
		log.Println("Error: ", "JSON is not valid")
		return context.Status(http.StatusBadRequest).JSON(
			&fiber.Map{"message": "JSON is not valid"})
	}
	var result models.Result
	if err := context.BodyParser(&result); err != nil {
		log.Println("Error: ", err.Error())
		return context.Status(http.StatusBadRequest).JSON(
			&fiber.Map{"message": err.Error()})
	}
	validator := validator.New()
	if err := validator.Struct(models.Result{}); err != nil {
		log.Println("Error: ", err)
		return context.Status(http.StatusBadRequest).JSON(
			&fiber.Map{"message": err.Error()},
		)
	}
	if result.SessionID == "" {
		for {
			if err := createSID(&result.SessionID); err != nil {
				log.Println("Error: ", err)
				continue
			}
			var exists bool
			if err := r.DB.Model(result).
				Where("sid = ?", result.SessionID).
				Select("count(*) > 0").
				Find(&exists).Error; err != nil {
				log.Println("Error: ", err)
				return context.Status(http.StatusInternalServerError).JSON(
					&fiber.Map{"message": err.Error()})
			}
			if exists {
				continue
			}
			break
		}
	}
	result.Message = "waiting"
	if err := r.DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "sid"}},
		DoUpdates: clause.Assignments(map[string]interface{}{"message": "waiting"}),
	}).Create(&result).Error; err != nil {
		log.Println(err)
		return context.Status(http.StatusInternalServerError).JSON(
			&fiber.Map{"message": err.Error()})
	}

	var data2ner models.Data2Ner
	if err := context.BodyParser(&data2ner); err != nil {
		log.Println("Error: ", err.Error())
		return context.Status(http.StatusBadRequest).JSON(
			&fiber.Map{"message": err.Error()})
	}

	if r.ChunkSize != 0 && len(data2ner.Tokens) > r.ChunkSize {
		//multiple sending by chunk
		i := 0
		var chunkResult models.Data2Ner
		chunkResult.SessionID = data2ner.SessionID
		var err error
		for i < len(data2ner.Tokens) {
			if len(data2ner.Tokens)-i > r.ChunkSize {
				chunkResult.Tokens = append(chunkResult.Tokens, data2ner.Tokens[i:i+r.ChunkSize]...)
			} else {
				chunkResult.Tokens = append(chunkResult.Tokens, data2ner.Tokens[i:len(data2ner.Tokens)]...)
			}
			var jsonChunk []byte
			jsonChunk, err = json.Marshal(chunkResult)
			if err != nil {
				log.Println("Error: ", err)
			}
			message := amqp.Publishing{
				ContentType: "text/plain",
				Body:        jsonChunk,
			}

			// Attempt to publish a message to the queue.
			if err = r.publishMessage(message); err != nil {
				log.Println("RabbitMQ publishing error: ", err)
			}
			chunkResult.Tokens = nil
			i += r.ChunkSize
		}
		//If an error occurs while chunked sending, we will respond.
		if err != nil {
			return context.Status(http.StatusInternalServerError).JSON(
				&fiber.Map{"message": err.Error() + " .Check sender logs"})
		}
	} else {
		// single sending
		message := amqp.Publishing{
			ContentType: "text/plain",
			Body:        context.Body(),
		}
		// Attempt to publish a message to the queue.
		if err := r.publishMessage(message); err != nil {
			log.Println("RabbitMQ publishing error: ", err)
			return context.Status(http.StatusInternalServerError).JSON(
				&fiber.Map{"message": err.Error()})
		}
	}

	return context.Status(http.StatusOK).JSON(&fiber.Map{
		"message": "data sent for processing",
		"sid":     result.SessionID,
	})
}

func (r *Repository) CreateResultFromFile(context *fiber.Ctx) error {
	var result models.Result

	// Context check for emptiness
	if len(context.Body()) == 0 {
		log.Println("Error: ", "File is empty")
		return context.Status(http.StatusBadRequest).JSON(
			&fiber.Map{"message": "File is empty"})
	}

	// Splitting a string into an array
	rawArray := util.SplitAny(string(context.Body()), "\n\r")
	// Create new sid
	for {
		if err := createSID(&result.SessionID); err != nil {
			log.Println("Error: ", err)
			return context.Status(http.StatusInternalServerError).JSON(
				&fiber.Map{"message": err.Error()})
		}
		var exists bool
		if err := r.DB.Model(result).
			Where("sid = ?", result.SessionID).
			Select("count(*) > 0").
			Find(&exists).Error; err != nil {
			log.Println("Error: ", err)
			return context.Status(http.StatusInternalServerError).JSON(
				&fiber.Map{"message": err.Error()})
		}
		if exists {
			continue
		}
		break
	}
	// Creating and filling a structure for ner with data
	var data models.Data2Ner
	data.SessionID = result.SessionID

	if r.ChunkSize != 0 && len(rawArray) > r.ChunkSize {
		/*if the chunk size is not equal to 0 and the
		number of records is greater than the chunk size,
		start sending by chunks
		*/
		var err error
		counter := 0
		for index, str := range rawArray {
			token := models.Data{
				Id:        index,
				IncString: str,
			}
			data.Tokens = append(data.Tokens, token)
			counter++
			if counter == r.ChunkSize {
				// publish a chunk that is a multiple of the chunk size
				var jsonChunk []byte
				jsonChunk, err = json.Marshal(data)
				if err != nil {
					log.Println("Error: ", err)
				}
				message := amqp.Publishing{
					ContentType: "text/plain",
					Body:        jsonChunk,
				}
				if err = r.publishMessage(message); err != nil {
					log.Println("Error: ", err)
				}
				data.Tokens = nil
				counter = 0
				continue
			}
			if index == len(rawArray)-1 {
				// publish the remaining chunk
				var jsonChunk []byte
				jsonChunk, err = json.Marshal(data)
				if err != nil {
					log.Println("Error: ", err)
				}
				message := amqp.Publishing{
					ContentType: "text/plain",
					Body:        jsonChunk,
				}
				if err = r.publishMessage(message); err != nil {
					log.Println(err)
				}
				data.Tokens = nil
			}
			//If an error occurs while chunked sending, we will respond.
			if err != nil {
				return context.Status(http.StatusInternalServerError).JSON(
					&fiber.Map{"message": err.Error() + " .Check sender logs"})
			}
		}
	} else {
		// otherwise single sending
		for index, str := range rawArray {
			token := models.Data{
				Id:        index,
				IncString: str,
			}
			data.Tokens = append(data.Tokens, token)
		}
		jsonChunk, err := json.Marshal(data)
		if err != nil {
			log.Println("Error: ", err)
		}
		message := amqp.Publishing{
			ContentType: "text/plain",
			Body:        jsonChunk,
		}

		if err = r.publishMessage(message); err != nil {
			log.Println("RabbitMQ publishing error: ", err)
			return context.Status(http.StatusInternalServerError).JSON(
				&fiber.Map{"message": err.Error()})
		}
	}

	// Adding a new record to the database
	result.Message = "waiting"
	if err := r.DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "sid"}},
		DoUpdates: clause.Assignments(map[string]interface{}{"message": "waiting"}),
	}).Create(&result).Error; err != nil {
		log.Println(err)
		return context.Status(http.StatusInternalServerError).JSON(
			&fiber.Map{"message": err.Error()})
	}

	return context.Status(http.StatusOK).JSON(&fiber.Map{
		"message": "data sent for processing",
		"sid":     result.SessionID,
	})
}

func (r *Repository) GetResults(context *fiber.Ctx) error {
	var allData []models.Result
	var results []models.Response
	//taking all data on request
	if err := r.DB.Find(&allData).Error; err != nil {
		return context.Status(http.StatusNotFound).JSON(
			&fiber.Map{"message": err.Error()})
	}
	for i := range allData {
		if err := r.DB.Where("sid=?", allData[i].SessionID).Find(&allData[i].Tokens).Error; err != nil {
			context.Status(http.StatusNotFound).JSON(
				&fiber.Map{"message": err.Error()})
			return nil
		}
		//writing the necessary data to the result
		res := models.Response{
			SessionID: allData[i].SessionID,
			Message:   allData[i].Message,
			Tokens:    allData[i].Tokens,
		}
		results = append(results, res)
	}
	return context.Status(http.StatusOK).JSON(results)
}

func (r *Repository) GetResult(context *fiber.Ctx) error {
	sid := context.Params("sid")
	result := &models.Result{}
	if sid == "" {
		return context.Status(http.StatusBadRequest).JSON(&fiber.Map{
			"message": "sid cannot be empty",
		})
	}
	if err := r.DB.Where("sid = ?", sid).First(result).Error; err != nil {
		return context.Status(http.StatusNotFound).JSON(
			&fiber.Map{"message": err.Error()})
	}
	if err := r.DB.Where("sid=?", sid).Find(&result.Tokens).Error; err != nil {
		return context.Status(http.StatusNotFound).JSON(
			&fiber.Map{"message": err.Error()})
	}
	resp := models.Response{
		SessionID: result.SessionID,
		Message:   result.Message,
		Tokens:    result.Tokens,
	}
	return context.Status(http.StatusOK).JSON(resp)
}

func (r *Repository) GetResultStat(context *fiber.Ctx) error {
	sid := context.Params("sid")
	result := &models.Result{}
	if sid == "" {
		return context.Status(http.StatusBadRequest).JSON(&fiber.Map{
			"message": "sid cannot be empty",
		})
	}
	if err := r.DB.Where("sid = ?", sid).First(result).Error; err != nil {
		return context.Status(http.StatusNotFound).JSON(
			&fiber.Map{"message": err.Error()})
	}
	if err := r.DB.Where("sid=?", sid).Find(&result.Tokens).Error; err != nil {
		return context.Status(http.StatusNotFound).JSON(
			&fiber.Map{"message": err.Error()})
	}
	return context.Status(http.StatusOK).JSON(result)
}

func (r *Repository) SetupRoutes(app *fiber.App) {
	api := app.Group("/api")
	api.Post("/send", r.CreateResult)
	api.Post("/send/file", r.CreateResultFromFile)
	api.Get("/get", r.GetResults)
	api.Get("/get/:sid", r.GetResult)
	api.Get("/get/stat/:sid", r.GetResultStat)
}
