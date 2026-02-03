package main

import (
    "encoding/json"
    "fmt"
    "html/template"
    "io"
    "log"
    "math/rand"
    "net/http"
    "os"
    "path/filepath"
    "strings"
    "sync"
    "time"
)

const (
    uploadDir = "./uploads/"
    baseURL   = "http://localhost:9000/"
    urlLength = 8
)

// Ticket 结构体存储提交的文本信息
type Ticket struct {
    ID        string    `json:"id"`
    Preview   string    `json:"preview"`
    Content   string    `json:"content"`
    Filename  string    `json:"filename"`
    Timestamp time.Time `json:"timestamp"`
    URL       string    `json:"url"`
}

// 全局变量存储最近提交的tickets
var (
    recentTickets []Ticket
    maxTickets    = 10
    ticketMutex   sync.RWMutex
)

func main() {
    rand.Seed(time.Now().UnixNano())

    // 确保上传目录存在
    err := os.MkdirAll(uploadDir, os.ModePerm)
    if err != nil {
        log.Fatalf("Could not create upload directory: %v", err)
    }

    // 初始化recentTickets
    recentTickets = make([]Ticket, 0, maxTickets)

    http.HandleFunc("/", handleIndex)
    http.HandleFunc("/upload", handleUpload)
    http.HandleFunc("/view/", handleView)
    http.HandleFunc("/api/tickets", handleGetTickets) // 新增API端点
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

    // 解析表单数据
    err := r.ParseMultipartForm(10 << 20) // 10 MB限制
    if err != nil {
        http.Error(w, "Unable to parse form", http.StatusBadRequest)
        return
    }

    // 获取文件或文本输入
    file, handler, err := r.FormFile("file")
    if err == nil {
        defer file.Close()

        // 生成文件名
        originalName := handler.Filename
        currentTime := time.Now().Format("20060102_150405")
        filename := fmt.Sprintf("%s_%s", currentTime, originalName)

        destPath := filepath.Join(uploadDir, filename)
        destFile, err := os.Create(destPath)
        if err != nil {
            http.Error(w, "Could not save file", http.StatusInternalServerError)
            return
        }
        defer destFile.Close()

        io.Copy(destFile, file)

        // 返回JSON响应
        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(map[string]interface{}{
            "success":  true,
            "message":  "File uploaded successfully",
            "url":      fmt.Sprintf("%sview/%s", baseURL, filename),
            "filename": filename,
        })
        return
    }

    // 处理文本上传
    text := r.FormValue("text")
    if text != "" {
        currentTime := time.Now().Format("20060102_150405")
        fileID := generateID()
        filename := fmt.Sprintf("%s_%s%s", currentTime, fileID, ".txt")
        destPath := filepath.Join(uploadDir, filename)
        err = os.WriteFile(destPath, []byte(text), 0644)
        if err != nil {
            http.Error(w, "Could not save text", http.StatusInternalServerError)
            return
        }

        // 创建ticket
        ticket := Ticket{
            ID:        fileID,
            Preview:   generatePreview(text, 100),
            Content:   text,
            Filename:  filename,
            Timestamp: time.Now(),
            URL:       fmt.Sprintf("%sview/%s", baseURL, filename),
        }

        // 添加到最近tickets列表
        ticketMutex.Lock()
        recentTickets = append([]Ticket{ticket}, recentTickets...)
        if len(recentTickets) > maxTickets {
            recentTickets = recentTickets[:maxTickets]
        }
        ticketMutex.Unlock()
		log.Println("new ticket", ticket)

        // 返回JSON响应
        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(map[string]interface{}{
            "success":  true,
            "message":  "Text uploaded successfully",
            "url":      ticket.URL,
            "filename": filename,
            "ticket":   ticket,
        })
        return
    }

    http.Error(w, "No file or text provided", http.StatusBadRequest)
}

func handleView(w http.ResponseWriter, r *http.Request) {
    filename := filepath.Base(r.URL.Path)
    filePath := filepath.Join(uploadDir, filename)

    // 检查文件是否存在
    if _, err := os.Stat(filePath); os.IsNotExist(err) {
        http.Error(w, "File not found", http.StatusNotFound)
        return
    }

    // 如果是文本文件，设置正确的Content-Type
    if strings.HasSuffix(filename, ".txt") {
        w.Header().Set("Content-Type", "text/plain; charset=utf-8")
    }

    // 提供文件
    http.ServeFile(w, r, filePath)
}

// 新增：获取最近tickets的API端点
func handleGetTickets(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodGet {
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
        return
    }

    ticketMutex.RLock()
    defer ticketMutex.RUnlock()

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(recentTickets)
}

func generateID() string {
    const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
    b := make([]byte, urlLength)
    for i := range b {
        b[i] = charset[rand.Intn(len(charset))]
    }
    return string(b)
}

// 生成文本预览
func generatePreview(text string, maxLength int) string {
    if len(text) <= maxLength {
        return text
    }
    // 确保在单词边界处截断
    preview := text[:maxLength]
    // 查找最后一个空格
    if lastSpace := strings.LastIndex(preview, " "); lastSpace > maxLength/2 {
        preview = preview[:lastSpace]
    }
    return preview + "..."
}
