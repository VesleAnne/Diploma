package models

import (
	"gorm.io/gorm"
)

type Result struct {
	gorm.Model
	SessionID string   `json:"sessionId" gorm:"unique;column:sid"`
	Message   string   `json:"message" gorm:"column:message"`
	Data      []Tokens `json:"data" gorm:"-"`
	Tokens    []Tokens `json:"tokens" gorm:"-"`
}

type Tokens struct {
	Id         int    `json:"-" gorm:"primary key;autoIncrement"`
	SessionID  string `json:"-" gorm:"column:sid"`
	TokenID    int    `json:"id"`
	Error      int   `json:"error"`
	Question   string `json:"Question"`
	Answer     string `json:"Answer"`
	Score      float32    `json:"Score"`
	Operator    int  `json:"OperatorFlag"`
}

type Response struct {
	SessionID string   `json:"sessionId"`
	Message   string   `json:"message"`
	Tokens    []Tokens `json:"tokens"`
}

type Data struct {
	Id        int    `json:"id"`
	IncString string `json:"string"`
}

type Data2Ner struct {
	SessionID string `json:"sessionId"`
	Tokens    []Data `json:"data"`
}

// input: tokens array, size of chunks
// output: [][]models.Tokens splitted into chunks by chunkSize
func Split2chunks(tokens []Tokens, chunkSize int) [][]Tokens {
	var divided [][]Tokens

	for i := 0; i < len(tokens); i += chunkSize {
		end := i + chunkSize
		if end > len(tokens) {
			end = len(tokens)
		}
		divided = append(divided, tokens[i:end])
	}
	return divided
}
