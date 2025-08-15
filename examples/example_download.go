//go:build ignore
// +build ignore

package main

import (
	"fmt"
	"log"

	"github.com/amikos-tech/pure-tokenizers"
)

func main() {
	fmt.Println("CGo-free Tokenizers - Download Example")
	fmt.Println("=====================================")

	// Example 1: Check cache path
	cachePath := tokenizers.GetCachedLibraryPath()
	fmt.Printf("Cache path: %s\n", cachePath)

	// Example 2: Download and cache library explicitly
	fmt.Println("\nDownloading library...")
	if err := tokenizers.DownloadAndCacheLibrary(); err != nil {
		log.Printf("Download failed: %v", err)
		fmt.Println("Note: This is expected if no releases are available yet")
	} else {
		fmt.Println("Library downloaded and cached successfully!")
	}

	// Example 3: Create tokenizer with automatic download
	fmt.Println("\nCreating tokenizer with automatic download...")
	tokenizer, err := tokenizers.FromFile("tokenizer.json",
		tokenizers.WithDownloadLibrary())
	if err != nil {
		log.Printf("Failed to create tokenizer: %v", err)
		fmt.Println("Note: This requires a valid tokenizer.json file and available releases")
		return
	}
	defer tokenizer.Close()

	fmt.Println("Tokenizer created successfully!")
}
