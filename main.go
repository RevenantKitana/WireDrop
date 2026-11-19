package main

import (
	"crypto/rand"
	"embed"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const Version = "v0.8.3"
const defaultSlugChars = "abcdefghijklmnopqrstuvwxyz0123456789"

func randomSlug(n int) string {
	b := make([]byte, n)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		return "wire"
	}
	for i := range b {
		b[i] = defaultSlugChars[int(b[i])%len(defaultSlugChars)]
	}
	return string(b)
}

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

// serveIndexHTML phục vụ file index.html cho Web UI với no-cache header
func serveIndexHTML(w http.ResponseWriter, r *http.Request) {
	data, err := fs.ReadFile(publicFS, "index.html")
	if err != nil {
		http.Error(w, "index.html not found in binary", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")
	_, _ = w.Write(data)
}

// serveStyleCSS phục vụ style.css với Content-Type chuẩn text/css và no-cache header (chống lỗi Render MIME/cache)
func serveStyleCSS(w http.ResponseWriter, r *http.Request) {
	data, err := fs.ReadFile(publicFS, "style.css")
	if err != nil {
		http.Error(w, "style.css not found in binary", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "text/css; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")
	_, _ = w.Write(data)
}

// serveAppJS phục vụ app.js với Content-Type chuẩn javascript và no-cache header
func serveAppJS(w http.ResponseWriter, r *http.Request) {
	data, err := fs.ReadFile(publicFS, "app.js")
	if err != nil {
		http.Error(w, "app.js not found in binary", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")
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
		if !strings.HasSuffix(r.URL.Path, "/events") && !strings.HasSuffix(r.URL.Path, "/chunk") && r.URL.Path != "/health" {
			log.Printf("[%s] %s %s - %v", r.Method, r.URL.Path, getClientIP(r), time.Since(start))
		}
	})
}

// corsMiddleware hỗ trợ gọi chéo domain
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-Filename, X-Live, X-Session-ID, X-File-Name, X-File-Size, X-Chunk-Index, X-Total-Chunks, X-Chunk-Size, X-Byte-Offset, X-Chunk-Checksum")
		w.Header().Set("Access-Control-Expose-Headers", "Content-Length, Content-Disposition, X-File-Size, X-Total-Chunks, X-Chunk-Index, X-Chunk-Size")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// roomDispatcher điều phối linh hoạt các request /{room} dựa vào hậu tố -s hoặc -r
func roomDispatcher(w http.ResponseWriter, r *http.Request) {
	room := strings.ToLower(r.PathValue("room"))

	// Phục vụ file tĩnh nếu bị lọt
	if room == "index.html" {
		serveIndexHTML(w, r)
		return
	}
	if room == "style.css" {
		serveStyleCSS(w, r)
		return
	}
	if room == "app.js" {
		serveAppJS(w, r)
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

// createHandler khởi tạo ServeMux và middleware của WireDrop
func createHandler() http.Handler {
	mux := http.NewServeMux()

	// 1. Phục vụ tĩnh với Content-Type chuẩn và No-Cache Header
	mux.HandleFunc("GET /style.css", serveStyleCSS)
	mux.HandleFunc("GET /app.js", serveAppJS)
	mux.HandleFunc("GET /favicon.ico", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	// 2. Health & API
	mux.HandleFunc("GET /health", handleHealth)
	mux.HandleFunc("POST /api/shorten", handleAPIShorten)
	mux.HandleFunc("POST /{room}/ack", handleRoomAck)

	// 3. Quản lý trạng thái và SSE
	mux.HandleFunc("GET /{room}/events", handleRoomEvents)
	mux.HandleFunc("POST /{room}/reset-receiver", handleResetReceiver)
	mux.HandleFunc("POST /{room}/chunk", handleChunkUpload)
	mux.HandleFunc("POST /{room}/abort", handleChunkAbort)

	// 4. Điều hướng thống nhất cho /{room} (bao gồm cả *-s và *-r)
	mux.HandleFunc("GET /{room}", roomDispatcher)
	mux.HandleFunc("POST /{room}", roomDispatcher)

	// 5. Điều hướng mặc định cho trang chủ root /
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		if isCLI(r) {
			http.Redirect(w, r, "/"+randomSlug(4)+"-r", http.StatusFound)
		} else {
			serveIndexHTML(w, r)
		}
	})

	return corsMiddleware(loggingMiddleware(mux))
}

// runServer khởi tạo HTTP Server và in banner nếu cần
func runServer(port string, printBanner bool) (*http.Server, error) {
	globalRooms.StartTTLWorker(5*time.Minute, 2*time.Hour)

	if port == "" {
		port = "8080"
	}
	addr := ":" + port
	handler := createHandler()
	server := &http.Server{
		Addr:    addr,
		Handler: handler,
	}

	if printBanner {
		sampleSlug := randomSlug(4)
		fmt.Println("================================================================")
		fmt.Printf("  ⚡ WireDrop %s (Pure Physical Connection Lock) is running!\n", Version)
		fmt.Printf("  • Web Hub (Trang chu #):    http://localhost:%s/#\n", port)
		fmt.Println("    (Mo link tren de xem Cheatsheet hoac tu tao phong tuy y)")
		fmt.Println("  ----------------------------------------------------------------")
		fmt.Printf("  • Vi du truyen nhanh (Phong mau ngau nhien: %s):\n", sampleSlug)
		fmt.Printf("    - Web phat (TX):          http://localhost:%s/%s-s\n", port, sampleSlug)
		fmt.Printf("    - Lenh nhan (RX Linux):   curl -sL http://localhost:%s/%s-r | sh\n", port, sampleSlug)
		fmt.Printf("    - Lenh nhan (RX Windows): irm http://localhost:%s/%s-r | iex\n", port, sampleSlug)
		fmt.Printf("    - Tai File (RX):          curl -sLJO http://localhost:%s/%s-r\n", port, sampleSlug)
		fmt.Println("  • 100% Go Standard Library • Zero-Dependency")
		fmt.Println("================================================================")
	}

	return server, nil
}

func main() {
	args := os.Args[1:]

	if len(args) > 0 {
		first := strings.ToLower(args[0])
		switch first {
		case "host", "--host":
			runPortableHost()
			return
		case "server", "--server":
			// Chạy server thông thường với flag
			runDedicatedServer(args[1:])
			return
		case "install", "--install":
			runInstallCommand()
			return
		case "uninstall", "--uninstall":
			runUninstallCommand()
			return
		case "-h", "--help", "-help":
			printTerminalPipeHelp()
			return
		default:
			// Nếu không phải cờ '--port' của server thì coi là lệnh Terminal Pipe
			// Ví dụ: 'wiredrop 88', 'wiredrop 88 -r', 'wiredrop 88 -s', 'wiredrop -r 88'
			if !strings.HasPrefix(first, "-port") && !strings.HasPrefix(first, "--port") {
				runTerminalPipeCommand(args)
				return
			}
		}
	}

	// Khi nhấp đúp chuột hoặc chạy không truyền tham số: Mở Interactive Launcher (FastHost / Local)
	if len(args) == 0 {
		runPortableHost()
		return
	}

	runDedicatedServer(args)
}

// runDedicatedServer chạy HTTP Server nền tảng
func runDedicatedServer(args []string) {
	fs := flag.NewFlagSet("server", flag.ExitOnError)
	portFlag := fs.String("port", "", "Cong lang nghe HTTP (vi du: 8080)")
	_ = fs.Parse(args)

	port := *portFlag
	if port == "" {
		port = os.Getenv("PORT")
	}
	if port == "" {
		port = "8080"
	}

	srv, _ := runServer(port, true)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Server sập: %v", err)
	}
}

// runInstallCommand cài đặt wiredrop vào thư mục User PATH toàn cục
func runInstallCommand() {
	exePath, err := os.Executable()
	if err != nil {
		log.Fatalf("[X] Không thể xác định đường dẫn file thực thi: %v", err)
	}
	exePath, _ = filepath.EvalSymlinks(exePath)

	var targetDir, targetExe string
	isWindows := os.Getenv("OS") == "Windows_NT" || strings.Contains(strings.ToLower(os.Getenv("OS")), "windows") || filepath.Ext(exePath) == ".exe"

	if isWindows {
		localApp := os.Getenv("LOCALAPPDATA")
		if localApp == "" {
			localApp = filepath.Join(os.Getenv("USERPROFILE"), "AppData", "Local")
		}
		targetDir = filepath.Join(localApp, "Microsoft", "WindowsApps")
		targetExe = filepath.Join(targetDir, "wiredrop.exe")
	} else {
		targetDir = "/usr/local/bin"
		targetExe = filepath.Join(targetDir, "wiredrop")
	}

	_ = os.MkdirAll(targetDir, 0755)

	srcData, err := os.ReadFile(exePath)
	if err != nil {
		log.Fatalf("[X] Không thể đọc file nguồn (%s): %v", exePath, err)
	}

	if err := os.WriteFile(targetExe, srcData, 0755); err != nil {
		log.Fatalf("[X] Lỗi cài đặt vào %s: %v\n(Gợi ý: Kiểm tra xem có tiến trình wiredrop nào đang chạy chiếm file không)", targetExe, err)
	}

	fmt.Println("================================================================")
	fmt.Printf("  ✓ ĐÃ CÀI ĐẶT WIREDROP TOÀN CỤC THÀNH CÔNG (%s)\n", Version)
	fmt.Println("================================================================")
	fmt.Printf("  • File thực thi: %s\n", targetExe)
	fmt.Println("  • Giờ đây bạn có thể mở bất kỳ cửa sổ CMD hoặc PowerShell nào")
	fmt.Println("    ở bất kỳ thư mục nào và gõ trực tiếp:")
	fmt.Println("      wiredrop <room_id>         (Mở đầu phát TX)")
	fmt.Println("      wiredrop <room_id> -r      (Mở đầu nhận RX)")
	fmt.Println("================================================================")
}

// runUninstallCommand gỡ bỏ wiredrop khỏi thư mục toàn cục
func runUninstallCommand() {
	var targetExe string
	isWindows := os.Getenv("OS") == "Windows_NT" || strings.Contains(strings.ToLower(os.Getenv("OS")), "windows")

	if isWindows {
		localApp := os.Getenv("LOCALAPPDATA")
		if localApp == "" {
			localApp = filepath.Join(os.Getenv("USERPROFILE"), "AppData", "Local")
		}
		targetExe = filepath.Join(localApp, "Microsoft", "WindowsApps", "wiredrop.exe")
	} else {
		targetExe = "/usr/local/bin/wiredrop"
	}

	if err := os.Remove(targetExe); err != nil {
		if os.IsNotExist(err) {
			fmt.Printf("[i] Không tìm thấy bản cài đặt tại: %s\n", targetExe)
			return
		}
		log.Fatalf("[X] Lỗi gỡ cài đặt: %v", err)
	}

	fmt.Printf("✓ Đã gỡ bỏ WireDrop thành công khỏi: %s\n", targetExe)
}
