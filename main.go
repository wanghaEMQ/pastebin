package main

import (
	"fmt"
	"html/template"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

const (
	uploadDir  = "./uploads/"
	baseURL    = "http://ServerIP:9000/"
	urlLength  = 8
)

func main() {
	rand.Seed(time.Now().UnixNano())

	// Ensure upload directory exists
	err := os.MkdirAll(uploadDir, os.ModePerm)
	if err != nil {
		log.Fatalf("Could not create upload directory: %v", err)
	}

	http.HandleFunc("/", handleIndex)
	http.HandleFunc("/upload", handleUpload)
	http.HandleFunc("/view/", handleView)
	http.Handle("/uploads/", http.StripPrefix("/uploads/", http.FileServer(http.Dir(uploadDir))))

	fmt.Println("Server started at :9000")
	log.Fatal(http.ListenAndServe(":9000", nil))
}

func handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	tmpl := template.Must(template.ParseFiles("./templates/index.html"))
	tmpl.Execute(w, nil)
}

func handleUpload(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodPost {
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
        return
    }

    // Parse form data
    err := r.ParseMultipartForm(10 << 20) // 10 MB limit
    if err != nil {
        http.Error(w, "Unable to parse form", http.StatusBadRequest)
        return
    }

    // Get file or text input
    file, handler, err := r.FormFile("file")
    if err == nil {
        defer file.Close()

        // Generate a filename with the original name and a random suffix
        originalName := handler.Filename
		currentTime := time.Now().Format("20060102_150405")
		filename := fmt.Sprintf("%s_%s%s", originalName[:len(originalName)-len(filepath.Ext(originalName))], currentTime, filepath.Ext(originalName))

        destPath := filepath.Join(uploadDir, filename)
        destFile, err := os.Create(destPath)
        if err != nil {
            http.Error(w, "Could not save file", http.StatusInternalServerError)
            return
        }
        defer destFile.Close()

        io.Copy(destFile, file)

        // Return URL for the uploaded file
        fmt.Fprintf(w, "File uploaded successfully: %sview/%s\n", baseURL, filename)
        return
    }

    // Handle text upload
    text := r.FormValue("text")
    if text != "" {
		currentTime := time.Now().Format("20060102_150405")
		filename := fmt.Sprintf("%s_%s%s", generateID(), currentTime, ".txt")
        destPath := filepath.Join(uploadDir, filename)
        err = os.WriteFile(destPath, []byte(text), 0644)
        if err != nil {
            http.Error(w, "Could not save text", http.StatusInternalServerError)
            return
        }

        // Return URL for the uploaded text
        fmt.Fprintf(w, "Text uploaded successfully: %sview/%s\n", baseURL, filename)
        return
    }

    http.Error(w, "No file or text provided", http.StatusBadRequest)
}

func handleView(w http.ResponseWriter, r *http.Request) {
	filename := filepath.Base(r.URL.Path)
	filePath := filepath.Join(uploadDir, filename)

	// Serve the file
	http.ServeFile(w, r, filePath)
}

func generateID() string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, urlLength)
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}
	return string(b)
}

