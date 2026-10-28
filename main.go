package main

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	_ "github.com/mattn/go-sqlite3"
	"github.com/otiai10/gosseract/v2"
)

type dbEntry struct {
	filename   string
	fileHash   string
	content    string
	checked_at time.Time
}

type searchHit struct {
	id       int
	filename string
	fileHash string
}

// $ pacman -Sy tesseract and then sudo pacman -S tesseract-data-eng
var db *sql.DB
var client *gosseract.Client

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

	_, err := db.Exec(query, entry.filename, entry.fileHash, entry.content, entry.checked_at)
	if err != nil {
		log.Printf("Insert failed: %v\n", err)
		return err
	}

	return nil
}

func searchDB(term string) ([]searchHit, error) {
	rows, err := db.Query(
		`SELECT id, filename, fileHash
         FROM "index"
         WHERE content LIKE ?`,
		"%"+term+"%",
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var hits []searchHit
	for rows.Next() {
		var hit searchHit
		if err := rows.Scan(&hit.id, &hit.filename, &hit.fileHash); err != nil {
			return nil, err
		}
		hits = append(hits, hit)
	}

	return hits, rows.Err()
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
		log.Printf("Could not read %s: %v", screenshotPath, err)
		return
	}

	for _, name := range fileNames {
		processImage(filepath.Join(screenshotPath, name), client)
	}
}

func main() {
	home, err := os.UserHomeDir()
	if err != nil {
		log.Fatal(err)
	}
	screenshotPath := filepath.Join(home, "Pictures", "Screenshots")

	if err := initDB(); err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	log.SetOutput(io.Discard)

	a := app.New()
	w := a.NewWindow("Tessera")
	w.Resize(fyne.NewSize(760, 500))

	status := widget.NewLabel("Indexing screenshots...")
	results := container.NewVBox()
	resultsScroll := container.NewVScroll(results)

	search := widget.NewEntry()
	search.SetPlaceHolder("Search screenshot text...")

	doSearch := func(term string) {
		results.RemoveAll()
		term = strings.TrimSpace(term)
		if term == "" {
			status.SetText("Type something to search.")
			results.Refresh()
			return
		}

		hits, err := searchDB(term)
		if err != nil {
			status.SetText("Search failed: " + err.Error())
			return
		}

		if len(hits) == 0 {
			results.Add(widget.NewLabel("No matches."))
		}
		for _, hit := range hits {
			results.Add(widget.NewLabel(fmt.Sprintf("%s  (%s)", hit.filename, hit.fileHash[:8])))
		}
		status.SetText(fmt.Sprintf("%d result(s) for %q", len(hits), term))
		results.Refresh()
	}

	search.OnSubmitted = doSearch
	searchBtn := widget.NewButton("Search", func() {
		doSearch(search.Text)
	})

	setStatus := func(msg string) {
		fyne.Do(func() { status.SetText(msg) })
	}

	client = gosseract.NewClient()
	go func() {
		defer client.Close()
		refreshIndex(screenshotPath, client)
		setStatus("Index up to date.")

		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			setStatus("Refreshing screenshot index...")
			refreshIndex(screenshotPath, client)
			setStatus("Index up to date.")
		}
	}()

	w.SetContent(container.NewBorder(
		container.NewBorder(nil, nil, nil, searchBtn, search),
		status,
		nil, nil,
		resultsScroll,
	))
	w.ShowAndRun()
}
