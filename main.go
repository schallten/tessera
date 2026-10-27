package main

import (
	"database/sql"
	"errors"
	"fmt"
	_ "github.com/mattn/go-sqlite3"
	"github.com/otiai10/gosseract/v2"
	"log"
	"os"
	"path/filepath"
	_ "slices"
	"strings"
)

// $ pacman -Sy tesseract and then sudo pacman -S tesseract-data-eng

//var ScreenshotPath = os.Getenv("XDG_PICTURES_DIR") + "/Screenshots"

func initDB() error {
	db, err := sql.Open("sqlite3", "./index.db")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	tableAndCols := `CREATE TABLE IF NOT EXISTS "index" 
	(
		id integer not null primary key,
		filename text not null,
		fileHash varchar(64) not null,
		content text not null,
		checked_at text not null
	);
	`

	_, err = db.Exec(tableAndCols)
	if err != nil {
		log.Printf("%q: %s\n", err, tableAndCols)
		return errors.New("lol")
	}
	return nil
}

func main() {
	home, err := os.UserHomeDir()
	if err != nil {
		log.Fatal(err)
	}
	var ScreenshotPath = home + "/Pictures/Screenshots"
	entries, err := os.ReadDir(ScreenshotPath)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("\n Found these images \n")

	var i = 0

	for _, entry := range entries {
		name := strings.ToLower(entry.Name())

		if strings.HasSuffix(name, ".jpeg") ||
			strings.HasSuffix(name, ".png") ||
			strings.HasSuffix(name, ".jpg") {

			i = i + 1
		}
	}
	fmt.Printf("\n found %d images ", i)

	// slice of files
	var fileNames = make([]string, i)
	fmt.Println("\nemp:", len(fileNames))
	var k = 0
	for _, entry := range entries {
		name := strings.ToLower(entry.Name())

		if strings.HasSuffix(name, ".jpeg") ||
			strings.HasSuffix(name, ".png") ||
			strings.HasSuffix(name, ".jpg") {

			fmt.Printf("Name : %s\n", entry.Name())
			fileNames[k] = entry.Name()
			k = k + 1
		}
	}

	fmt.Println("\n initializing database \n")
	err = initDB()
	if err != nil {
		return
	}
	fmt.Println("\n done \n")

	client := gosseract.NewClient()
	defer client.Close()
	fmt.Println("\n")

	for i := 0; i < len(fileNames); i++ {
		path := filepath.Join(ScreenshotPath, fileNames[i])

		fmt.Println("\n===== Testing", path, "=====\n")

		if err := client.SetImage(path); err != nil {
			log.Printf("Failed to set image: %v", err)
			continue
		}

		text, err := client.Text()
		if err != nil {
			log.Printf("OCR failed: %v", err)
			continue
		}
		if text == "" {
			fmt.Println("\n No text info found / Could not parse\n")
		} else {
			fmt.Println(text)
		}
		fmt.Println("=====================\n")
	}

	client.SetImage("assets/test.png")
	text, _ := client.Text()
	fmt.Println(text)
	// Hello, World!
}
