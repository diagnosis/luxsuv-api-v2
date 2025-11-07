package main

import (
	"log"

	"github.com/diagnosis/luxsuv-api-v2/internal/secure"
)

func main() {
	s, _ := secure.HashPassword("Cola5418")
	log.Println(s)
}
