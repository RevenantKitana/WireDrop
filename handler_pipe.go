package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// formatContentDisposition tạo header Content-Disposition chuẩn RFC 5987 cho cả ASCII và UTF-8
func formatContentDisposition(rawName string) string {
	base := filepath.Base(rawName)
	safeQuotes := strings.ReplaceAll(base, `"`, `_`)
	encoded := url.PathEscape(base)
	return fmt.Sprintf("attachment; filename=\"%s\"; filename*=UTF-8''%s", safeQuotes, encoded)
}

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
	slug := strings.ToLower(r.PathValue("room"))
	slug = strings.TrimSuffix(slug, "-s")
	slug = strings.TrimSuffix(slug, "-r")
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
		} else if room.Slot.Type == PayloadFile || room.Slot.Type == PayloadChunkStream {
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

		// Kiểm tra nếu là tín hiệu xóa sạch dữ liệu phòng (clear)
		if r.URL.Query().Get("clear") == "1" {
			room.ClearSlot()
			room.BroadcastSSE("payload_cleared", "")
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			_, _ = w.Write([]byte("CLEARED\n"))
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
			fileEvt, _ := json.Marshal(map[string]interface{}{
				"type": "file",
				"name": header.Filename,
				"size": len(data),
			})
			room.BroadcastSSE("payload_updated", string(fileEvt))

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
			var fileSize int64

			if contentLength > 0 && contentLength <= ramThreshold {
				data, err := io.ReadAll(r.Body)
				if err != nil {
					http.Error(w, "Upload error", http.StatusBadRequest)
					return
				}
				fileSize = int64(len(data))
				room.SetFile(filename, fileSize, data, "")
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
				fileSize = written
				room.SetFile(filename, fileSize, nil, tmpFile.Name())
			}

			fileEvt, _ := json.Marshal(map[string]interface{}{
				"type": "file",
				"name": filename,
				"size": fileSize,
			})
			room.BroadcastSSE("payload_updated", string(fileEvt))
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

		payloadJSON, _ := json.Marshal(map[string]interface{}{
			"type":    "text",
			"content": string(cleanText),
		})
		room.BroadcastSSE("payload_updated", string(payloadJSON))

		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte("SAVED\n"))

	default:
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
	}
}

// handleReceiver xử lý Đầu Nhận (*-r): Dành cho Worker B (Thuần túy Read-Only)
func handleReceiver(w http.ResponseWriter, r *http.Request) {
	slug := strings.ToLower(r.PathValue("room"))
	slug = strings.TrimSuffix(slug, "-r")
	slug = strings.TrimSuffix(slug, "-s")
	if slug == "" {
		http.NotFound(w, r)
		return
	}

	connID := getConnID(r)

	// 1. Nếu mở bằng Trình duyệt để xem trang (không truyền connID): Luôn phục vụ Web SPA ở chế độ Receiver ngay cả khi phòng chưa nạp dữ liệu
	if r.Method == http.MethodGet && !isCLI(r) && connID == "" {
		serveIndexHTML(w, r)
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

	// 2. Kiểm tra Khóa Kết Nối Vật Lý cho Đầu Nhận
	allowed, errMsg := room.CheckReceiverAuth(connID)
	if !allowed {
		http.Error(w, errMsg, http.StatusConflict) // 409 Line Busy
		return
	}

	// 3. Kiểm tra nếu Máy B muốn mở Live Pipe (curl -sN domain.com/1-r | bash)
	if r.URL.Query().Get("live") == "1" || strings.Contains(r.Header.Get("Accept"), "text/event-stream") || r.Header.Get("Range") == "live" {
		streamConnID := "cli_stream_" + fmt.Sprintf("%d", time.Now().UnixNano())
		ok, lockErr := room.AcquireReceiver(streamConnID, false)
		if !ok {
			http.Error(w, lockErr, http.StatusConflict)
			return
		}
		defer room.ReleaseReceiver(streamConnID)

		handleLivePipeStream(w, r, room)
		return
	}

	// 4. Phục vụ vật phẩm đang nằm trong khe cắm duy nhất (Single Slot)
	room.RLock()
	slot := room.Slot
	shouldBurn := room.BurnAfterRead
	room.RUnlock()

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

		if isCLI(r) && strings.TrimSpace(slot.TextData) != "" {
			ackData := fmt.Sprintf(`{"by":"cli","action":"read","time":"%s"}`, time.Now().Format("15:04:05"))
			room.BroadcastSSE("payload_consumed", ackData)
		}

		if shouldBurn {
			room.ClearSlot()
			room.BroadcastSSE("payload_cleared", "")
		}

	case PayloadFile:
		// Trả về File nhị phân kèm Content-Disposition để curl -sLJO tự nhận đúng tên file
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Disposition", formatContentDisposition(slot.FileName))
		w.Header().Set("Content-Length", fmt.Sprintf("%d", slot.FileSize))
		w.Header().Set("X-File-Size", fmt.Sprintf("%d", slot.FileSize))
		w.Header().Set("X-Total-Chunks", "1")

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

		if isCLI(r) {
			ackData := fmt.Sprintf(`{"by":"cli","action":"download","time":"%s"}`, time.Now().Format("15:04:05"))
			room.BroadcastSSE("payload_consumed", ackData)
		}

		if shouldBurn {
			room.ClearSlot()
			room.BroadcastSSE("payload_cleared", "")
		}

	case PayloadChunkStream:
		// Phục vụ file cắt mảnh Turbo HTTP stream vi khung 256KB liên tục sang RX
		sess := room.GetChunkSession()
		if sess == nil {
			http.Error(w, "Chunk session expired", http.StatusGone)
			return
		}

		sess.Lock()
		if sess.idleTimer != nil {
			sess.idleTimer.Stop()
		}
		sess.Unlock()

		room.BroadcastSSE("rx_connected", "")

		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Disposition", formatContentDisposition(sess.FileName))
		if sess.TotalSize > 0 {
			w.Header().Set("Content-Length", fmt.Sprintf("%d", sess.TotalSize))
			w.Header().Set("X-File-Size", fmt.Sprintf("%d", sess.TotalSize))
		}
		w.Header().Set("X-Total-Chunks", fmt.Sprintf("%d", sess.TotalChunks))
		w.Header().Set("X-Chunk-Size", fmt.Sprintf("%d", sess.ChunkSize))
		w.Header().Set("X-Accel-Buffering", "no")

		flusher, ok := w.(http.Flusher)
		for i := 0; i < sess.TotalChunks; i++ {
			pipe := sess.GetOrCreatePipe(i)

			for {
				frame, okFrame := pipe.ReadFrame(sess.doneChan, ChunkStallLimit)
				if !okFrame {
					sess.Cancel("Pop subchunk timeout or aborted")
					return
				}

				if len(frame.Data) > 0 {
					_, writeErr := w.Write(frame.Data)
					sess.Lock()
					sess.FlushedTotalBytes += int64(len(frame.Data))
					sess.Unlock()

					if ok && sess.TotalChunks > 1 {
						flusher.Flush()
					}
					if frame.BufPtr != nil {
						subChunkPool.Put(frame.BufPtr)
					}
					if writeErr != nil {
						sess.Cancel("RX connection terminated")
						return
					}
				}

				if frame.EOF {
					break
				}
			}

			// Hoàn tất chunk i, giải phóng và thông báo cho upload slot kế tiếp
			sess.AdvanceRxChunk(i)

			ackData, _ := json.Marshal(map[string]interface{}{
				"type":  "chunk_ack",
				"index": i,
				"total": sess.TotalChunks,
			})
			room.BroadcastSSE("chunk_ack", string(ackData))
		}

		consumedBy := "web"
		if isCLI(r) {
			consumedBy = "cli"
		}
		consumedData := fmt.Sprintf(`{"by":"%s","action":"download","time":"%s"}`, consumedBy, time.Now().Format("15:04:05"))
		room.BroadcastSSE("payload_consumed", consumedData)

		room.ClearSlot()
		room.BroadcastSSE("payload_cleared", "")

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
			room.BroadcastSSE("payload_consumed", fmt.Sprintf(`{"by":"cli","action":"pipe_exec","time":"%s"}`, time.Now().Format("15:04:05")))
		}
	}
}

// handleRoomEvents phát luồng SSE cập nhật trạng thái và duy trì khóa kết nối cho Web UI A và B
func handleRoomEvents(w http.ResponseWriter, r *http.Request) {
	slug := strings.ToLower(r.PathValue("room"))
	slug = strings.TrimSuffix(slug, "-s")
	slug = strings.TrimSuffix(slug, "-r")

	room := globalRooms.GetOrCreate(slug)
	room.Touch()

	role := strings.ToLower(r.URL.Query().Get("role"))
	connID := getConnID(r)
	force := r.URL.Query().Get("force") == "1"

	// Chế độ Pre-check kiểm tra kết nối có bị chiếm hay không (dành cho Web trước khi mở SSE)
	if r.URL.Query().Get("check") == "1" {
		w.Header().Set("Content-Type", "application/json")
		if !force {
			if role == "sender" {
				room.RLock()
				busy := (room.SenderConn != nil && room.SenderConn.ID != connID)
				room.RUnlock()
				if busy {
					w.WriteHeader(http.StatusConflict)
					_ = json.NewEncoder(w).Encode(map[string]interface{}{
						"error":   "line_busy",
						"role":    "sender",
						"message": "Đầu phát (*-s) đang có một cửa sổ khác chiếm giữ. Bấm [THỬ KẾT NỐI LẠI] để cắm lại dây vào cửa sổ này!",
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
						"message": "Đầu nhận (*-r) đang có một cửa sổ khác chiếm giữ. Bấm [THỬ KẾT NỐI LẠI] để cắm lại dây vào cửa sổ này!",
					})
					return
				}
			}
		}
		room.RLock()
		slotType := room.Slot.Type
		isChunked := (slotType == PayloadChunkStream)
		if isChunked {
			slotType = PayloadFile
		}
		totalChunks := 0
		chunkSize := 0
		if room.ChunkSess != nil {
			totalChunks = room.ChunkSess.TotalChunks
			chunkSize = room.ChunkSess.ChunkSize
		}
		respData := map[string]interface{}{
			"status":          "ok",
			"slug":            room.Slug,
			"sender_paired":   room.SenderConn != nil,
			"receiver_paired": room.ReceiverConn != nil,
			"paired":          room.ReceiverConn != nil,
			"burn":            room.BurnAfterRead,
			"slot_type":       slotType,
			"slot_text":       room.Slot.TextData,
			"slot_file_name":  room.Slot.FileName,
			"slot_file_size":  room.Slot.FileSize,
			"chunked":         isChunked,
			"total_chunks":    totalChunks,
			"chunk_size":      chunkSize,
		}
		room.RUnlock()
		_ = json.NewEncoder(w).Encode(respData)
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
		ok, errMsg := room.AcquireSender(connID, force)
		if !ok {
			http.Error(w, errMsg, http.StatusConflict)
			return
		}
		defer room.ReleaseSender(connID)
	} else if role == "receiver" {
		if connID == "" {
			connID = fmt.Sprintf("receiver_%d", time.Now().UnixNano())
		}
		ok, errMsg := room.AcquireReceiver(connID, force)
		if !ok {
			http.Error(w, errMsg, http.StatusConflict)
			return
		}
		defer room.ReleaseReceiver(connID)
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache, no-transform")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.Header().Set("Content-Encoding", "identity")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Gửi ngay 4KB padding để phá bộ đệm của Cloudflare Edge tức thì
	_, _ = fmt.Fprintf(w, ": %s\n\n", strings.Repeat(" ", 4096))
	flusher.Flush()

	sseChan := room.SubscribeSSE()
	defer room.UnsubscribeSSE(sseChan)

	// Gửi trạng thái ban đầu của sợi dây
	room.RLock()
	initSlotType := room.Slot.Type
	initChunked := (initSlotType == PayloadChunkStream)
	if initChunked {
		initSlotType = PayloadFile
	}
	initTotalChunks := 0
	initChunkSize := 0
	if room.ChunkSess != nil {
		initTotalChunks = room.ChunkSess.TotalChunks
		initChunkSize = room.ChunkSess.ChunkSize
	}
	initData, _ := json.Marshal(map[string]interface{}{
		"slug":            room.Slug,
		"sender_paired":   room.SenderConn != nil,
		"receiver_paired": room.ReceiverConn != nil,
		"paired":          room.ReceiverConn != nil,
		"burn":            room.BurnAfterRead,
		"slot_type":       initSlotType,
		"slot_text":       room.Slot.TextData,
		"slot_file_name":  room.Slot.FileName,
		"slot_file_size":  room.Slot.FileSize,
		"chunked":         initChunked,
		"total_chunks":    initTotalChunks,
		"chunk_size":      initChunkSize,
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
	slug := strings.ToLower(r.PathValue("room"))
	slug = strings.TrimSuffix(slug, "-s")
	slug = strings.TrimSuffix(slug, "-r")

	room := globalRooms.GetOrCreate(slug)
	room.ResetReceiver()

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok", "message": "Receiver unpaired"})
}

// handleChunkUpload tiếp nhận từng khối từ Bên Gửi (TX) theo cơ chế Turbo HTTP Pipelining
func handleChunkUpload(w http.ResponseWriter, r *http.Request) {
	slug := strings.ToLower(r.PathValue("room"))
	slug = strings.TrimSuffix(slug, "-s")
	slug = strings.TrimSuffix(slug, "-r")
	if slug == "" {
		http.NotFound(w, r)
		return
	}

	room := globalRooms.GetOrCreate(slug)
	room.Touch()

	connID := getConnID(r)
	allowed, errMsg := room.CheckSenderAuth(connID)
	if !allowed {
		http.Error(w, errMsg, http.StatusConflict)
		return
	}

	sessionID := r.Header.Get("X-Session-ID")
	fileName := r.Header.Get("X-File-Name")
	if decoded, decErr := url.PathUnescape(fileName); decErr == nil && decoded != "" {
		fileName = decoded
	}
	if fileName == "" {
		fileName = "unnamed.bin"
	}
	totalSize, _ := strconv.ParseInt(r.Header.Get("X-File-Size"), 10, 64)
	chunkIndex, _ := strconv.Atoi(r.Header.Get("X-Chunk-Index"))
	totalChunks, _ := strconv.Atoi(r.Header.Get("X-Total-Chunks"))
	if totalChunks <= 0 {
		totalChunks = 1
	}
	chunkSize, _ := strconv.Atoi(r.Header.Get("X-Chunk-Size"))
	if chunkSize <= 0 {
		chunkSize = ChunkSize
	}

	var sess *ChunkSession
	if chunkIndex == 0 {
		sess = room.StartChunkSession(sessionID, fileName, totalSize, totalChunks, chunkSize)
		fileEvt, _ := json.Marshal(map[string]interface{}{
			"type":         "file",
			"name":         fileName,
			"size":         totalSize,
			"chunked":      true,
			"total_chunks": totalChunks,
			"chunk_size":   chunkSize,
		})
		room.BroadcastSSE("payload_updated", string(fileEvt))

		room.RLock()
		rxConnected := (room.ReceiverConn != nil)
		room.RUnlock()
		if !rxConnected {
			sess.Lock()
			sess.idleTimer = time.AfterFunc(ChunkIdleHold, func() {
				room.ClearSlot()
				room.BroadcastSSE("session_timeout", "10 minutes idle")
				room.BroadcastSSE("payload_cleared", "")
			})
			sess.Unlock()
		}
	} else {
		sess = room.GetChunkSession()
		if sess == nil || sess.SessionID != sessionID {
			http.Error(w, "Chunk session expired or invalid", http.StatusGone)
			return
		}
	}

	// Kiểm tra và chờ slot nạp (Pipelining: tối đa 2 chunk chạy gối đầu)
	if !sess.WaitForUploadSlot(chunkIndex, ChunkStallLimit) {
		http.Error(w, "Upload slot timeout or aborted", http.StatusGone)
		return
	}

	pipe := sess.GetOrCreatePipe(chunkIndex)
	chunkStartOffset := sess.GetChunkStartOffset(chunkIndex)
	if offsetHdr := r.Header.Get("X-Byte-Offset"); offsetHdr != "" {
		if o, err := strconv.ParseInt(offsetHdr, 10, 64); err == nil {
			chunkStartOffset = o
		}
	}

	// Đọc r.Body theo từng SubChunk 256KB
	r.Body = http.MaxBytesReader(w, r.Body, int64(MaxChunkSize+65536))
	currentLocalOffset := int64(0)

	expectedChunkSize := int64(chunkSize)
	if totalSize > 0 && int64(chunkIndex+1)*int64(chunkSize) > totalSize {
		expectedChunkSize = totalSize - int64(chunkIndex)*int64(chunkSize)
	}

	for {
		bufPtr := subChunkPool.Get().(*[]byte)
		n, err := io.ReadFull(r.Body, *bufPtr)
		if n > 0 {
			subStart := chunkStartOffset + currentLocalOffset
			subEnd := subStart + int64(n)
			currentLocalOffset += int64(n)

			sess.RLock()
			flushedBytes := sess.FlushedTotalBytes
			sess.RUnlock()

			if subEnd <= flushedBytes {
				// Đã xả xong ở lượt trước (retry mid-chunk) -> Bỏ qua
				subChunkPool.Put(bufPtr)
			} else if subStart < flushedBytes {
				// Trùng 1 phần -> Cắt phần đã gửi
				skip := flushedBytes - subStart
				trimmed := (*bufPtr)[skip:n]
				ok := pipe.WriteFrame(&SubChunkFrame{
					Data:   trimmed,
					BufPtr: bufPtr,
					Offset: flushedBytes,
				}, sess.doneChan, ChunkStallLimit)
				if !ok {
					subChunkPool.Put(bufPtr)
					http.Error(w, "Pipe write frame failed", http.StatusGone)
					return
				}
			} else {
				// Vi khung mới hoàn toàn -> Ghi vào pipe
				ok := pipe.WriteFrame(&SubChunkFrame{
					Data:   (*bufPtr)[:n],
					BufPtr: bufPtr,
					Offset: subStart,
				}, sess.doneChan, ChunkStallLimit)
				if !ok {
					subChunkPool.Put(bufPtr)
					http.Error(w, "Pipe write frame failed", http.StatusGone)
					return
				}
			}

			if currentLocalOffset >= expectedChunkSize {
				// Đã nạp đủ dung lượng của chunk này!
				break
			}
		}

		if err != nil {
			if n == 0 {
				subChunkPool.Put(bufPtr)
			}
			if err == io.EOF && (totalSize <= 0 || currentLocalOffset >= expectedChunkSize) {
				break
			}
			// Bị ngắt kết nối hoặc đứt giữa chừng -> Không gửi EOF vào pipe, để RX tiếp tục chờ retry
			http.Error(w, fmt.Sprintf("Chunk incomplete: got %d/%d bytes: %v", currentLocalOffset, expectedChunkSize, err), http.StatusBadRequest)
			return
		}
	}

	// Báo cho pipe biết chunk này đã nạp trọn vẹn
	pipe.WriteFrame(&SubChunkFrame{EOF: true}, sess.doneChan, ChunkStallLimit)

	room.RLock()
	rxOnline := (room.ReceiverConn != nil)
	room.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	if chunkIndex == 0 && !rxOnline {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"status":       "waiting_rx",
			"chunk":        chunkIndex,
			"rx_online":    false,
			"next_allowed": chunkIndex + 1,
			"message":      "Chunk 0 buffered. Waiting for receiver.",
		})
		return
	}

	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"status":       "ok",
		"chunk":        chunkIndex,
		"rx_online":    rxOnline,
		"next_allowed": chunkIndex + 1,
	})
}

// handleChunkAbort hủy phiên truyền file do Bên Gửi yêu cầu
func handleChunkAbort(w http.ResponseWriter, r *http.Request) {
	slug := strings.ToLower(r.PathValue("room"))
	slug = strings.TrimSuffix(slug, "-s")
	slug = strings.TrimSuffix(slug, "-r")

	room, exists := globalRooms.Get(slug)
	if exists {
		connID := getConnID(r)
		if connID != "" {
			allowed, _ := room.CheckSenderAuth(connID)
			if !allowed {
				http.Error(w, "Unauthorized sender", http.StatusConflict)
				return
			}
		}
		room.ClearSlot()
		room.BroadcastSSE("payload_cleared", "")
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok", "message": "Session aborted"})
}

// handleRoomAck nhận phản hồi báo nhận từ Web UI Receiver (SAO CHÉP / TẢI VỀ)
func handleRoomAck(w http.ResponseWriter, r *http.Request) {
	slug := strings.ToLower(r.PathValue("room"))
	slug = strings.TrimSuffix(slug, "-r")
	slug = strings.TrimSuffix(slug, "-s")

	room, exists := globalRooms.Get(slug)
	if !exists {
		http.NotFound(w, r)
		return
	}

	action := r.URL.Query().Get("action")
	if action == "" {
		action = "copy"
	}

	ackData := fmt.Sprintf(`{"by":"web","action":"%s","time":"%s"}`, action, time.Now().Format("15:04:05"))
	room.BroadcastSSE("payload_consumed", ackData)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// handleHealth kiểm tra tình trạng sống của server
func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte("OK\n"))
}

// handleAPIShorten phục vụ yêu cầu tạo shortlink tuyển chọn theo yêu cầu (/api/shorten)
func handleAPIShorten(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	var payload struct {
		Slug        string `json:"slug"`
		Room        string `json:"room"`
		Destination string `json:"destination"`
	}
	_ = json.NewDecoder(r.Body).Decode(&payload)

	slug := payload.Slug
	if slug == "" {
		slug = payload.Room
	}
	if slug == "" {
		slug = randomSlug(4)
	}
	slug = strings.TrimSuffix(slug, "-s")
	slug = strings.TrimSuffix(slug, "-r")

	destination := payload.Destination
	if destination == "" {
		scheme := "http"
		if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
			scheme = "https"
		}
		destination = scheme + "://" + r.Host
	}

	gateway := os.Getenv("SHORTLINK_GATEWAY")
	if gateway == "" {
		gateway = "https://s.wd.io.vn"
	}

	shortR, shortS := registerShortlinkWithGateway(gateway, slug, destination)

	w.Header().Set("Content-Type", "application/json")
	if shortR == "" {
		// Fallback nếu gateway không phản hồi
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"status":   "fallback",
			"slug":     slug,
			"short_r":  fmt.Sprintf("%s/%s-r", destination, slug),
			"short_s":  fmt.Sprintf("%s/%s-s", destination, slug),
			"fallback": true,
		})
		return
	}

	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"status":   "ok",
		"slug":     slug,
		"short_r":  shortR,
		"short_s":  shortS,
		"fallback": false,
	})
}

