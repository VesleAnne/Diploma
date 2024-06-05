package main

import (
	"encoding/json"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"log"
	"math"
	"ner/api/broker"
	"ner/api/models"
	"os"
	"reflect"
)

const DbUrlDefault = "postgresql://postgres:postgres@postgres:5432?sslmode=disable"

func main() {
	dbURL := os.Getenv("POSTGRES_URL")
	if broker.IsEnvNotExist(dbURL) {
		dbURL = DbUrlDefault
	}
	db, err := gorm.Open(postgres.Open(dbURL), &gorm.Config{})
	if err != nil {
		log.Fatalln(err)
	}

	// get the maximum chunk size from postgres limit (65535) & num of models.Tokens{} fields
	postgresMaxRowsInChunk := math.Floor(float64(65535 / reflect.TypeOf(models.Tokens{}).NumField()))
	broker.InitAMQP()
	broker.Connect()
	go broker.Reconnector()

	forever := make(chan bool)
	go func() {
		broker.Consume(func(message string) {
			var result models.Result
			if err = json.Unmarshal([]byte(message), &result); err != nil {
				log.Println("Unmarshalling error: ", err)
			}
			log.Println("Response from greendata bot received: ", result.SessionID)
			if err = db.Model(&models.Result{}).Where("sid = ?", result.SessionID).Updates(result).Error; err != nil {
				log.Println(err)
			}
			var tokens []models.Tokens
			for i := range result.Tokens {
				token := result.Tokens[i]
				token.SessionID = result.SessionID
				tokens = append(tokens, token)
			}
			tx := db.Begin()
			if len(tokens) > int(postgresMaxRowsInChunk) {
				// splitting an array into chunks due to the postgres limit of 65535 parameters in a transaction
				tokens2chunks := models.Split2chunks(tokens, int(postgresMaxRowsInChunk))

				for i := range tokens2chunks {
					err = tx.Save(tokens2chunks[i]).Error
					if err != nil {
						tx.Rollback()
						log.Println(err)
					}
				}
			} else {
				err = tx.Save(tokens).Error
				if err != nil {
					tx.Rollback()
					log.Println(err)
				}
			}

			if err = tx.Commit().Error; err != nil {
				log.Println("database save error: ", err)
			}
		})
	}()
	<-forever
}
