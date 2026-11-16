package main

import (
	"embed"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

// Nhúng toàn bộ thư mục public/ vào trong file binary khi biên dịch
//
//go:embed public/*
var embeddedWebFS embed.FS

var publicFS fs.FS

func init() {
	var err error
	publicFS, err = fs.Sub(embeddedWebFS, "public")
	if err != nil {
		log.Fatalf("Lỗi khởi tạo embedded static files: %v", err)
	}
}

// serveIndexHTML phục vụ file index.html cho Web UI
func serveIndexHTML(w http.ResponseWriter, r *http.Request) {
	data, err := fs.ReadFile(publicFS, "index.html")
	if err != nil {
		http.Error(w, "index.html not found in binary", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(data)
}

// getClientIP bóc tách địa chỉ IP phục vụ ghi log truy cập
func getClientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		return strings.TrimSpace(parts[0])
	}
	if xrip := r.Header.Get("X-Real-IP"); xrip != "" {
		return strings.TrimSpace(xrip)
	}
	addr := r.RemoteAddr
	if colon := strings.LastIndex(addr, ":"); colon != -1 {
		return addr[:colon]
	}
	return addr
}

// loggingMiddleware ghi log request
func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		if !strings.HasSuffix(r.URL.Path, "/events") && r.URL.Path != "/health" {
			log.Printf("[%s] %s %s - %v", r.Method, r.URL.Path, getClientIP(r), time.Since(start))
		}
	})
}

// corsMiddleware hỗ trợ gọi chéo domain
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-Filename, X-Live")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// roomDispatcher điều phối linh hoạt các request /{room} dựa vào hậu tố -s hoặc -r
func roomDispatcher(w http.ResponseWriter, r *http.Request) {
	room := r.PathValue("room")

	// Phục vụ file tĩnh nếu bị lọt
	if room == "style.css" || room == "app.js" {
		http.FileServerFS(publicFS).ServeHTTP(w, r)
		return
	}
	if room == "favicon.ico" {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	if strings.HasSuffix(room, "-s") {
		handleSender(w, r)
		return
	}
	if strings.HasSuffix(room, "-r") {
		handleReceiver(w, r)
		return
	}

	// Nếu gõ link không có hậu tố (ví dụ /1)
	if isCLI(r) {
		http.Redirect(w, r, fmt.Sprintf("/%s-r", room), http.StatusFound)
	} else {
		http.Redirect(w, r, fmt.Sprintf("/%s-s", room), http.StatusFound)
	}
}

func main() {
	// Dọn dẹp các phòng quá hạn (quét mỗi 5 phút, giải phóng phòng sau 2 giờ không hoạt động)
	globalRooms.StartTTLWorker(5*time.Minute, 2*time.Hour)

	mux := http.NewServeMux()

	// 1. Phục vụ tĩnh
	mux.Handle("GET /style.css", http.FileServerFS(publicFS))
	mux.Handle("GET /app.js", http.FileServerFS(publicFS))
	mux.HandleFunc("GET /favicon.ico", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	// 2. Health & Bootstrap Scripts
	mux.HandleFunc("GET /health", handleHealth)
	mux.HandleFunc("GET /i", handleInitScriptLinux)
	mux.HandleFunc("GET /i.ps1", handleInitScriptWindows)

	// 3. Quản lý trạng thái và SSE
	mux.HandleFunc("GET /{room}/events", handleRoomEvents)
	mux.HandleFunc("POST /{room}/reset-receiver", handleResetReceiver)

	// 4. Điều hướng thống nhất cho /{room} (bao gồm cả *-s và *-r)
	mux.HandleFunc("GET /{room}", roomDispatcher)
	mux.HandleFunc("POST /{room}", roomDispatcher)

	// 5. Điều hướng mặc định cho trang chủ root /
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		if isCLI(r) {
			http.Redirect(w, r, "/1-r", http.StatusFound)
		} else {
			http.Redirect(w, r, "/1-s", http.StatusFound)
		}
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	addr := ":" + port

	handler := corsMiddleware(loggingMiddleware(mux))

	fmt.Println("================================================================")
	fmt.Println("  ⚡ WireDrop v4.0.0 (Pure Physical Connection Lock) is running!")
	fmt.Printf("  • Master Sender UI:    http://localhost:%s/1-s\n", port)
	fmt.Printf("  • Worker Receiver CLI: curl -sL http://localhost:%s/1-r | sh\n", port)
	fmt.Printf("  • Windows Receiver:    irm http://localhost:%s/1-r | iex\n", port)
	fmt.Println("  • 100% Go Standard Library • Zero-Dependency")
	fmt.Println("================================================================")

	if err := http.ListenAndServe(addr, handler); err != nil {
		log.Fatalf("Server sập: %v", err)
	}
}
