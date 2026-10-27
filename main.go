package main

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	_ "github.com/mattn/go-sqlite3"
	"github.com/otiai10/gosseract/v2"
	"io"
	"log"
	"os"
	"path/filepath"
	_ "slices"
	"strings"
	"time"
)

type dbEntry struct {
	filename   string
	fileHash   string
	content    string
	checked_at time.Time
}

// $ pacman -Sy tesseract and then sudo pacman -S tesseract-data-eng
var db *sql.DB

//var ScreenshotPath = os.Getenv("XDG_PICTURES_DIR") + "/Screenshots"

func initDB() error {
	var err error
	db, err = sql.Open("sqlite3", "./index.db")
	if err != nil {
		log.Fatal(err)
	}
	// defer db.Close()
	db.SetMaxOpenConns(1)

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

func putInDB(entry dbEntry) error {
	query := `INSERT INTO "index" (filename, fileHash, content, checked_at) VALUES (?, ?, ?, ?)`

	//currentTime := time.Now().Format(time.RFC3339)

	_, err := db.Exec(query, entry.filename, entry.fileHash, entry.content, entry.checked_at)
	if err != nil {
		log.Printf("Insert failed: %v\n", err)
		return err
	}

	return nil
}

func searchDB(term string) error {
	rows, err := db.Query(
		`SELECT filename FROM "index" WHERE content LIKE ?`, "%"+term+"%",
	)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var filename string
		if err := rows.Scan(&filename); err != nil {
			return err
		}
		fmt.Println(filename)
		if filename != "" {
			return err
		}
	}
	return rows.Err()
}

func hashFile(filename string) (string, error) {
	hasher := sha256.New()
	file, err := os.Open(filename)
	if err != nil {
		fmt.Println(err)
		return "", errors.New("could not open file for hashing")
	}
	defer file.Close()
	if _, err := io.Copy(hasher, file); err != nil {
		fmt.Println(err)
		return "", errors.New("could not copy into hasher")
	}
	hashBytes := hasher.Sum(nil)
	hashString := hex.EncodeToString(hashBytes)

	return hashString, nil

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
	var dbEntries = make([]dbEntry, i)

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
			//dbEntries[i].id = i
			dbEntries[i].filename = path
			dbEntries[i].fileHash, err = hashFile(path)
			if err != nil {
				fmt.Println(err)
				return
			}
			dbEntries[i].content = text
			dbEntries[i].checked_at = time.Now()
			putInDB(dbEntries[i])
		}
		fmt.Println("=====================\n")
	}

	client.SetImage("assets/test.png")
	text, _ := client.Text()
	fmt.Println(text)
	// Hello, World!
	var term string
	fmt.Print("Search : ")
	fmt.Scanln(&term)
	err = searchDB(term)
	if err != nil {
		log.Println(err)
	}
	defer db.Close()
}
