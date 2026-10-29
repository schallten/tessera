# tessera

OCR screenshot indexer. Scans `~/Pictures/Screenshots` for `.jpg`/`.jpeg`/`.png` images, runs Tesseract OCR on new files, stores text and SHA-256 hashes in SQLite (`index.db`), and provides a Fyne GUI to search indexed screenshot content.

Dependencies: `tesseract`, `tesseract-data-eng`, Go, SQLite.
