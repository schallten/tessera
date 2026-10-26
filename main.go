package main

import (
	tea "charm.land/bubbletea/v2"
	"database/sql"
	"fmt"
	_ "github.com/mattn/go-sqlite3"
	"github.com/otiai10/gosseract/v2"
)

// $ pacman -Sy tesseract and then sudo pacman -S tesseract-data-eng

func main() {
	client := gosseract.NewClient()
	defer client.Close()
	client.SetImage("assets/test.png")
	text, _ := client.Text()
	fmt.Println(text)
	// Hello, World!
}
