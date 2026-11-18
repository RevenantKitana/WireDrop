package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"syscall"
	"time"
)

// EnvConfig lưu cấu hình đọc từ file .env
type EnvConfig struct {
	Port            string
	CloudflareToken string
	CustomDomain    string
	TunnelProtocol  string
}

// loadEnvFile tìm và đọc file .env
func loadEnvFile(dir string) EnvConfig {
	cfg := EnvConfig{Port: "8080", TunnelProtocol: "http2"}

	// Tìm .env ở thư mục exe hoặc thư mục làm việc hiện tại
	envPaths := []string{
		filepath.Join(dir, ".env"),
		".env",
	}

	for _, p := range envPaths {
		data, err := os.ReadFile(p)
		if err == nil {
			lines := strings.Split(string(data), "\n")
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if line == "" || strings.HasPrefix(line, "#") {
					continue
				}
				parts := strings.SplitN(line, "=", 2)
				if len(parts) == 2 {
					key := strings.TrimSpace(parts[0])
					val := strings.TrimSpace(parts[1])
					val = strings.Trim(val, `"'`)
					switch key {
					case "PORT":
						if val != "" {
							cfg.Port = val
						}
					case "CLOUDFLARE_TUNNEL_TOKEN":
						cfg.CloudflareToken = val
					case "CUSTOM_DOMAIN":
						cfg.CustomDomain = val
					case "TUNNEL_PROTOCOL":
						if val != "" {
							cfg.TunnelProtocol = strings.ToLower(val)
						}
					}
				}
			}
			break
		}
	}
	return cfg
}

// saveEnvFile lưu cấu hình vào .env
func saveEnvFile(dir string, cfg EnvConfig) error {
	p := filepath.Join(dir, ".env")
	proto := cfg.TunnelProtocol
	if proto == "" {
		proto = "http2"
	}
	content := fmt.Sprintf("# WireDrop FastHosting Configuration\nPORT=%s\nCLOUDFLARE_TUNNEL_TOKEN=%s\nCUSTOM_DOMAIN=%s\nTUNNEL_PROTOCOL=%s\n",
		cfg.Port, cfg.CloudflareToken, cfg.CustomDomain, proto)
	return os.WriteFile(p, []byte(content), 0644)
}

// progressWriter theo dõi và in thanh tiến độ tải file
type progressWriter struct {
	total       int64
	downloaded  int64
	lastPercent int
}

func (pw *progressWriter) Write(p []byte) (int, error) {
	n := len(p)
	pw.downloaded += int64(n)
	if pw.total > 0 {
		percent := int(float64(pw.downloaded) / float64(pw.total) * 100)
		if percent != pw.lastPercent {
			pw.lastPercent = percent
			bars := percent / 5
			barStr := strings.Repeat("█", bars) + strings.Repeat("░", 20-bars)
			downloadedMB := float64(pw.downloaded) / (1024 * 1024)
			totalMB := float64(pw.total) / (1024 * 1024)
			fmt.Printf("\r    [%s] %3d%% (%.1f/%.1f MB)", barStr, percent, downloadedMB, totalMB)
		}
	}
	return n, nil
}

// findOrDownloadCloudflared định vị hoặc tải cloudflared portable
func findOrDownloadCloudflared(exeDir string) (string, error) {
	binName := "cloudflared"
	if runtime.GOOS == "windows" {
		binName = "cloudflared.exe"
	}

	// 1. Kiểm tra trong ./bin/ cạnh file exe
	localBin := filepath.Join(exeDir, "bin", binName)
	if _, err := os.Stat(localBin); err == nil {
		return localBin, nil
	}

	// 2. Kiểm tra cùng cấp với file exe
	localSame := filepath.Join(exeDir, binName)
	if _, err := os.Stat(localSame); err == nil {
		return localSame, nil
	}

	// 3. Kiểm tra trong LocalAppData (Windows) hoặc ~/.wiredrop (Linux/Mac)
	var cacheBin string
	if runtime.GOOS == "windows" {
		localApp := os.Getenv("LOCALAPPDATA")
		if localApp == "" {
			localApp = exeDir
		}
		cacheBin = filepath.Join(localApp, "WireDrop", "bin", binName)
	} else {
		home := os.Getenv("HOME")
		if home == "" {
			home = "."
		}
		cacheBin = filepath.Join(home, ".wiredrop", "bin", binName)
	}

	if _, err := os.Stat(cacheBin); err == nil {
		return cacheBin, nil
	}

	// 4. Nếu chưa có, xác định URL tải tương ứng kiến trúc OS
	var downloadURL string
	switch runtime.GOOS {
	case "windows":
		switch runtime.GOARCH {
		case "arm64":
			downloadURL = "https://github.com/cloudflare/cloudflared/releases/latest/download/cloudflared-windows-arm64.exe"
		default:
			downloadURL = "https://github.com/cloudflare/cloudflared/releases/latest/download/cloudflared-windows-amd64.exe"
		}
	case "linux":
		switch runtime.GOARCH {
		case "arm64":
			downloadURL = "https://github.com/cloudflare/cloudflared/releases/latest/download/cloudflared-linux-arm64"
		default:
			downloadURL = "https://github.com/cloudflare/cloudflared/releases/latest/download/cloudflared-linux-amd64"
		}
	case "darwin":
		switch runtime.GOARCH {
		case "arm64":
			downloadURL = "https://github.com/cloudflare/cloudflared/releases/latest/download/cloudflared-darwin-arm64"
		default:
			downloadURL = "https://github.com/cloudflare/cloudflared/releases/latest/download/cloudflared-darwin-amd64"
		}
	default:
		return "", fmt.Errorf("he dieu hanh %s chua duoc ho tro tu dong tai", runtime.GOOS)
	}

	// Ưu tiên tải về thư mục bin/ cục bộ cạnh exe nếu có thể ghi được
	targetPath := localBin
	targetDir := filepath.Dir(targetPath)
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		// Nếu không ghi được cạnh exe (ví dụ ổ đĩa chỉ đọc), dùng cacheBin
		targetPath = cacheBin
		_ = os.MkdirAll(filepath.Dir(targetPath), 0755)
	}

	fmt.Printf("[*] Dang tu dong tai Cloudflare Portable (%s-%s)...\n", runtime.GOOS, runtime.GOARCH)
	fmt.Printf("    Nguon: %s\n", downloadURL)

	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Get(downloadURL)
	if err != nil {
		return "", fmt.Errorf("loi ket noi toi GitHub: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("loi tai file: HTTP %d", resp.StatusCode)
	}

	tmpFile := targetPath + ".tmp"
	out, err := os.OpenFile(tmpFile, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
	if err != nil {
		return "", fmt.Errorf("khong the tao file ghi: %w", err)
	}

	pw := &progressWriter{total: resp.ContentLength}
	_, err = io.Copy(out, io.TeeReader(resp.Body, pw))
	_ = out.Close()
	if err != nil {
		_ = os.Remove(tmpFile)
		return "", fmt.Errorf("loi trong qua trinh tai file: %w", err)
	}

	fmt.Println()
	if err := os.Rename(tmpFile, targetPath); err != nil {
		return "", fmt.Errorf("loi luu file: %w", err)
	}

	// Đảm bảo cờ thực thi trên Linux/macOS
	if runtime.GOOS != "windows" {
		_ = os.Chmod(targetPath, 0755)
	}

	fmt.Printf("[OK] Da tai Cloudflare Portable ve: %s\n", targetPath)
	return targetPath, nil
}

// setupCustomDomainEnvInteractive hỗ trợ cấu hình token domain qua terminal
func setupCustomDomainEnvInteractive(exeDir string, cfg *EnvConfig) bool {
	reader := bufio.NewReader(os.Stdin)
	fmt.Println()
	fmt.Println("================================================================")
	fmt.Println("        THIET LAP DOMAIN CA NHAN VOI CLOUDFLARE TUNNEL          ")
	fmt.Println("================================================================")
	fmt.Println("De dung Domain rieng co dinh, ban can Token tu Cloudflare Dashboard:")
	fmt.Println(" 1. Mo: https://dash.cloudflare.com/ > Zero Trust > Networks > Tunnels")
	fmt.Println(" 2. Bam Add a tunnel > Chon Cloudflared > Copy chuoi TOKEN.")
	fmt.Printf(" 3. Tai tab Public Hostname, tro subdomain ve: http://localhost:%s\n", cfg.Port)
	fmt.Println("----------------------------------------------------------------")

	fmt.Print(">> Dan Cloudflare Tunnel Token cua ban (eyJh...): ")
	token, _ := reader.ReadString('\n')
	token = strings.TrimSpace(token)
	if token == "" {
		fmt.Println("[!] Token khong duoc de trong. Huy thao tac.")
		return false
	}

	fmt.Print(">> Nhap ten domain ca nhan cua ban (vi du: wire.yourdomain.com): ")
	domain, _ := reader.ReadString('\n')
	domain = strings.TrimSpace(domain)

	cfg.CloudflareToken = token
	cfg.CustomDomain = domain

	if err := saveEnvFile(exeDir, *cfg); err != nil {
		fmt.Printf("[!] Khong the luu .env: %v\n", err)
	} else {
		fmt.Println("[OK] Da luu cau hinh vao .env thanh cong!")
	}
	return true
}

// runPortableHost điều phối toàn bộ trải nghiệm FastHost Portable độc lập
func runPortableHost() {
	exePath, _ := os.Executable()
	exeDir := filepath.Dir(exePath)

	cfg := loadEnvFile(exeDir)

	// Kiểm tra đối số dòng lệnh trực tiếp (ví dụ: wiredrop host 1, wiredrop host 2)
	mode := ""
	if len(os.Args) > 1 {
		for _, arg := range os.Args[1:] {
			if arg == "1" || arg == "--quick" {
				mode = "1"
				break
			}
			if arg == "2" || arg == "--custom" {
				mode = "2"
				break
			}
			if arg == "3" || arg == "--local" {
				mode = "3"
				break
			}
		}
	}

	// Nếu chưa có cờ, hiển thị Menu tương tác
	if mode == "" {
		labelCustom := "Domain ca nhan (Chua cau hinh)"
		if cfg.CloudflareToken != "" {
			if cfg.CustomDomain != "" {
				labelCustom = fmt.Sprintf("Domain ca nhan (%s)", cfg.CustomDomain)
			} else {
				labelCustom = "Domain ca nhan (Da cau hinh Token)"
			}
		}

		fmt.Println()
		fmt.Println("================================================================")
		fmt.Printf("     ⚡ WIREDROP PORTABLE %s - CLOUDFLARE TUNNEL LAUNCHER      \n", Version)
		fmt.Println("================================================================")
		fmt.Println(" [1] Domain ngau nhien (Quick Tunnel - Mac dinh)")
		fmt.Printf(" [2] %s\n", labelCustom)
		fmt.Println(" [3] Chi chay cuc bo (Local Only - Khong mo Internet)")
		fmt.Println(" [0] Thoat")
		fmt.Println("----------------------------------------------------------------")
		fmt.Print("Lua chon cua ban [1/2/3/0] (Mac dinh: 1): ")

		reader := bufio.NewReader(os.Stdin)
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)
		if input == "" {
			input = "1"
		}
		mode = input
	}

	switch mode {
	case "0":
		fmt.Println("Tam biet!")
		return
	case "3":
		// Chạy local thuần túy
		runLocalServerOnly(cfg.Port)
		return
	case "2":
		if cfg.CloudflareToken == "" {
			if !setupCustomDomainEnvInteractive(exeDir, &cfg) {
				pauseOnExit()
				return
			}
		}
		runTunnelFlow(exeDir, cfg, false)
	case "1":
		fallthrough
	default:
		runTunnelFlow(exeDir, cfg, true)
	}

	pauseOnExit()
}

// runLocalServerOnly chạy WireDrop thuần túy không mở tunnel
func runLocalServerOnly(port string) {
	srv, _ := runServer(port, true)
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logFatalIfInteractive(fmt.Sprintf("Loi khoi dong server: %v", err))
		}
	}()

	<-sigChan
	fmt.Println("\n[*] Dang dung WireDrop server...")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
	fmt.Println("[OK] Da giai phong cong. Thoat an toan.")
}

// runTunnelFlow khởi động server nội bộ và kết nối Cloudflare Tunnel
func runTunnelFlow(exeDir string, cfg EnvConfig, isQuick bool) {
	cfBin, err := findOrDownloadCloudflared(exeDir)
	if err != nil {
		fmt.Printf("[X] Khong the chuan bi Cloudflare Portable: %v\n", err)
		return
	}

	fmt.Printf("[*] Khoi dong WireDrop server tren cong %s...\n", cfg.Port)
	srv, _ := runServer(cfg.Port, false)

	serverErr := make(chan error, 1)
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErr <- err
		}
	}()

	// Đợi 300ms kiểm tra port có bị xung đột
	select {
	case err := <-serverErr:
		fmt.Printf("[X] Khong the khoi dong WireDrop (cong %s bi chiem): %v\n", cfg.Port, err)
		return
	case <-time.After(300 * time.Millisecond):
	}

	var cfCmd *exec.Cmd
	proto := cfg.TunnelProtocol
	if proto == "" {
		proto = "http2"
	}
	if isQuick {
		fmt.Printf("[*] Dang mo duong ham Cloudflare Quick Tunnel (protocol: %s)...\n", proto)
		cfCmd = exec.Command(cfBin, "tunnel", "--no-autoupdate", "--protocol", proto, "--url", "http://127.0.0.1:"+cfg.Port)
	} else {
		fmt.Printf("[*] Dang ket noi Cloudflare Named Tunnel theo Token (protocol: %s)...\n", proto)
		cfCmd = exec.Command(cfBin, "tunnel", "--no-autoupdate", "--protocol", proto, "run", "--token", cfg.CloudflareToken)
	}

	// Đọc cả stdout và stderr của cloudflared qua io.Pipe để lấy URL (bất kể phiên bản cloudflared ghi vào kênh nào)
	pr, pw := io.Pipe()
	cfCmd.Stdout = pw
	cfCmd.Stderr = pw

	if err := cfCmd.Start(); err != nil {
		fmt.Printf("[X] Khong the khoi chay Cloudflare: %v\n", err)
		_ = pw.Close()
		_ = pr.Close()
		_ = srv.Close()
		return
	}

	// Bắt tín hiệu Ctrl+C để dọn dẹp tiến trình an toàn
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	tunnelURLChan := make(chan string, 1)
	cfExitChan := make(chan error, 1)

	go func() {
		err := cfCmd.Wait()
		_ = pw.Close()
		cfExitChan <- err
	}()

	// Quét log tìm URL
	go func() {
		defer pr.Close()
		re := regexp.MustCompile(`https://[-a-zA-Z0-9.]*\.trycloudflare\.com`)
		scanner := bufio.NewScanner(pr)
		for scanner.Scan() {
			line := scanner.Text()
			if isQuick {
				if match := re.FindString(line); match != "" {
					select {
					case tunnelURLChan <- match:
					default:
					}
				}
			}
		}
	}()

	var activeURL string
	if isQuick {
		fmt.Print("[*] Dang ket noi toi Cloudflare Edge...")
		dotTicker := time.NewTicker(600 * time.Millisecond)
		defer dotTicker.Stop()

	loopQuick:
		for {
			select {
			case url := <-tunnelURLChan:
				activeURL = url
				// Kiem tra Cloudflare Edge warmup de tranh DNS propagation lag
				warmupClient := &http.Client{Timeout: 2 * time.Second}
				for retry := 0; retry < 12; retry++ {
					resp, err := warmupClient.Get(activeURL + "/health")
					if err == nil && resp.StatusCode == http.StatusOK {
						_ = resp.Body.Close()
						break
					}
					if resp != nil {
						_ = resp.Body.Close()
					}
					time.Sleep(500 * time.Millisecond)
				}
				fmt.Println(" [XONG]")
				break loopQuick
			case <-dotTicker.C:
				fmt.Print(".")
			case err := <-cfExitChan:
				fmt.Printf("\n[X] Cloudflare thoat som voi loi: %v\n", err)
				_ = srv.Close()
				return
			case <-time.After(35 * time.Second):
				fmt.Println("\n[X] Het thoi gian cho (Timeout) bat URL Cloudflare.")
				_ = cfCmd.Process.Kill()
				_ = srv.Close()
				return
			case <-sigChan:
				fmt.Println("\n[*] Nguoi dung yeu cau dung.")
				_ = cfCmd.Process.Kill()
				_ = srv.Close()
				return
			}
		}
	} else {
		// Custom Domain
		if cfg.CustomDomain != "" {
			activeURL = "https://" + cfg.CustomDomain
		} else {
			activeURL = "https://<your-custom-domain>"
		}
	}

	randSlug := randomSlug(4)
	fmt.Println()
	fmt.Println("================================================================")
	if isQuick {
		fmt.Println("       WIREDROP FASTHOST ONLINE (DOMAIN NGAU NHIEN)             ")
	} else {
		fmt.Println("       WIREDROP FASTHOST ONLINE (DOMAIN CA NHAN CO DINH)        ")
	}
	fmt.Println("================================================================")
	fmt.Printf(" • Web Hub (Trang chu #):   %s/#\n", activeURL)
	fmt.Println("   (Mo link tren de xem Cheatsheet hoac tu tao phong tuy y)")
	fmt.Println()
	fmt.Printf(" • Vi du truyen nhanh (Phong mau ngau nhien: %s):\n", randSlug)
	fmt.Printf("   - Web phat (TX):         %s/%s-s\n", activeURL, randSlug)
	fmt.Println("   - Lenh nhan (RX):")
	fmt.Printf("     Linux/macOS:           curl -sL %s/%s-r | sh\n", activeURL, randSlug)
	fmt.Printf("     Windows PowerShell:    irm %s/%s-r | iex\n", activeURL, randSlug)
	fmt.Printf("     Tai File:              curl -sLJO %s/%s-r\n", activeURL, randSlug)
	if isQuick {
		fmt.Println("----------------------------------------------------------------")
		fmt.Println(" [!] Luu y: Mot so nha mang VN (Viettel/VNPT/FPT) chan trycloudflare.com")
		fmt.Println("     Neu may khac khong truy cap duoc, hay dung [2] Domain ca nhan!")
	}
	fmt.Println("----------------------------------------------------------------")
	fmt.Println(" [Nhan Ctrl + C de dung va dong duong ham bat cu luc nao]")
	fmt.Println("================================================================")
	fmt.Println()

	// Chờ tín hiệu dừng hoặc tiến trình cloudflared kết thúc
	select {
	case <-sigChan:
		fmt.Println("\n[*] Dang dong duong ham va giai phong tien trinh...")
	case err := <-cfExitChan:
		fmt.Printf("\n[*] Duong ham Cloudflare da ngat (%v)...\n", err)
	}

	if cfCmd.Process != nil {
		_ = cfCmd.Process.Kill()
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
	fmt.Printf("[OK] Da giai phong cong %s. Thoat an toan.\n", cfg.Port)
}

// pauseOnExit tạm dừng màn hình nếu chạy từ Windows Explorer đúp chuột
func pauseOnExit() {
	if runtime.GOOS == "windows" {
		fmt.Print("\nNhan phim Enter de dong cua so...")
		_, _ = bufio.NewReader(os.Stdin).ReadBytes('\n')
	}
}

// logFatalIfInteractive in lỗi và dừng
func logFatalIfInteractive(msg string) {
	fmt.Println(msg)
	pauseOnExit()
	os.Exit(1)
}
