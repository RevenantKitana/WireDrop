package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"
)

// isStdinPipe kiểm tra xem stdin có phải từ đường ống pipe hay không (ví dụ: cat file | wiredrop 88)
func isStdinPipe() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) == 0
}

// isStdoutPipe kiểm tra xem stdout có được pipe sang tiến trình khác hay không (ví dụ: wiredrop 88 -r | bash)
func isStdoutPipe() bool {
	fi, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) == 0
}

// openBrowser mở URL bằng trình duyệt mặc định trên các hệ điều hành
func openBrowser(targetURL string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", targetURL)
	case "darwin":
		cmd = exec.Command("open", targetURL)
	default:
		cmd = exec.Command("xdg-open", targetURL)
	}
	return cmd.Start()
}

// getLocalOutboundIP lấy IP LAN cục bộ để in hướng dẫn kết nối mạng nội bộ
func getLocalOutboundIP() string {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return "127.0.0.1"
	}
	defer conn.Close()
	localAddr := conn.LocalAddr().(*net.UDPAddr)
	return localAddr.IP.String()
}

// TerminalPipeOptions chứa các cờ cấu hình cho Terminal Pipe
type TerminalPipeOptions struct {
	Slug         string
	IsReceiver   bool
	UseShortlink bool
	OpenWeb      bool
	IsQuiet      bool
	ServerURL    string
}

// parseTerminalPipeArgs bóc tách tham số dòng lệnh linh hoạt
func parseTerminalPipeArgs(args []string) TerminalPipeOptions {
	opts := TerminalPipeOptions{
		ServerURL: "",
	}

	for i := 0; i < len(args); i++ {
		arg := args[i]
		lower := strings.ToLower(arg)

		switch lower {
		case "open", "send", "s":
			// Bỏ qua từ khóa hành động gửi
		case "recv", "r":
			opts.IsReceiver = true
		case "-r", "--rx", "-rx", "--r":
			opts.IsReceiver = true
		case "-s", "-short", "--short", "--s":
			opts.UseShortlink = true
		case "-w", "-web", "--web", "--w":
			opts.OpenWeb = true
		case "-q", "-quiet", "--quiet", "--q":
			opts.IsQuiet = true
		case "-u", "-url", "--url":
			if i+1 < len(args) {
				opts.ServerURL = args[i+1]
				i++
			}
		case "-h", "--help", "-help":
			printTerminalPipeHelp()
			os.Exit(0)
		default:
			if !strings.HasPrefix(arg, "-") && opts.Slug == "" {
				opts.Slug = arg
			}
		}
	}

	// Nếu slug chưa có: sinh ngẫu nhiên 4 ký tự
	if opts.Slug == "" {
		opts.Slug = randomSlug(4)
	}

	// Tự động nhận diện hậu tố -s hoặc -r nếu người dùng gõ thẳng ví dụ 'wiredrop 88-r'
	if strings.HasSuffix(opts.Slug, "-r") {
		opts.IsReceiver = true
		opts.Slug = strings.TrimSuffix(opts.Slug, "-r")
	} else if strings.HasSuffix(opts.Slug, "-s") {
		opts.IsReceiver = false
		opts.Slug = strings.TrimSuffix(opts.Slug, "-s")
	}

	// Tự động bật chế độ im lặng (Quiet) nếu Stdout đang bị pipe sang tiến trình khác
	if isStdoutPipe() {
		opts.IsQuiet = true
	}

	return opts
}

// printTerminalPipeHelp in hướng dẫn sử dụng dòng lệnh
func printTerminalPipeHelp() {
	fmt.Printf("WireDrop %s - Terminal-Native Pipe & Live Relay\n\n", Version)
	fmt.Println("CÚ PHÁP:")
	fmt.Println("  wiredrop <room_id> [flags]         Mở phiên Terminal Pipe (Mặc định: Đầu Phát TX)")
	fmt.Println("  wiredrop <room_id> -r [flags]      Mở luồng Terminal Receiver (Đầu Nhận RX)")
	fmt.Println("  wiredrop host [flags]              Khởi chạy Cloudflare FastHost Tunnel")
	fmt.Println("  wiredrop server [flags]            Khởi chạy HTTP Server thuần túy")
	fmt.Println()
	fmt.Println("CÁC CỜ HỖ TRỢ CHO TERMINAL PIPE:")
	fmt.Println("  -r, --rx           Chế độ Nhận (Receiver): Mở luồng lắng nghe stdout")
	fmt.Println("  -s, --short        Ưu tiên rút gọn qua Gateway s.wd.io.vn")
	fmt.Println("  -w, --web          Mở phòng bằng trình duyệt web mặc định")
	fmt.Println("  -q, --quiet        Tắt banner, chỉ in raw stream (tự động bật khi pipe |)")
	fmt.Println("  -u, --url <addr>   Chỉ định địa chỉ WireDrop Server từ xa (mặc định: localhost:8080)")
	fmt.Println("  -h, --help         In hướng dẫn này")
	fmt.Println()
	fmt.Println("VÍ DỤ TRUYỀN NHANH:")
	fmt.Println("  wiredrop 88                        Mở prompt tương tác phát text/file")
	fmt.Println("  wiredrop 88 -r                     Lắng nghe dữ liệu phòng 88 trên terminal")
	fmt.Println("  cat script.sh | wiredrop 88        Pipe trực tiếp script vào phòng 88")
	fmt.Println("  wiredrop 88 -r | bash              Hứng dữ liệu và thực thi trực tiếp qua bash")
}

// resolveAndEnsureServer xác định địa chỉ server và tự khởi động server cục bộ nếu chưa có
func resolveAndEnsureServer(customURL string) string {
	if customURL != "" {
		return strings.TrimRight(customURL, "/")
	}

	if envURL := os.Getenv("WIREDROP_SERVER"); envURL != "" {
		return strings.TrimRight(envURL, "/")
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	localURL := "http://localhost:" + port

	// Thử kiểm tra xem server local đã chạy chưa
	client := &http.Client{Timeout: 300 * time.Millisecond}
	resp, err := client.Get(localURL + "/health")
	if err == nil && resp.StatusCode == http.StatusOK {
		_ = resp.Body.Close()
		return localURL
	}
	if resp != nil {
		_ = resp.Body.Close()
	}

	// Server chưa chạy: Tự động khởi động embedded server chạy ngầm
	go func() {
		srv, _ := runServer(port, false)
		_ = srv.ListenAndServe()
	}()

	// Đợi tối đa 500ms cho server nội bộ sẵn sàng
	for i := 0; i < 10; i++ {
		time.Sleep(50 * time.Millisecond)
		resp, err := client.Get(localURL + "/health")
		if err == nil && resp.StatusCode == http.StatusOK {
			_ = resp.Body.Close()
			break
		}
		if resp != nil {
			_ = resp.Body.Close()
		}
	}

	return localURL
}

// runTerminalPipeCommand là hàm điều phối chính khi người dùng gõ 'wiredrop <room>' hoặc 'wiredrop open <room>'
func runTerminalPipeCommand(args []string) {
	opts := parseTerminalPipeArgs(args)
	serverURL := resolveAndEnsureServer(opts.ServerURL)

	// Xử lý cờ mở Web Browser (-w)
	if opts.OpenWeb {
		roleSuffix := "-s"
		if opts.IsReceiver {
			roleSuffix = "-r"
		}

		targetURL := fmt.Sprintf("%s/%s%s", serverURL, opts.Slug, roleSuffix)
		if opts.UseShortlink {
			targetURL = fmt.Sprintf("https://s.wd.io.vn/%s%s", opts.Slug, roleSuffix)
		}

		fmt.Printf("[*] Đang mở trình duyệt: %s\n", targetURL)
		if err := openBrowser(targetURL); err != nil {
			fmt.Printf("[X] Không thể mở trình duyệt: %v. Hãy mở link thủ công.\n", err)
		}
		return
	}

	// Nếu là Bên Nhận (RX)
	if opts.IsReceiver {
		runTerminalReceiver(serverURL, opts.Slug, opts.IsQuiet, opts.UseShortlink)
		return
	}

	// Nếu là Bên Phát (TX)
	// Kiểm tra xem Stdin có bị pipe từ lệnh khác vào hay không (ví dụ: cat file | wiredrop 88)
	if isStdinPipe() {
		runTerminalSenderPipe(serverURL, opts.Slug)
		return
	}

	// Nếu là Interactive REPL thông thường
	runTerminalSenderREPL(serverURL, opts.Slug, opts.UseShortlink)
}

// runTerminalSenderPipe đọc toàn bộ dữ liệu từ Stdin và stream liên tục sang phòng
func runTerminalSenderPipe(serverURL, slug string) {
	postURL := fmt.Sprintf("%s/%s-s", serverURL, slug)
	req, err := http.NewRequest(http.MethodPost, postURL, os.Stdin)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[X] Lỗi tạo request: %v\n", err)
		os.Exit(1)
	}
	req.Header.Set("Content-Type", "application/octet-stream")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[X] Lỗi đẩy pipe: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusConflict {
		fmt.Fprintln(os.Stderr, "[X] Lỗi 409: Đầu phát (*-s) đang bị một phiên làm việc khác chiếm giữ!")
		os.Exit(1)
	}
	if resp.StatusCode != http.StatusOK {
		fmt.Fprintf(os.Stderr, "[X] Lỗi đẩy pipe: HTTP %d\n", resp.StatusCode)
		os.Exit(1)
	}
}

// runTerminalSenderREPL cung cấp giao diện tương tác REPL cho Bên Phát (TX)
func runTerminalSenderREPL(serverURL, slug string, useShortlink bool) {
	lanIP := getLocalOutboundIP()
	port := "8080"
	if u, err := url.Parse(serverURL); err == nil && u.Port() != "" {
		port = u.Port()
	}

	// Xây dựng các đường link mời máy nhận
	urlDirectRx := fmt.Sprintf("http://%s:%s/%s-r", lanIP, port, slug)
	urlLocalRx := fmt.Sprintf("%s/%s-r", serverURL, slug)

	var displayRxWeb string
	var displayRxCLI string
	var displayRxWin string
	var displayRxFile string

	if useShortlink {
		shortR, _ := registerShortlinkWithGateway("https://s.wd.io.vn", slug, serverURL)
		if shortR != "" {
			displayRxWeb = shortR
			displayRxCLI = fmt.Sprintf("curl -sN %s", shortR)
			displayRxWin = fmt.Sprintf("irm %s | iex", shortR)
			displayRxFile = fmt.Sprintf("curl -sLJO %s", shortR)
		}
	}

	if displayRxWeb == "" {
		displayRxWeb = urlDirectRx
		displayRxCLI = fmt.Sprintf("curl -sN %s", urlDirectRx)
		displayRxWin = fmt.Sprintf("irm %s | iex", urlDirectRx)
		displayRxFile = fmt.Sprintf("curl -sLJO %s", urlDirectRx)
	}

	fmt.Println()
	fmt.Println("================================================================")
	fmt.Printf("  ⚡ WIREDROP PIPE: %s [ĐẦU PHÁT TX ĐÃ SẴN SÀNG]\n", slug)
	fmt.Println("================================================================")
	fmt.Println("  Gửi 1 trong các câu lệnh dưới đây cho Máy Nhận (0 CẦN CÀI ĐẶT):")
	fmt.Println()
	fmt.Printf("  📱 Web / Điện thoại:      %s\n", displayRxWeb)
	fmt.Printf("  🐧 Linux / macOS:        %s\n", displayRxCLI)
	fmt.Printf("                           (Chạy script: curl -sL %s | sh)\n", displayRxWeb)
	fmt.Printf("  🪟 Windows PowerShell:   %s\n", displayRxWin)
	fmt.Printf("  📦 Tải File nhị phân:    %s\n", displayRxFile)
	fmt.Println()
	fmt.Printf("  (Nếu máy nhận ĐÃ CÓ sẵn WireDrop binary: wiredrop %s -r)\n", slug)
	fmt.Println("----------------------------------------------------------------")
	fmt.Println("  Gõ bất kỳ dòng chữ nào và nhấn [Enter] để bắn sang máy nhận.")
	fmt.Println("  Lệnh nội bộ: :file <path> | :clear | :status | :help | :exit")
	fmt.Println("================================================================")

	scanner := bufio.NewScanner(os.Stdin)
	postLiveURL := fmt.Sprintf("%s/%s-s?live=1", serverURL, slug)
	postClearURL := fmt.Sprintf("%s/%s-s?clear=1", serverURL, slug)
	checkStatusURL := fmt.Sprintf("%s/%s/events?check=1&role=sender", serverURL, slug)

	client := &http.Client{Timeout: 5 * time.Second}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		fmt.Println("\n[*] Đã đóng phiên Terminal Pipe. Tạm biệt!")
		os.Exit(0)
	}()

	for {
		fmt.Printf("[%s-s:TX]> ", slug)
		if !scanner.Scan() {
			break
		}
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		if trimmed == "" {
			continue
		}

		// Xử lý các lệnh nội bộ REPL (bắt đầu bằng dấu hai chấm ':')
		if strings.HasPrefix(trimmed, ":") {
			parts := strings.SplitN(trimmed, " ", 2)
			cmd := strings.ToLower(parts[0])

			switch cmd {
			case ":exit", ":quit", ":q":
				fmt.Println("[*] Thoát phiên làm việc.")
				return

			case ":help", ":h":
				fmt.Println("--- DANH SÁCH LỆNH REPL ---")
				fmt.Println("  :file <đường_dẫn>    Bắn trực tiếp 1 file vào phòng")
				fmt.Println("  :clear               Xóa sạch dữ liệu phòng bên RX")
				fmt.Println("  :status              Kiểm tra trạng thái máy nhận (RX)")
				fmt.Println("  :exit, :quit         Thoát Terminal Pipe")
				continue

			case ":clear":
				resp, err := client.Post(postClearURL, "text/plain", nil)
				if err == nil && resp.StatusCode == http.StatusOK {
					_ = resp.Body.Close()
					fmt.Println("[OK] Đã gửi tín hiệu xóa sạch dữ liệu phòng.")
				} else {
					fmt.Println("[!] Không thể xóa dữ liệu phòng.")
				}
				continue

			case ":status":
				resp, err := client.Get(checkStatusURL)
				if err == nil && resp.StatusCode == http.StatusOK {
					var st struct {
						ReceiverPaired bool   `json:"receiver_paired"`
						SlotType       string `json:"slot_type"`
						SlotFileName   string `json:"slot_file_name"`
					}
					_ = json.NewDecoder(resp.Body).Decode(&st)
					_ = resp.Body.Close()

					rxStatus := "CHƯA KẾT NỐI (Chờ máy nhận)"
					if st.ReceiverPaired {
						rxStatus = "ĐÃ KẾT NỐI (ONLINE ●)"
					}
					fmt.Printf("[STATUS] Đầu nhận (RX): %s | Dữ liệu phòng: %s\n", rxStatus, st.SlotType)
				} else {
					fmt.Println("[!] Không thể lấy trạng thái phòng.")
				}
				continue

			case ":file":
				if len(parts) < 2 || strings.TrimSpace(parts[1]) == "" {
					fmt.Println("[!] Cú pháp: :file <đường_dẫn_tới_file>")
					continue
				}
				filePath := strings.TrimSpace(parts[1])
				filePath = strings.Trim(filePath, `"'`)
				uploadFileFromREPL(serverURL, slug, filePath)
				continue

			default:
				fmt.Printf("[!] Lệnh '%s' không tồn tại. Gõ :help để xem danh sách.\n", cmd)
				continue
			}
		}

		// Gửi dòng text thông thường qua HTTP Live Streaming
		req, err := http.NewRequest(http.MethodPost, postLiveURL, strings.NewReader(line+"\n"))
		if err == nil {
			req.Header.Set("Content-Type", "text/plain; charset=utf-8")
			req.Header.Set("X-Live", "1")
			resp, errDo := client.Do(req)
			if errDo != nil {
				fmt.Printf("[X] Lỗi truyền dòng: %v\n", errDo)
			} else {
				if resp.StatusCode == http.StatusConflict {
					fmt.Println("[X] Lỗi 409: Đầu phát (*-s) đang bị cửa sổ khác chiếm giữ!")
				}
				_ = resp.Body.Close()
			}
		}
	}
	_ = urlLocalRx
}

// uploadFileFromREPL nạp file nhị phân vào phòng ngay trong phiên làm việc
func uploadFileFromREPL(serverURL, slug, filePath string) {
	fi, err := os.Stat(filePath)
	if err != nil {
		fmt.Printf("[X] Không tìm thấy file: %v\n", err)
		return
	}

	file, err := os.Open(filePath)
	if err != nil {
		fmt.Printf("[X] Không thể mở file: %v\n", err)
		return
	}
	defer file.Close()

	fileName := filepath.Base(filePath)
	fileSize := fi.Size()
	fileSizeMB := float64(fileSize) / (1024 * 1024)

	fmt.Printf("[*] Đang nạp file '%s' (%.2f MB)...\n", fileName, fileSizeMB)

	uploadURL := fmt.Sprintf("%s/%s-s", serverURL, slug)
	req, err := http.NewRequest(http.MethodPost, uploadURL, file)
	if err != nil {
		fmt.Printf("[X] Lỗi tạo request: %v\n", err)
		return
	}

	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("X-Filename", fileName)
	req.ContentLength = fileSize

	client := &http.Client{Timeout: 30 * time.Minute}
	start := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("[X] Lỗi nạp file: %v\n", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Printf("[X] Server từ chối nạp file: HTTP %d\n", resp.StatusCode)
		return
	}

	duration := time.Since(start).Seconds()
	speed := fileSizeMB / duration
	fmt.Printf("[OK] Đã nạp xong file '%s' (%.2f MB) trong %.1fs (%.2f MB/s)!\n", fileName, fileSizeMB, duration, speed)
}

// runTerminalReceiver mở luồng nhận dữ liệu trực tiếp hiển thị ra terminal hoặc stdout
func runTerminalReceiver(serverURL, slug string, isQuiet, useShortlink bool) {
	targetURL := fmt.Sprintf("%s/%s-r?live=1", serverURL, slug)

	if !isQuiet {
		fmt.Println()
		fmt.Println("================================================================")
		fmt.Printf("  ⚡ WIREDROP PIPE: %s [ĐẦU NHẬN RX ĐANG LẮNG NGHE]\n", slug)
		fmt.Println("================================================================")
		fmt.Printf("  • Kết nối tới: %s\n", targetURL)
		fmt.Println("  • Đang chờ dữ liệu từ Đầu Phát (TX)... (Nhấn Ctrl+C để thoát)")
		fmt.Println("----------------------------------------------------------------")
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		if !isQuiet {
			fmt.Println("\n[*] Đã ngắt kết nối đầu nhận. Tạm biệt!")
		}
		os.Exit(0)
	}()

	client := &http.Client{Timeout: 0}

	for {
		req, err := http.NewRequest(http.MethodGet, targetURL, nil)
		if err != nil {
			if !isQuiet {
				fmt.Fprintf(os.Stderr, "[X] Lỗi tạo kết nối: %v\n", err)
			}
			return
		}
		req.Header.Set("Accept", "text/plain")

		resp, err := client.Do(req)
		if err != nil {
			if !isQuiet {
				fmt.Printf("[!] Mất kết nối tới server. Đang thử lại sau 2 giây...\n")
			}
			time.Sleep(2 * time.Second)
			continue
		}

		// Kiểm tra nếu payload là file đính kèm
		cd := resp.Header.Get("Content-Disposition")
		if strings.Contains(cd, "filename=") {
			saveIncomingFile(resp, cd, isQuiet)
			_ = resp.Body.Close()
			return
		}

		// Stream text từng dòng
		reader := bufio.NewReader(resp.Body)
		for {
			line, errRead := reader.ReadString('\n')
			if len(line) > 0 {
				// Bỏ qua dòng heartbeat keep-alive
				if strings.HasPrefix(line, ": keep-alive") {
					continue
				}
				// In thẳng ra stdout
				fmt.Print(line)
			}
			if errRead != nil {
				_ = resp.Body.Close()
				break
			}
		}

		if isQuiet {
			// Trong chế độ pipe, nếu server đóng stream thì thoát
			return
		}

		time.Sleep(1 * time.Second)
	}
}

// saveIncomingFile tự động lưu file nhị phân nhận được từ RX xuống đĩa cứng
func saveIncomingFile(resp *http.Response, contentDisposition string, isQuiet bool) {
	fileName := "downloaded_file"
	parts := strings.Split(contentDisposition, "filename=")
	if len(parts) > 1 {
		rawName := strings.Trim(parts[1], `"; `)
		if rawName != "" {
			fileName = rawName
		}
	}

	outFile, err := os.Create(fileName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[X] Không thể tạo file '%s': %v\n", fileName, err)
		return
	}
	defer outFile.Close()

	if !isQuiet {
		fileSizeMB := float64(resp.ContentLength) / (1024 * 1024)
		fmt.Printf("[*] Phát hiện file: '%s' (%.2f MB). Đang tải xuống...\n", fileName, fileSizeMB)
	}

	written, err := io.Copy(outFile, resp.Body)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[X] Lỗi ghi file: %v\n", err)
		return
	}

	if !isQuiet {
		fmt.Printf("[OK] Đã lưu thành công '%s' (%d bytes) vào thư mục hiện tại!\n", fileName, written)
	}
}
