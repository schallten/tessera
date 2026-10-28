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
var client *gosseract.Client

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
		fileHash varchar(64) not null unique,
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
	query := `INSERT OR IGNORE INTO "index" (filename, fileHash, content, checked_at) VALUES (?, ?, ?, ?)`

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
		`SELECT id, filename, fileHash
         FROM "index"
         WHERE content LIKE ?`,
		"%"+term+"%",
	)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var id int
		var filename, fileHash string

		if err := rows.Scan(&id, &filename, &fileHash); err != nil {
			return err
		}

		fmt.Println(id, filename, fileHash)
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

func alreadyScanned(hash string) (bool, error) {
	var exists bool

	err := db.QueryRow(
		`SELECT EXISTS(
            SELECT 1 FROM "index" WHERE fileHash = ?
        )`,
		hash,
	).Scan(&exists)

	return exists, err
}

type timings struct {
	hash   time.Duration
	db     time.Duration
	ocr    time.Duration
	insert time.Duration
}

func isImage(name string) bool {
	name = strings.ToLower(name)
	return strings.HasSuffix(name, ".jpeg") ||
		strings.HasSuffix(name, ".png") ||
		strings.HasSuffix(name, ".jpg")
}

func listImages(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var fileNames []string
	for _, entry := range entries {
		if isImage(entry.Name()) {
			fileNames = append(fileNames, entry.Name())
		}
	}

	return fileNames, nil
}

func processImage(path string, client *gosseract.Client) timings {
	var t timings

	start := time.Now()
	fileHash, err := hashFile(path)
	t.hash = time.Since(start)
	if err != nil {
		log.Printf("Hashing failed: %v", err)
		return t
	}

	start = time.Now()
	exists, err := alreadyScanned(fileHash)
	t.db = time.Since(start)
	if err != nil {
		log.Printf("Database lookup failed: %v", err)
		return t
	}

	if exists {
		log.Printf("Skipping already scanned: %s", filepath.Base(path))
		return t
	}

	start = time.Now()
	if err := client.SetImage(path); err != nil {
		t.ocr = time.Since(start)
		log.Printf("Failed to set image: %v", err)
		return t
	}

	text, err := client.Text()
	t.ocr = time.Since(start)
	if err != nil {
		log.Printf("OCR failed: %v", err)
		return t
	}

	entry := dbEntry{
		filename:   path,
		fileHash:   fileHash,
		content:    text,
		checked_at: time.Now(),
	}

	start = time.Now()
	err = putInDB(entry)
	t.insert = time.Since(start)
	if err != nil {
		log.Printf("Database insert failed: %v", err)
	}

	return t
}

func refreshIndex(screenshotPath string, client *gosseract.Client) {
	fileNames, err := listImages(screenshotPath)
	if err != nil {
		log.Fatal(err)
	}

	for _, name := range fileNames {
		processImage(filepath.Join(screenshotPath, name), client)
	}
}

func startPeriodicRefresh(screenshotPath string, client *gosseract.Client) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		log.Println("Refreshing screenshot index...")
		refreshIndex(screenshotPath, client)
	}
}

func main() {
	var hashTime, dbTime, ocrTime, insertTime time.Duration
	home, err := os.UserHomeDir()
	if err != nil {
		log.Fatal(err)
	}
	var ScreenshotPath = home + "/Pictures/Screenshots"
	fileNames, err := listImages(ScreenshotPath)
	if err != nil {
		log.Fatal(err)
	}

	//	fmt.Println("\n initializing database \n")
	err = initDB()

	client = gosseract.NewClient()
	go startPeriodicRefresh(ScreenshotPath, client)

	if err != nil {
		return
	}
	//	fmt.Println("\n done \n")

	//	fmt.Println("\n")

	for _, name := range fileNames {
		t := processImage(filepath.Join(ScreenshotPath, name), client)
		hashTime += t.hash
		dbTime += t.db
		ocrTime += t.ocr
		insertTime += t.insert
	}

	client.SetImage("assets/test.png")
	//	text, _ := client.Text()
	//	fmt.Println(text)
	// Hello, World!
	var term string
	fmt.Printf("hash: %v | db: %v | ocr: %v | insert: %v\n", hashTime, dbTime, ocrTime, insertTime)
	fmt.Print("Search : ")
	fmt.Scanln(&term)
	err = searchDB(term)
	if err != nil {
		log.Println(err)
	}
	defer client.Close()
	defer db.Close()
}
