package main

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// --- 1. Data Structures (The "Good Taste" part) ---
type Post struct {
	Slug        string    `json:"slug"`        // The SEO link: /post/my-first-post
	Title       string    `json:"title"`       // Browser Tab Title
	Description string    `json:"description"` // Meta Description for SEO
	Content     string    `json:"content"`     // The HTML/Markdown body
	PublishedAt time.Time `json:"published_at"`
}

// --- 2. The Store (Keep it boring) ---
var db *sql.DB

func initDB() {
	var err error

	// just create a single db file malt.db
	db, err := sql.Open("sqlite", "malt.db")
	if err != nil {
		log.Fatal(err)
	}

	query := `
	CREATE TABLE IF NOT EXISTS posts (
		slug TEXT PRIMARY KEY,
		title TEXT,
		description TEXT,
		content TEXT,
		published_at DATETIME
	);`

	if _, err := db.Exec(query); err != nil {
		log.Fatal(err)
	}
}

// --- 3. Handlers (Minimal logic) ---

// GET /api/posts - Returns list for the homepage
func handleListPosts(w http.ResponseWriter, r *http.Request) {
	rows, err := db.Query("SELECT slug, title, description, published_at FROM posts ORDER BY published_at DESC")
	if err != nil {
		http.Error(w, "Database error", 500)
		return
	}
	defer rows.Close()

	var posts []Post
	for rows.Next() {
		var p Post
		// Note: We don't fetch 'Content' here to keep the list payload tiny
		if err := rows.Scan(&p.Slug, &p.Title, &p.Description, &p.PublishedAt); err != nil {
			continue
		}
		posts = append(posts, p)
	}

	jsonResponse(w, posts)
}

// GET /api/posts/{slug} - Returns single post for rendering
func handleGetPost(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug") // Go 1.22 feature

	var p Post
	row := db.QueryRow("SELECT slug, title, description, content, published_at FROM posts WHERE slug = ?", slug)
	if err := row.Scan(&p.Slug, &p.Title, &p.Description, &p.Content, &p.PublishedAt); err != nil {
		http.Error(w, "Post not found", 404)
		return
	}

	jsonResponse(w, p)
}

// POST /api/publish - The protected push endpoint
func handlePublish(w http.ResponseWriter, r *http.Request) {
	// "Torvalds" Auth: Simple, fast, secure enough for personal use.
	if r.Header.Get("X-MALT-KEY") != os.Getenv("MALT_SECRET") {
		http.Error(w, "Go away", 401)
		return
	}

	var p Post
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		http.Error(w, "Bad JSON", 400)
		return
	}

	// Auto-generate Slug from Title if missing (Simple SEO logic)
	if p.Slug == "" {
		p.Slug = strings.ToLower(strings.ReplaceAll(p.Title, " ", "-"))
	}
	p.PublishedAt = time.Now()

	_, err := db.Exec(`
		INSERT INTO posts (slug, title, description, content, published_at) 
		VALUES (?, ?, ?, ?, ?) 
		ON CONFLICT(slug) DO UPDATE SET 
			title=excluded.title, 
			content=excluded.content, 
			description=excluded.description
	`, p.Slug, p.Title, p.Description, p.Content, p.PublishedAt)

	if err != nil {
		http.Error(w, "Failed to save: "+err.Error(), 500)
		return
	}

	jsonResponse(w, map[string]string{"status": "published", "link": "/post/" + p.Slug})
}

// Helper for JSON
func jsonResponse(w http.ResponseWriter, data any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

// --- 4. The Core ---
func main() {
	initDB()
	defer db.Close()

	mux := http.NewServeMux()

	// 1. API Routes
	mux.HandleFunc("GET /api/posts", handleListPosts)
	mux.HandleFunc("GET /api/posts/{slug}", handleGetPost)
	mux.HandleFunc("POST /api/publish", handlePublish)

	// 2. Serve Frontend (SPA Catch-all)
	// This serves index.html for any route that doesn't match above (e.g., /post/my-slug)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Serve index.html directly
		http.ServeFile(w, r, "index.html")
	})

	log.Println("Malt running on :8080")
	server := &http.Server{
		Addr:         ":8080",
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
	}
	log.Fatal(server.ListenAndServe())
}
