package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// isCLI xác định client là công cụ dòng lệnh (curl, wget, PowerShell) hay Trình duyệt
func isCLI(r *http.Request) bool {
	if r.URL.Query().Get("raw") == "1" {
		return true
	}
	ua := strings.ToLower(r.Header.Get("User-Agent"))
	accept := strings.ToLower(r.Header.Get("Accept"))

	if strings.Contains(ua, "curl") ||
		strings.Contains(ua, "wget") ||
		strings.Contains(ua, "httpie") ||
		strings.Contains(ua, "powershell") ||
		strings.Contains(ua, "winhttp") {
		return true
	}

	return !strings.Contains(accept, "text/html")
}

// getConnID lấy mã phiên kết nối từ URL query param hoặc Header
func getConnID(r *http.Request) string {
	if cid := r.URL.Query().Get("conn"); cid != "" {
		return strings.TrimSpace(cid)
	}
	return strings.TrimSpace(r.Header.Get("X-Conn-ID"))
}

// handleSender xử lý Đầu Phát (*-s): Chỉ dành riêng cho Master A
func handleSender(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("room")
	slug = strings.TrimSuffix(slug, "-s")
	if slug == "" {
		http.NotFound(w, r)
		return
	}

	room := globalRooms.GetOrCreate(slug)
	room.Touch()

	// 1. Nếu mở bằng Trình duyệt: Phục vụ Web SPA ở chế độ Sender
	if r.Method == http.MethodGet && !isCLI(r) {
		serveIndexHTML(w, r)
		return
	}

	// 2. Kiểm tra quyền cắm vào đầu phát (Physical Connection Check)
	connID := getConnID(r)

	switch r.Method {
	case http.MethodGet:
		// Trả về thông tin trạng thái khe cắm hiện tại cho CLI
		room.RLock()
		slotType := "empty"
		if room.Slot.Type == PayloadText {
			slotType = "text"
		} else if room.Slot.Type == PayloadFile {
			slotType = "file"
		}
		isHeld := room.SenderConn != nil
		room.RUnlock()

		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = fmt.Fprintf(w, "WireDrop Pipe /%s-s (Status: %s, Occupied: %v)\n", slug, slotType, isHeld)

	case http.MethodPost:
		// Kiểm tra quyền phát dữ liệu (nếu web đang giữ mà gửi từ nơi khác thì chặn 409)
		allowed, errMsg := room.CheckSenderAuth(connID)
		if !allowed {
			http.Error(w, errMsg, http.StatusConflict)
			return
		}

		// Kiểm tra nếu là Live Command đẩy sang Live Pipe
		if r.URL.Query().Get("live") == "1" || r.Header.Get("X-Live") == "1" {
			r.Body = http.MaxBytesReader(w, r.Body, 1024*1024)
			body, _ := io.ReadAll(r.Body)
			cmd := strings.TrimSpace(string(body))
			if cmd != "" {
				room.BroadcastCommand(cmd)
				room.BroadcastSSE("cmd_sent", cmd)
			}
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			_, _ = w.Write([]byte("SENT\n"))
			return
		}

		// Kiểm tra nếu có upload file dạng multipart/form-data
		if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
			// Giới hạn form upload 100MB
			if err := r.ParseMultipartForm(100 * 1024 * 1024); err != nil {
				http.Error(w, "Form parse error or file > 100MB", http.StatusBadRequest)
				return
			}

			files := r.MultipartForm.File["file"]
			if len(files) == 0 {
				http.Error(w, "No file provided", http.StatusBadRequest)
				return
			}

			// Chỉ lấy duy nhất file đầu tiên theo nguyên tắc Single Atomic Slot
			header := files[0]
			file, err := header.Open()
			if err != nil {
				http.Error(w, "Cannot open file", http.StatusInternalServerError)
				return
			}
			defer file.Close()

			data, err := io.ReadAll(file)
			if err != nil {
				http.Error(w, "Cannot read file", http.StatusInternalServerError)
				return
			}

			// Nạp file vào khe cắm duy nhất
			room.SetFile(header.Filename, int64(len(data)), data, "")

			// Báo cho Web A và B biết slot đã đổi sang File
			room.BroadcastSSE("payload_updated", fmt.Sprintf(`{"type":"file","name":"%s","size":%d}`, header.Filename, len(data)))

			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"status": "ok",
				"type":   "file",
				"name":   header.Filename,
				"size":   len(data),
			})
			return
		}

		// Nạp dữ liệu Raw (Text/Script hoặc File qua curl -T)
		r.Body = http.MaxBytesReader(w, r.Body, 150*1024*1024)

		// Nếu có header X-Filename hoặc cờ upload file
		if filename := r.Header.Get("X-Filename"); filename != "" {
			// Nhận file nhị phân
			const ramThreshold = 15 * 1024 * 1024
			contentLength := r.ContentLength

			if contentLength > 0 && contentLength <= ramThreshold {
				data, err := io.ReadAll(r.Body)
				if err != nil {
					http.Error(w, "Upload error", http.StatusBadRequest)
					return
				}
				room.SetFile(filename, int64(len(data)), data, "")
			} else {
				// File lớn > 15MB: Ghi tạm vào /tmp
				tmpFile, err := os.CreateTemp("", "tc_pipe_*")
				if err != nil {
					http.Error(w, "Cannot create temp file", http.StatusInternalServerError)
					return
				}
				defer tmpFile.Close()

				written, err := io.Copy(tmpFile, r.Body)
				if err != nil {
					_ = os.Remove(tmpFile.Name())
					http.Error(w, "Stream upload failed", http.StatusBadRequest)
					return
				}
				room.SetFile(filename, written, nil, tmpFile.Name())
			}

			room.BroadcastSSE("payload_updated", fmt.Sprintf(`{"type":"file","name":"%s"}`, filename))
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			_, _ = fmt.Fprintf(w, "FILE_LOADED: %s\n", filename)
			return
		}

		// Ngược lại: Nhận Text/Script thông thường
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Payload read error", http.StatusBadRequest)
			return
		}

		// Tự động chuẩn hóa CRLF \r\n -> \n
		cleanText := bytes.ReplaceAll(body, []byte("\r\n"), []byte("\n"))
		room.SetText(string(cleanText))

		room.BroadcastSSE("payload_updated", fmt.Sprintf(`{"type":"text","content":"%s"}`, string(cleanText)))

		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte("SAVED\n"))

	default:
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
	}
}

// handleReceiver xử lý Đầu Nhận (*-r): Dành cho Worker B (Thuần túy Read-Only)
func handleReceiver(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("room")
	slug = strings.TrimSuffix(slug, "-r")
	if slug == "" {
		http.NotFound(w, r)
		return
	}

	room, exists := globalRooms.Get(slug)
	if !exists {
		// Trả về comment rỗng an toàn nếu phòng chưa từng tồn tại
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte("# WireDrop: (empty)\n"))
		return
	}
	room.Touch()

	connID := getConnID(r)

	// 1. Nếu mở bằng Trình duyệt để xem trang (không truyền connID): Phục vụ Web SPA ở chế độ Receiver
	if r.Method == http.MethodGet && !isCLI(r) && connID == "" {
		serveIndexHTML(w, r)
		return
	}

	// 2. Kiểm tra Khóa Kết Nối Vật Lý cho Đầu Nhận
	allowed, errMsg := room.CheckReceiverAuth(connID)
	if !allowed {
		http.Error(w, errMsg, http.StatusConflict) // 409 Line Busy
		return
	}

	// 3. Kiểm tra nếu Máy B muốn mở Live Pipe (curl -sN domain.com/1-r | bash)
	if r.URL.Query().Get("live") == "1" || strings.Contains(r.Header.Get("Accept"), "text/event-stream") || r.Header.Get("Range") == "live" {
		streamConnID := "cli_stream_" + fmt.Sprintf("%d", time.Now().UnixNano())
		ok, lockErr := room.AcquireReceiver(streamConnID)
		if !ok {
			http.Error(w, lockErr, http.StatusConflict)
			return
		}
		defer room.ReleaseReceiver(streamConnID)

		handleLivePipeStream(w, r, room)
		return
	}

	// 4. Phục vụ vật phẩm đang nằm trong khe cắm duy nhất (Single Slot)
	room.Lock()
	slot := room.Slot
	shouldBurn := room.BurnAfterRead

	// Nếu bật Burn-After-Read: rút cạn slot sau khi phục vụ
	if shouldBurn {
		room.Slot = PipeSlot{Type: PayloadEmpty}
	}
	room.Unlock()

	switch slot.Type {
	case PayloadText:
		// Trả về text thô cho curl / irm
		textToSend := slot.TextData
		if strings.TrimSpace(textToSend) == "" {
			textToSend = "# WireDrop: (empty)\n"
		} else {
			textToSend = strings.ReplaceAll(textToSend, "\r\n", "\n")
		}

		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte(textToSend))

		if shouldBurn {
			room.BroadcastSSE("payload_cleared", "")
		}

	case PayloadFile:
		// Trả về File nhị phân kèm Content-Disposition để curl -sLJO tự nhận đúng tên file
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filepath.Base(slot.FileName)))
		w.Header().Set("Content-Length", fmt.Sprintf("%d", slot.FileSize))

		if len(slot.FileData) > 0 {
			_, _ = w.Write(slot.FileData)
		} else if slot.TempPath != "" {
			f, err := os.Open(slot.TempPath)
			if err == nil {
				defer f.Close()
				_, _ = io.Copy(w, f)
			}
			if shouldBurn {
				_ = os.Remove(slot.TempPath)
			}
		}

		if shouldBurn {
			room.BroadcastSSE("payload_cleared", "")
		}

	default:
		// Khe cắm đang rỗng: Trả về comment an toàn, không báo lỗi PowerShell iex
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte("# WireDrop: (empty)\n"))
	}
}

// handleLivePipeStream phục vụ luồng lệnh tức thì cho bash (curl -sN .../1-r | bash)
func handleLivePipeStream(w http.ResponseWriter, r *http.Request, room *RoomState) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Transfer-Encoding", "chunked")
	w.Header().Set("Cache-Control", "no-cache, no-transform")
	w.Header().Set("X-Accel-Buffering", "no")

	cmdChan := room.SubscribeStream()
	defer room.UnsubscribeStream(cmdChan)

	// Gửi banner chào mừng an toàn
	_, _ = fmt.Fprint(w, "# WireDrop Live Pipe Connected (Pure 1-to-1 Stream)\n")
	flusher.Flush()

	heartbeat := time.NewTicker(20 * time.Second)
	defer heartbeat.Stop()

	notify := r.Context().Done()

	for {
		select {
		case <-notify:
			return
		case <-heartbeat.C:
			// Heartbeat keep-alive tránh bị proxy/firewall ngắt kết nối
			_, err := fmt.Fprint(w, ": keep-alive\n")
			if err != nil {
				return
			}
			flusher.Flush()
		case cmd, ok := <-cmdChan:
			if !ok {
				return
			}
			cleanCmd := strings.TrimRight(cmd, "\r\n")
			_, err := fmt.Fprintf(w, "%s\n", cleanCmd)
			if err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

// handleRoomEvents phát luồng SSE cập nhật trạng thái và duy trì khóa kết nối cho Web UI A và B
func handleRoomEvents(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("room")
	slug = strings.TrimSuffix(slug, "-s")
	slug = strings.TrimSuffix(slug, "-r")

	room := globalRooms.GetOrCreate(slug)
	room.Touch()

	role := strings.ToLower(r.URL.Query().Get("role"))
	connID := getConnID(r)

	// Chế độ Pre-check kiểm tra kết nối có bị chiếm hay không (dành cho Web trước khi mở SSE)
	if r.URL.Query().Get("check") == "1" {
		w.Header().Set("Content-Type", "application/json")
		if role == "sender" {
			room.RLock()
			busy := (room.SenderConn != nil && room.SenderConn.ID != connID)
			room.RUnlock()
			if busy {
				w.WriteHeader(http.StatusConflict)
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"error":   "line_busy",
					"role":    "sender",
					"message": "Đầu phát (*-s) đang có một cửa sổ khác chiếm giữ. Sợi dây này chỉ cho phép duy nhất 1 cửa sổ phát!",
				})
				return
			}
		} else if role == "receiver" {
			room.RLock()
			busy := (room.ReceiverConn != nil && room.ReceiverConn.ID != connID)
			room.RUnlock()
			if busy {
				w.WriteHeader(http.StatusConflict)
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"error":   "line_busy",
					"role":    "receiver",
					"message": "Đầu nhận (*-r) đang có một cửa sổ hoặc kết nối khác chiếm giữ. Sợi dây này chỉ cho phép duy nhất 1 kết nối nhận!",
				})
				return
			}
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "SSE unsupported", http.StatusInternalServerError)
		return
	}

	// Chiếm giữ đầu dây theo vai trò
	if role == "sender" {
		if connID == "" {
			connID = fmt.Sprintf("sender_%d", time.Now().UnixNano())
		}
		ok, errMsg := room.AcquireSender(connID)
		if !ok {
			http.Error(w, errMsg, http.StatusConflict)
			return
		}
		defer room.ReleaseSender(connID)
	} else if role == "receiver" {
		if connID == "" {
			connID = fmt.Sprintf("receiver_%d", time.Now().UnixNano())
		}
		ok, errMsg := room.AcquireReceiver(connID)
		if !ok {
			http.Error(w, errMsg, http.StatusConflict)
			return
		}
		defer room.ReleaseReceiver(connID)
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	sseChan := room.SubscribeSSE()
	defer room.UnsubscribeSSE(sseChan)

	// Gửi trạng thái ban đầu của sợi dây
	room.RLock()
	initData, _ := json.Marshal(map[string]interface{}{
		"slug":            room.Slug,
		"sender_paired":   room.SenderConn != nil,
		"receiver_paired": room.ReceiverConn != nil,
		"paired":          room.ReceiverConn != nil,
		"burn":            room.BurnAfterRead,
		"slot_type":       room.Slot.Type,
		"slot_text":       room.Slot.TextData,
		"slot_file_name":  room.Slot.FileName,
		"slot_file_size":  room.Slot.FileSize,
	})
	room.RUnlock()

	_, _ = fmt.Fprintf(w, "event: init\ndata: %s\n\n", initData)
	flusher.Flush()

	heartbeat := time.NewTicker(15 * time.Second)
	defer heartbeat.Stop()

	notify := r.Context().Done()
	for {
		select {
		case <-notify:
			return
		case <-heartbeat.C:
			_, _ = fmt.Fprint(w, ": heartbeat\n\n")
			flusher.Flush()
		case msg, ok := <-sseChan:
			if !ok {
				return
			}
			_, _ = w.Write([]byte(msg))
			flusher.Flush()
		}
	}
}

// handleResetReceiver giải phóng Máy B khỏi đầu nhận
func handleResetReceiver(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("room")
	slug = strings.TrimSuffix(slug, "-s")
	slug = strings.TrimSuffix(slug, "-r")

	room := globalRooms.GetOrCreate(slug)
	room.ResetReceiver()

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok", "message": "Receiver unpaired"})
}

// handleInitScriptLinux trả về script bootstrap tạo phím tắt 'r' cho Linux (/i)
func handleInitScriptLinux(w http.ResponseWriter, r *http.Request) {
	host := r.Host
	scheme := "http"
	if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}

	script := fmt.Sprintf(`# WireDrop Linux Helper
r() {
    local room="${1:-1}"
    curl -sL "%s://%s/${room}-r" | sh
}
echo "✓ WireDrop alias 'r' initialized! Usage: r [room_id]"
`, scheme, host)

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte(script))
}

// handleInitScriptWindows trả về script bootstrap tạo phím tắt 'r' cho PowerShell (/i.ps1)
func handleInitScriptWindows(w http.ResponseWriter, r *http.Request) {
	host := r.Host
	scheme := "http"
	if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}

	script := fmt.Sprintf(`# WireDrop Windows PowerShell Helper
function r($room="1") {
    irm "%s://%s/${room}-r" | iex
}
Write-Host "✓ WireDrop function 'r' initialized! Usage: r [room_id]" -ForegroundColor Green
`, scheme, host)

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte(script))
}

// handleHealth kiểm tra tình trạng sống của server
func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte("OK\n"))
}
