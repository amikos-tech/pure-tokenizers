package main

import (
	"log"

	tokenizers "github.com/amikos-tech/pure-tokenizers"
)

func main() {
	tokenizer, err := tokenizers.FromFile("./tokenizer.json")
	if err != nil {
		log.Fatal(err)
	}
	defer tokenizer.Close()

	res, err := tokenizer.Encode("Hello, world!")
	if err != nil {
		log.Fatal(err)
	}
	log.Println("Tokens:", res.Tokens)
}
