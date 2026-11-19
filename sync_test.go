package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestReceiverWebServesIndexHTML(t *testing.T) {
	handler := createHandler()
	ts := httptest.NewServer(handler)
	defer ts.Close()

	// Giả lập Trình duyệt mở link /freshroom-r khi phòng chưa từng có dữ liệu
	req, err := http.NewRequest("GET", ts.URL+"/freshroom-r", nil)
	if err != nil {
		t.Fatalf("Lỗi tạo request: %v", err)
	}
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Lỗi gọi HTTP: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Kỳ vọng Status 200 OK, nhận %d", resp.StatusCode)
	}
	contentType := resp.Header.Get("Content-Type")
	if !strings.Contains(contentType, "text/html") {
		t.Errorf("Kỳ vọng Content-Type chứa text/html, nhận %s", contentType)
	}

	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "WIREDROP") {
		t.Errorf("Kỳ vọng body chứa WIREDROP Web SPA, nhận: %s", string(body))
	}
}

func TestReceiverCLIServesEmpty(t *testing.T) {
	handler := createHandler()
	ts := httptest.NewServer(handler)
	defer ts.Close()

	// Giả lập curl /emptyroom-r
	req, err := http.NewRequest("GET", ts.URL+"/emptyroom-r", nil)
	if err != nil {
		t.Fatalf("Lỗi tạo request: %v", err)
	}
	req.Header.Set("User-Agent", "curl/8.4.0")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Lỗi gọi HTTP: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Kỳ vọng Status 200 OK, nhận %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "# WireDrop: (empty)") {
		t.Errorf("Kỳ vọng body chứa '# WireDrop: (empty)', nhận: %s", string(body))
	}
}

func TestSSEHeadersAndBufferingPadding(t *testing.T) {
	handler := createHandler()
	ts := httptest.NewServer(handler)
	defer ts.Close()

	// Mở kết nối SSE
	req, err := http.NewRequest("GET", ts.URL+"/cloudtest/events?role=receiver&conn=conn_test_cf", nil)
	if err != nil {
		t.Fatalf("Lỗi tạo request: %v", err)
	}
	req.Header.Set("Accept", "text/event-stream")

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Lỗi kết nối SSE: %v", err)
	}
	defer resp.Body.Close()

	// Kiểm tra Headers chống đệm Cloudflare Edge
	if xab := resp.Header.Get("X-Accel-Buffering"); xab != "no" {
		t.Errorf("Kỳ vọng X-Accel-Buffering: no, nhận: %s", xab)
	}
	if ce := resp.Header.Get("Content-Encoding"); ce != "identity" {
		t.Errorf("Kỳ vọng Content-Encoding: identity, nhận: %s", ce)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.Contains(ct, "text/event-stream") {
		t.Errorf("Kỳ vọng Content-Type text/event-stream, nhận: %s", ct)
	}

	// Đọc chunk đầu tiên: phải có 4KB padding để phá đệm Cloudflare
	reader := bufio.NewReader(resp.Body)
	paddingLine, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("Không thể đọc dòng đệm padding: %v", err)
	}

	if !strings.HasPrefix(paddingLine, ": ") || len(paddingLine) < 4000 {
		t.Errorf("Kỳ vọng dòng padding dài >= 4000 bytes, nhận: %d bytes", len(paddingLine))
	}
}

func TestSenderAndReceiverMatchingAndSync(t *testing.T) {
	handler := createHandler()
	ts := httptest.NewServer(handler)
	defer ts.Close()

	// 1. Receiver kết nối SSE tới phòng 'matchbox'
	rxReq, _ := http.NewRequest("GET", ts.URL+"/matchbox/events?role=receiver&conn=conn_rx", nil)
	rxReq.Header.Set("Accept", "text/event-stream")
	rxClient := &http.Client{}
	rxResp, err := rxClient.Do(rxReq)
	if err != nil {
		t.Fatalf("Lỗi kết nối RX SSE: %v", err)
	}
	defer rxResp.Body.Close()

	rxScanner := bufio.NewScanner(rxResp.Body)

	// Đọc bỏ qua dòng padding 2KB ban đầu
	if !rxScanner.Scan() {
		t.Fatalf("Không đọc được padding ban đầu")
	}

	// 2. Sender kết nối SSE tới phòng 'MATCHBOX' (kiểm tra case-insensitive)
	txReq, _ := http.NewRequest("GET", ts.URL+"/MATCHBOX/events?role=sender&conn=conn_tx", nil)
	txReq.Header.Set("Accept", "text/event-stream")
	txClient := &http.Client{}
	txResp, err := txClient.Do(txReq)
	if err != nil {
		t.Fatalf("Lỗi kết nối TX SSE: %v", err)
	}
	defer txResp.Body.Close()

	// 3. Sender gửi văn bản có ký tự đặc biệt, xuống dòng và ngoặc kép
	complexScript := "echo \"Line 1: Hello from 'WireDrop'\"\r\necho \"Line 2: Multi-line & Unicode: Tiếng Việt\"\r\n"
	postReq, _ := http.NewRequest("POST", ts.URL+"/MATCHBOX-s?conn=conn_tx", strings.NewReader(complexScript))
	postReq.Header.Set("Content-Type", "text/plain; charset=utf-8")
	postResp, err := http.DefaultClient.Do(postReq)
	if err != nil {
		t.Fatalf("Lỗi gửi POST: %v", err)
	}
	defer postResp.Body.Close()
	if postResp.StatusCode != http.StatusOK {
		t.Fatalf("Kỳ vọng POST 200 OK, nhận %d", postResp.StatusCode)
	}

	// 4. Kiểm tra Receiver SSE nhận được payload_updated với JSON chuẩn
	var foundPayloadUpdated bool
	timeout := time.After(3 * time.Second)
	linesChan := make(chan string, 20)

	go func() {
		for rxScanner.Scan() {
			linesChan <- rxScanner.Text()
		}
	}()

loop:
	for {
		select {
		case line := <-linesChan:
			if strings.HasPrefix(line, "data: {") && strings.Contains(line, "payload_updated") || strings.Contains(line, "Line 1") {
				// Parse JSON để kiểm tra tính hợp lệ
				jsonStr := strings.TrimPrefix(line, "data: ")
				var parsed map[string]interface{}
				if err := json.Unmarshal([]byte(jsonStr), &parsed); err != nil {
					t.Fatalf("Lỗi JSON SSE payload_updated không hợp lệ: %v, chuỗi: %s", err, jsonStr)
				}
				if parsed["type"] == "text" && strings.Contains(parsed["content"].(string), "Tiếng Việt") {
					foundPayloadUpdated = true
					break loop
				}
			}
		case <-timeout:
			break loop
		}
	}

	if !foundPayloadUpdated {
		t.Errorf("Receiver SSE không nhận được sự kiện payload_updated đồng bộ từ Sender")
	}

	// 5. Kiểm tra CLI tải /matchbox-r (Receiver)
	cliReq, _ := http.NewRequest("GET", ts.URL+"/matchbox-r", nil)
	cliReq.Header.Set("User-Agent", "curl/8.0")
	cliResp, err := http.DefaultClient.Do(cliReq)
	if err != nil {
		t.Fatalf("Lỗi CLI GET: %v", err)
	}
	defer cliResp.Body.Close()

	cliBody, _ := io.ReadAll(cliResp.Body)
	expectedClean := "echo \"Line 1: Hello from 'WireDrop'\"\necho \"Line 2: Multi-line & Unicode: Tiếng Việt\"\n"
	if string(cliBody) != expectedClean {
		t.Errorf("Kỳ vọng CLI nhận:\n%q\nNhận được:\n%q", expectedClean, string(cliBody))
	}
}

func TestForceReacquire(t *testing.T) {
	handler := createHandler()
	ts := httptest.NewServer(handler)
	defer ts.Close()

	// Tab 1 chiếm giữ Sender
	req1, _ := http.NewRequest("GET", ts.URL+"/forceromm/events?role=sender&conn=tab_1", nil)
	resp1, err := http.DefaultClient.Do(req1)
	if err != nil {
		t.Fatalf("Lỗi tab 1: %v", err)
	}
	defer resp1.Body.Close()

	// Tab 2 kiểm tra không cờ force -> 409
	checkReq, _ := http.NewRequest("GET", ts.URL+"/forceromm/events?role=sender&conn=tab_2&check=1", nil)
	checkResp, err := http.DefaultClient.Do(checkReq)
	if err != nil {
		t.Fatalf("Lỗi check tab 2: %v", err)
	}
	defer checkResp.Body.Close()
	if checkResp.StatusCode != http.StatusConflict {
		t.Errorf("Kỳ vọng 409 Conflict cho tab 2, nhận %d", checkResp.StatusCode)
	}

	// Tab 2 kiểm tra VỚI cờ force=1 -> 200 OK
	forceCheckReq, _ := http.NewRequest("GET", ts.URL+"/forceromm/events?role=sender&conn=tab_2&check=1&force=1", nil)
	forceCheckResp, err := http.DefaultClient.Do(forceCheckReq)
	if err != nil {
		t.Fatalf("Lỗi force check tab 2: %v", err)
	}
	defer forceCheckResp.Body.Close()
	if forceCheckResp.StatusCode != http.StatusOK {
		t.Errorf("Kỳ vọng 200 OK khi force=1, nhận %d", forceCheckResp.StatusCode)
	}

	// Tab 2 cắm SSE VỚI cờ force=1 -> thành công cướp quyền
	forceSSEReq, _ := http.NewRequest("GET", ts.URL+"/forceromm/events?role=sender&conn=tab_2&force=1", nil)
	forceSSEResp, err := http.DefaultClient.Do(forceSSEReq)
	if err != nil {
		t.Fatalf("Lỗi force SSE tab 2: %v", err)
	}
	defer forceSSEResp.Body.Close()
	if forceSSEResp.StatusCode != http.StatusOK {
		t.Errorf("Kỳ vọng 200 OK khi force SSE, nhận %d", forceSSEResp.StatusCode)
	}
}

func TestCheckReturnsRoomSnapshot(t *testing.T) {
	handler := createHandler()
	ts := httptest.NewServer(handler)
	defer ts.Close()

	// 1. Sender nạp text vào phòng 'snaptest'
	postReq, _ := http.NewRequest("POST", ts.URL+"/snaptest-s?conn=snap_tx", strings.NewReader("secret_token_123"))
	postReq.Header.Set("Content-Type", "text/plain; charset=utf-8")
	postResp, err := http.DefaultClient.Do(postReq)
	if err != nil || postResp.StatusCode != http.StatusOK {
		t.Fatalf("Lỗi POST ban đầu: %v", err)
	}
	postResp.Body.Close()

	// 2. Receiver pre-check GET /snaptest/events?role=receiver&conn=snap_rx&check=1
	checkReq, _ := http.NewRequest("GET", ts.URL+"/snaptest/events?role=receiver&conn=snap_rx&check=1", nil)
	checkResp, err := http.DefaultClient.Do(checkReq)
	if err != nil {
		t.Fatalf("Lỗi check request: %v", err)
	}
	defer checkResp.Body.Close()

	if checkResp.StatusCode != http.StatusOK {
		t.Fatalf("Kỳ vọng 200 OK từ check, nhận: %d", checkResp.StatusCode)
	}

	var data map[string]interface{}
	if err := json.NewDecoder(checkResp.Body).Decode(&data); err != nil {
		t.Fatalf("Lỗi parse JSON: %v", err)
	}

	if data["status"] != "ok" {
		t.Errorf("Kỳ vọng status ok, nhận: %v", data["status"])
	}
	if data["slot_text"] != "secret_token_123" {
		t.Errorf("Kỳ vọng slot_text 'secret_token_123', nhận: %v", data["slot_text"])
	}
	if int(data["slot_type"].(float64)) != 1 {
		t.Errorf("Kỳ vọng slot_type 1 (text), nhận: %v", data["slot_type"])
	}
}

func TestChunkedPingPongPipe(t *testing.T) {
	handler := createHandler()
	ts := httptest.NewServer(handler)
	defer ts.Close()

	slug := "testpipe"
	sessionID := "sess_test_123"
	fileName := "bigdata.bin"

	// Tạo 3 chunks nhỏ để test (mỗi chunk 100KB, tổng 300KB)
	chunkSize := 100 * 1024
	totalChunks := 3
	totalSize := int64(chunkSize * totalChunks)

	chunk0Data := bytes.Repeat([]byte("A"), chunkSize)
	chunk1Data := bytes.Repeat([]byte("B"), chunkSize)
	chunk2Data := bytes.Repeat([]byte("C"), chunkSize)

	// 1. TX nạp Chunk 0 khi RX chưa kết nối
	req0, _ := http.NewRequest("POST", fmt.Sprintf("%s/%s-s/chunk", ts.URL, slug), bytes.NewReader(chunk0Data))
	req0.Header.Set("X-Session-ID", sessionID)
	req0.Header.Set("X-File-Name", fileName)
	req0.Header.Set("X-File-Size", fmt.Sprintf("%d", totalSize))
	req0.Header.Set("X-Chunk-Index", "0")
	req0.Header.Set("X-Total-Chunks", fmt.Sprintf("%d", totalChunks))
	req0.Header.Set("X-Chunk-Size", fmt.Sprintf("%d", chunkSize))

	resp0, err := http.DefaultClient.Do(req0)
	if err != nil || resp0.StatusCode != http.StatusOK {
		t.Fatalf("Lỗi nạp chunk 0: %v, code: %d", err, resp0.StatusCode)
	}
	var resMap0 map[string]interface{}
	_ = json.NewDecoder(resp0.Body).Decode(&resMap0)
	resp0.Body.Close()

	if resMap0["status"] != "waiting_rx" {
		t.Errorf("Kỳ vọng status waiting_rx khi RX chưa cắm dây, nhận: %v", resMap0["status"])
	}

	// 2. RX cắm dây kéo file qua GET /{slug}-r trong goroutine riêng
	rxReadChan := make(chan []byte, 1)
	rxErrChan := make(chan error, 1)

	go func() {
		rxReq, _ := http.NewRequest("GET", fmt.Sprintf("%s/%s-r?raw=1", ts.URL, slug), nil)
		rxResp, err := http.DefaultClient.Do(rxReq)
		if err != nil {
			rxErrChan <- err
			return
		}
		defer rxResp.Body.Close()

		if rxResp.StatusCode != http.StatusOK {
			rxErrChan <- fmt.Errorf("RX HTTP status: %d", rxResp.StatusCode)
			return
		}

		allData, err := io.ReadAll(rxResp.Body)
		if err != nil {
			rxErrChan <- err
			return
		}
		rxReadChan <- allData
	}()

	// Đợi RX bắt đầu kéo Chunk 0 (chờ 50ms)
	time.Sleep(50 * time.Millisecond)

	// 3. TX nạp tiếp Chunk 1
	req1, _ := http.NewRequest("POST", fmt.Sprintf("%s/%s-s/chunk", ts.URL, slug), bytes.NewReader(chunk1Data))
	req1.Header.Set("X-Session-ID", sessionID)
	req1.Header.Set("X-File-Name", fileName)
	req1.Header.Set("X-File-Size", fmt.Sprintf("%d", totalSize))
	req1.Header.Set("X-Chunk-Index", "1")
	req1.Header.Set("X-Total-Chunks", fmt.Sprintf("%d", totalChunks))
	req1.Header.Set("X-Chunk-Size", fmt.Sprintf("%d", chunkSize))

	resp1, err := http.DefaultClient.Do(req1)
	if err != nil || resp1.StatusCode != http.StatusOK {
		t.Fatalf("Lỗi nạp chunk 1: %v", err)
	}
	resp1.Body.Close()

	// 4. TX nạp tiếp Chunk 2 (chunk cuối)
	req2, _ := http.NewRequest("POST", fmt.Sprintf("%s/%s-s/chunk", ts.URL, slug), bytes.NewReader(chunk2Data))
	req2.Header.Set("X-Session-ID", sessionID)
	req2.Header.Set("X-File-Name", fileName)
	req2.Header.Set("X-File-Size", fmt.Sprintf("%d", totalSize))
	req2.Header.Set("X-Chunk-Index", "2")
	req2.Header.Set("X-Total-Chunks", fmt.Sprintf("%d", totalChunks))
	req2.Header.Set("X-Chunk-Size", fmt.Sprintf("%d", chunkSize))

	resp2, err := http.DefaultClient.Do(req2)
	if err != nil || resp2.StatusCode != http.StatusOK {
		t.Fatalf("Lỗi nạp chunk 2: %v", err)
	}
	resp2.Body.Close()

	// 5. Kiểm tra dữ liệu RX nhận được
	select {
	case err := <-rxErrChan:
		t.Fatalf("Lỗi phía RX kéo dữ liệu: %v", err)
	case receivedBytes := <-rxReadChan:
		if len(receivedBytes) != int(totalSize) {
			t.Fatalf("Kỳ vọng nhận %d bytes, thực tế: %d bytes", totalSize, len(receivedBytes))
		}
		// Kiểm tra tính toàn vẹn của từng mảnh
		expected := append(append(chunk0Data, chunk1Data...), chunk2Data...)
		if !bytes.Equal(receivedBytes, expected) {
			t.Errorf("Dữ liệu ghép từ 3 chunks không khớp với bản gốc!")
		}
	case <-time.After(5 * time.Second):
		t.Fatalf("Quá thời gian chờ RX tải xong (Timeout)")
	}
}

func TestChunkedAbortAndCleanup(t *testing.T) {
	handler := createHandler()
	ts := httptest.NewServer(handler)
	defer ts.Close()

	slug := "abortpipe"
	sessionID := "sess_abort_999"
	fileName := "abortfile.bin"
	chunkSize := 1024
	totalChunks := 5
	totalSize := int64(chunkSize * totalChunks)

	// 1. Nạp chunk 0
	req0, _ := http.NewRequest("POST", fmt.Sprintf("%s/%s-s/chunk?conn=abort_tx", ts.URL, slug), bytes.NewReader(bytes.Repeat([]byte("X"), chunkSize)))
	req0.Header.Set("X-Session-ID", sessionID)
	req0.Header.Set("X-File-Name", fileName)
	req0.Header.Set("X-File-Size", fmt.Sprintf("%d", totalSize))
	req0.Header.Set("X-Chunk-Index", "0")
	req0.Header.Set("X-Total-Chunks", fmt.Sprintf("%d", totalChunks))
	req0.Header.Set("X-Chunk-Size", fmt.Sprintf("%d", chunkSize))

	resp0, err := http.DefaultClient.Do(req0)
	if err != nil || resp0.StatusCode != http.StatusOK {
		t.Fatalf("Lỗi nạp chunk 0: %v", err)
	}
	resp0.Body.Close()

	// 2. Pre-check xác nhận session đang giữ file
	checkReq, _ := http.NewRequest("GET", fmt.Sprintf("%s/%s/events?role=receiver&conn=abort_rx&check=1", ts.URL, slug), nil)
	checkResp, err := http.DefaultClient.Do(checkReq)
	if err != nil || checkResp.StatusCode != http.StatusOK {
		t.Fatalf("Lỗi check: %v", err)
	}
	var checkData map[string]interface{}
	_ = json.NewDecoder(checkResp.Body).Decode(&checkData)
	checkResp.Body.Close()

	if checkData["chunked"] != true {
		t.Errorf("Kỳ vọng chunked == true, nhận: %v", checkData["chunked"])
	}

	// 3. Sender gửi lệnh Abort
	abortReq, _ := http.NewRequest("POST", fmt.Sprintf("%s/%s-s/abort?conn=abort_tx", ts.URL, slug), nil)
	abortResp, err := http.DefaultClient.Do(abortReq)
	if err != nil || abortResp.StatusCode != http.StatusOK {
		t.Fatalf("Lỗi abort: %v", err)
	}
	abortResp.Body.Close()

	// 4. Pre-check xác nhận slot đã được giải phóng hoàn toàn về Empty (slot_type = 0)
	checkReq2, _ := http.NewRequest("GET", fmt.Sprintf("%s/%s/events?role=receiver&conn=abort_rx&check=1", ts.URL, slug), nil)
	checkResp2, err := http.DefaultClient.Do(checkReq2)
	if err != nil || checkResp2.StatusCode != http.StatusOK {
		t.Fatalf("Lỗi check sau abort: %v", err)
	}
	var checkData2 map[string]interface{}
	_ = json.NewDecoder(checkResp2.Body).Decode(&checkData2)
	checkResp2.Body.Close()

	if int(checkData2["slot_type"].(float64)) != 0 {
		t.Errorf("Kỳ vọng slot_type = 0 sau abort, nhận: %v", checkData2["slot_type"])
	}
	if checkData2["chunked"] == true {
		t.Errorf("Kỳ vọng chunked == false sau abort, nhận true")
	}
}

func TestSingleFileUploadAndDownload(t *testing.T) {
	handler := createHandler()
	ts := httptest.NewServer(handler)
	defer ts.Close()

	slug := "singlefiletest"
	fileContent := []byte("The quick brown fox jumps over the lazy dog. 1234567890!")
	fileName := "sample.txt"

	// Tạo multipart form
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", fileName)
	if err != nil {
		t.Fatalf("Lỗi CreateFormFile: %v", err)
	}
	_, _ = part.Write(fileContent)
	_ = writer.Close()

	// 1. Upload file qua POST /singlefiletest-s
	postReq, _ := http.NewRequest("POST", fmt.Sprintf("%s/%s-s?conn=single_tx", ts.URL, slug), body)
	postReq.Header.Set("Content-Type", writer.FormDataContentType())
	postResp, err := http.DefaultClient.Do(postReq)
	if err != nil || postResp.StatusCode != http.StatusOK {
		t.Fatalf("Lỗi upload: %v, status: %d", err, postResp.StatusCode)
	}
	postResp.Body.Close()

	// 2. Download file qua GET /singlefiletest-r
	getReq, _ := http.NewRequest("GET", fmt.Sprintf("%s/%s-r?conn=single_rx", ts.URL, slug), nil)
	getResp, err := http.DefaultClient.Do(getReq)
	if err != nil || getResp.StatusCode != http.StatusOK {
		t.Fatalf("Lỗi download: %v, status: %d", err, getResp.StatusCode)
	}
	defer getResp.Body.Close()

	downloaded, err := io.ReadAll(getResp.Body)
	if err != nil {
		t.Fatalf("Lỗi read body: %v", err)
	}

	t.Logf("Downloaded len: %d, Content-Length header: %s, Disposition: %s",
		len(downloaded), getResp.Header.Get("Content-Length"), getResp.Header.Get("Content-Disposition"))

	if !bytes.Equal(downloaded, fileContent) {
		t.Errorf("Dữ liệu tải về không khớp! Kỳ vọng %d bytes, nhận %d bytes", len(fileContent), len(downloaded))
	}
}

func TestOneBatchChunkedDownload(t *testing.T) {
	handler := createHandler()
	ts := httptest.NewServer(handler)
	defer ts.Close()

	slug := "onebatch"
	sessionID := "sess_one_batch"
	fileName := "onebatch.bin"
	chunkSize := 102400 // 100KB
	totalChunks := 1
	totalSize := int64(chunkSize)
	chunk0Data := bytes.Repeat([]byte("Z"), chunkSize)

	// 1. Upload chunk 0 (totalChunks = 1)
	req0, _ := http.NewRequest("POST", fmt.Sprintf("%s/%s-s/chunk?conn=tx_1", ts.URL, slug), bytes.NewReader(chunk0Data))
	req0.Header.Set("X-Session-ID", sessionID)
	req0.Header.Set("X-File-Name", fileName)
	req0.Header.Set("X-File-Size", fmt.Sprintf("%d", totalSize))
	req0.Header.Set("X-Chunk-Index", "0")
	req0.Header.Set("X-Total-Chunks", fmt.Sprintf("%d", totalChunks))
	req0.Header.Set("X-Chunk-Size", fmt.Sprintf("%d", chunkSize))

	resp0, err := http.DefaultClient.Do(req0)
	if err != nil || resp0.StatusCode != http.StatusOK {
		t.Fatalf("Lỗi upload chunk 0: %v, status: %d", err, resp0.StatusCode)
	}
	var resMap0 map[string]interface{}
	_ = json.NewDecoder(resp0.Body).Decode(&resMap0)
	resp0.Body.Close()
	t.Logf("Upload resp: %v", resMap0)

	// 2. Download qua GET /onebatch-r?conn=rx_1
	rxReq, _ := http.NewRequest("GET", fmt.Sprintf("%s/%s-r?conn=rx_1", ts.URL, slug), nil)
	rxResp, err := http.DefaultClient.Do(rxReq)
	if err != nil || rxResp.StatusCode != http.StatusOK {
		t.Fatalf("Lỗi download: %v, status: %d", err, rxResp.StatusCode)
	}
	defer rxResp.Body.Close()

	allData, err := io.ReadAll(rxResp.Body)
	if err != nil {
		t.Fatalf("Lỗi read body: %v", err)
	}

	t.Logf("Downloaded len: %d, Content-Length: %s, Disposition: %s",
		len(allData), rxResp.Header.Get("Content-Length"), rxResp.Header.Get("Content-Disposition"))

	if len(allData) != chunkSize {
		t.Errorf("Kỳ vọng %d bytes, nhận %d bytes", chunkSize, len(allData))
	}
	if !bytes.Equal(allData, chunk0Data) {
		t.Errorf("Dữ liệu không khớp!")
	}
}

func TestHomeAndFrontendJSValidity(t *testing.T) {
	handler := createHandler()
	ts := httptest.NewServer(handler)
	defer ts.Close()

	// 1. Kiểm tra trang chủ GET / (và /#) cho trình duyệt
	req, _ := http.NewRequest("GET", ts.URL+"/", nil)
	req.Header.Set("Accept", "text/html,application/xhtml+xml")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64)")
	resp, err := http.DefaultClient.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("Lỗi truy cập trang chủ /: %v, status: %d", err, resp.StatusCode)
	}
	htmlBytes, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	htmlStr := string(htmlBytes)
	if !strings.Contains(htmlStr, `id="homeView"`) || !strings.Contains(htmlStr, `id="roomInput"`) {
		t.Errorf("Trang chủ thiếu cấu trúc homeView / roomInput!")
	}

	// 2. Kiểm tra /app.js và /style.css trả về đúng MIME type và không rỗng
	jsResp, err := http.Get(ts.URL + "/app.js")
	if err != nil || jsResp.StatusCode != http.StatusOK {
		t.Fatalf("Lỗi tải /app.js: %v", err)
	}
	jsBytes, _ := io.ReadAll(jsResp.Body)
	jsResp.Body.Close()
	if !strings.Contains(jsResp.Header.Get("Content-Type"), "javascript") || len(jsBytes) < 1000 {
		t.Errorf("Nội dung /app.js không hợp lệ!")
	}

	// 3. Nếu máy có Node.js: Chạy node --check để đảm bảo 100% không có lỗi cú pháp JS (tránh trắng trang)
	if nodePath, lookErr := exec.LookPath("node"); lookErr == nil {
		cmd := exec.Command(nodePath, "--check", "public/app.js")
		if out, cmdErr := cmd.CombinedOutput(); cmdErr != nil {
			t.Fatalf("Lỗi cú pháp JavaScript trong public/app.js làm hỏng trang Web:\n%s", string(out))
		}
	}
}

func TestEncodedUTF8FileNameInChunkedUpload(t *testing.T) {
	handler := createHandler()
	ts := httptest.NewServer(handler)
	defer ts.Close()

	slug := "utf8name"
	origName := "Báo cáo tháng 11 (Final).pdf"
	encodedName := url.PathEscape(origName)
	data := []byte("PDF-DUMMY-CONTENT")

	req, _ := http.NewRequest("POST", fmt.Sprintf("%s/%s-s/chunk?conn=tx_utf8", ts.URL, slug), bytes.NewReader(data))
	req.Header.Set("X-Session-ID", "sess_utf8_1")
	req.Header.Set("X-File-Name", encodedName)
	req.Header.Set("X-File-Size", fmt.Sprintf("%d", len(data)))
	req.Header.Set("X-Chunk-Index", "0")
	req.Header.Set("X-Total-Chunks", "1")
	req.Header.Set("X-Chunk-Size", fmt.Sprintf("%d", len(data)))

	resp, err := http.DefaultClient.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("Lỗi upload chunk UTF-8: %v", err)
	}
	resp.Body.Close()

	// Kiểm tra snapshot trả về đúng tên file tiếng Việt gốc (đã giải mã URI)
	snapResp, err := http.Get(fmt.Sprintf("%s/%s/events?role=receiver&conn=rx_utf8&check=1", ts.URL, slug))
	if err != nil || snapResp.StatusCode != http.StatusOK {
		t.Fatalf("Lỗi đọc snapshot: %v", err)
	}
	var snap map[string]interface{}
	_ = json.NewDecoder(snapResp.Body).Decode(&snap)
	snapResp.Body.Close()

	if snap["slot_file_name"] != origName {
		t.Errorf("Tên file chưa được giải mã đúng! Kỳ vọng %q, nhận %q", origName, snap["slot_file_name"])
	}
}

func TestTurboSubChunkStreamingTTFB(t *testing.T) {
	handler := createHandler()
	ts := httptest.NewServer(handler)
	defer ts.Close()

	slug := "turbottfb"
	chunkSize := 1024 * 1024 // 1MB (4 x 256KB sub-chunks)
	totalChunks := 1
	totalSize := int64(chunkSize)

	// TX tạo io.Pipe để kiểm soát việc gửi dữ liệu theo từng nhát 256KB
	pr, pw := io.Pipe()
	defer pw.Close()

	first256KB := bytes.Repeat([]byte("A"), 256*1024)
	rest768KB := bytes.Repeat([]byte("B"), 768*1024)

	uploadErrChan := make(chan error, 1)
	go func() {
		req, _ := http.NewRequest("POST", fmt.Sprintf("%s/%s-s/chunk?conn=tx_ttfb", ts.URL, slug), pr)
		req.Header.Set("X-Session-ID", "sess_ttfb")
		req.Header.Set("X-File-Name", "stream_test.bin")
		req.Header.Set("X-File-Size", fmt.Sprintf("%d", totalSize))
		req.Header.Set("X-Chunk-Index", "0")
		req.Header.Set("X-Total-Chunks", fmt.Sprintf("%d", totalChunks))
		req.Header.Set("X-Chunk-Size", fmt.Sprintf("%d", chunkSize))

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			uploadErrChan <- err
			return
		}
		resp.Body.Close()
		uploadErrChan <- nil
	}()

	// 1. TX chỉ mới bơm 256KB đầu tiên vào pipe (chưa đóng pipe, chưa xong 1MB)
	go func() {
		_, _ = pw.Write(first256KB)
	}()

	// Chờ server nhận request upload và chuyển slot sang PayloadChunkStream
	for i := 0; i < 50; i++ {
		r, ok := globalRooms.Get(slug)
		if ok {
			r.RLock()
			isChunk := (r.Slot.Type == PayloadChunkStream)
			r.RUnlock()
			if isChunk {
				break
			}
		}
		time.Sleep(10 * time.Millisecond)
	}

	// 2. RX cắm dây nhận
	rxReq, _ := http.NewRequest("GET", fmt.Sprintf("%s/%s-r?conn=rx_ttfb", ts.URL, slug), nil)
	rxResp, err := http.DefaultClient.Do(rxReq)
	if err != nil || rxResp.StatusCode != http.StatusOK {
		t.Fatalf("Lỗi kết nối RX: %v", err)
	}
	defer rxResp.Body.Close()

	// 3. RX đọc 256KB đầu tiên ngay khi TX chưa gửi phần còn lại (chứng minh TTFB tức thì)
	buf256 := make([]byte, 256*1024)
	n, err := io.ReadFull(rxResp.Body, buf256)
	if err != nil || n != len(buf256) {
		t.Fatalf("RX không đọc được 256KB đầu: n=%d, err=%v", n, err)
	}
	if !bytes.Equal(buf256, first256KB) {
		t.Errorf("256KB đầu tiên không khớp!")
	}

	// 4. Giờ TX mới bơm nốt 768KB còn lại và đóng pipe hoàn tất upload
	_, _ = pw.Write(rest768KB)
	_ = pw.Close()

	if uploadErr := <-uploadErrChan; uploadErr != nil {
		t.Fatalf("Upload thất bại: %v", uploadErr)
	}

	// 5. RX đọc nốt phần còn lại
	bufRest, err := io.ReadAll(rxResp.Body)
	if err != nil || len(bufRest) != len(rest768KB) {
		t.Fatalf("RX đọc phần còn lại thất bại: len=%d, err=%v", len(bufRest), err)
	}
	if !bytes.Equal(bufRest, rest768KB) {
		t.Errorf("768KB còn lại không khớp!")
	}
}

func TestParallelChunkPipelining(t *testing.T) {
	handler := createHandler()
	ts := httptest.NewServer(handler)
	defer ts.Close()

	slug := "turbopipe"
	chunkSize := 512 * 1024 // 512KB mỗi chunk
	totalChunks := 3
	totalSize := int64(chunkSize * totalChunks)

	chunk0Data := bytes.Repeat([]byte("0"), chunkSize)
	chunk1Data := bytes.Repeat([]byte("1"), chunkSize)
	chunk2Data := bytes.Repeat([]byte("2"), chunkSize)

	// 1. Nạp trước chunk 0 (bước khởi tạo phiên)
	req0, _ := http.NewRequest("POST", fmt.Sprintf("%s/%s-s/chunk?conn=tx_pipe", ts.URL, slug), bytes.NewReader(chunk0Data))
	req0.Header.Set("X-Session-ID", "sess_pipeline")
	req0.Header.Set("X-File-Name", "pipeline.bin")
	req0.Header.Set("X-File-Size", fmt.Sprintf("%d", totalSize))
	req0.Header.Set("X-Chunk-Index", "0")
	req0.Header.Set("X-Total-Chunks", "3")
	req0.Header.Set("X-Chunk-Size", fmt.Sprintf("%d", chunkSize))

	resp0, err := http.DefaultClient.Do(req0)
	if err != nil || resp0.StatusCode != http.StatusOK {
		t.Fatalf("Nạp chunk 0 lỗi: %v", err)
	}
	resp0.Body.Close()

	// 2. RX cắm vào đọc luồng
	rxDataChan := make(chan []byte, 1)
	rxErrChan := make(chan error, 1)
	go func() {
		rxReq, _ := http.NewRequest("GET", fmt.Sprintf("%s/%s-r?conn=rx_pipe", ts.URL, slug), nil)
		rxResp, err := http.DefaultClient.Do(rxReq)
		if err != nil {
			rxErrChan <- err
			return
		}
		defer rxResp.Body.Close()
		allData, err := io.ReadAll(rxResp.Body)
		if err != nil {
			rxErrChan <- err
			return
		}
		rxDataChan <- allData
	}()

	// 3. TX nạp song song 2 luồng (Chunk 1 và Chunk 2 cùng lúc!)
	var wg sync.WaitGroup
	wg.Add(2)
	uploadErr := make(chan error, 2)

	go func() {
		defer wg.Done()
		req1, _ := http.NewRequest("POST", fmt.Sprintf("%s/%s-s/chunk?conn=tx_pipe", ts.URL, slug), bytes.NewReader(chunk1Data))
		req1.Header.Set("X-Session-ID", "sess_pipeline")
		req1.Header.Set("X-File-Name", "pipeline.bin")
		req1.Header.Set("X-File-Size", fmt.Sprintf("%d", totalSize))
		req1.Header.Set("X-Chunk-Index", "1")
		req1.Header.Set("X-Total-Chunks", "3")
		req1.Header.Set("X-Chunk-Size", fmt.Sprintf("%d", chunkSize))
		r1, e1 := http.DefaultClient.Do(req1)
		if e1 != nil || r1.StatusCode != http.StatusOK {
			uploadErr <- fmt.Errorf("chunk 1 upload err: %v", e1)
			return
		}
		r1.Body.Close()
	}()

	go func() {
		defer wg.Done()
		req2, _ := http.NewRequest("POST", fmt.Sprintf("%s/%s-s/chunk?conn=tx_pipe", ts.URL, slug), bytes.NewReader(chunk2Data))
		req2.Header.Set("X-Session-ID", "sess_pipeline")
		req2.Header.Set("X-File-Name", "pipeline.bin")
		req2.Header.Set("X-File-Size", fmt.Sprintf("%d", totalSize))
		req2.Header.Set("X-Chunk-Index", "2")
		req2.Header.Set("X-Total-Chunks", "3")
		req2.Header.Set("X-Chunk-Size", fmt.Sprintf("%d", chunkSize))
		r2, e2 := http.DefaultClient.Do(req2)
		if e2 != nil || r2.StatusCode != http.StatusOK {
			uploadErr <- fmt.Errorf("chunk 2 upload err: %v", e2)
			return
		}
		r2.Body.Close()
	}()

	wg.Wait()
	close(uploadErr)
	for e := range uploadErr {
		t.Fatalf("Lỗi song song upload: %v", e)
	}

	// 4. Kiểm tra RX nhận đúng toàn vẹn thứ tự byte
	select {
	case err := <-rxErrChan:
		t.Fatalf("RX kéo lỗi: %v", err)
	case received := <-rxDataChan:
		expected := append(append(chunk0Data, chunk1Data...), chunk2Data...)
		if !bytes.Equal(received, expected) {
			t.Errorf("Dữ liệu nhận từ Pipelining 2 luồng không khớp bản gốc!")
		}
	case <-time.After(5 * time.Second):
		t.Fatalf("Timeout chờ RX kéo xong")
	}
}

func TestMidChunkRetryDeduplication(t *testing.T) {
	handler := createHandler()
	ts := httptest.NewServer(handler)
	defer ts.Close()

	slug := "turboretry"
	chunkSize := 1024 * 1024 // 1MB
	totalChunks := 2
	totalSize := int64(chunkSize * totalChunks)

	chunk0Data := bytes.Repeat([]byte("0"), chunkSize)
	chunk1SubA := bytes.Repeat([]byte("A"), 256*1024)
	chunk1SubB := bytes.Repeat([]byte("B"), 768*1024)
	chunk1Full := append(chunk1SubA, chunk1SubB...)

	// 1. Nạp Chunk 0
	req0, _ := http.NewRequest("POST", fmt.Sprintf("%s/%s-s/chunk?conn=tx_retry", ts.URL, slug), bytes.NewReader(chunk0Data))
	req0.Header.Set("X-Session-ID", "sess_retry")
	req0.Header.Set("X-File-Name", "retry.bin")
	req0.Header.Set("X-File-Size", fmt.Sprintf("%d", totalSize))
	req0.Header.Set("X-Chunk-Index", "0")
	req0.Header.Set("X-Total-Chunks", "2")
	req0.Header.Set("X-Chunk-Size", fmt.Sprintf("%d", chunkSize))

	resp0, err := http.DefaultClient.Do(req0)
	if err != nil || resp0.StatusCode != http.StatusOK {
		t.Fatalf("Nạp chunk 0 lỗi: %v", err)
	}
	resp0.Body.Close()

	// 2. RX bắt đầu đọc
	rxDataChan := make(chan []byte, 1)
	rxErrChan := make(chan error, 1)
	go func() {
		rxReq, _ := http.NewRequest("GET", fmt.Sprintf("%s/%s-r?conn=rx_retry", ts.URL, slug), nil)
		rxResp, err := http.DefaultClient.Do(rxReq)
		if err != nil {
			rxErrChan <- err
			return
		}
		defer rxResp.Body.Close()
		all, err := io.ReadAll(rxResp.Body)
		if err != nil {
			rxErrChan <- err
			return
		}
		rxDataChan <- all
	}()

	// 3. TX bắt đầu nạp Chunk 1 qua pipe, chỉ bơm 256KB đầu rồi ngắt kết nối giả lập đứt mạng
	pr1, pw1 := io.Pipe()
	go func() {
		_, _ = pw1.Write(chunk1SubA)
		time.Sleep(50 * time.Millisecond)
		_ = pw1.CloseWithError(fmt.Errorf("simulated network drop mid-chunk"))
	}()

	req1Fail, _ := http.NewRequest("POST", fmt.Sprintf("%s/%s-s/chunk?conn=tx_retry", ts.URL, slug), pr1)
	req1Fail.Header.Set("X-Session-ID", "sess_retry")
	req1Fail.Header.Set("X-File-Name", "retry.bin")
	req1Fail.Header.Set("X-File-Size", fmt.Sprintf("%d", totalSize))
	req1Fail.Header.Set("X-Chunk-Index", "1")
	req1Fail.Header.Set("X-Total-Chunks", "2")
	req1Fail.Header.Set("X-Chunk-Size", fmt.Sprintf("%d", chunkSize))
	_, _ = http.DefaultClient.Do(req1Fail)

	// Đợi 50ms để server xả 256KB xuống socket của RX và cập nhật FlushedTotalBytes
	time.Sleep(50 * time.Millisecond)

	// 4. TX retry lại trọn vẹn Chunk 1 từ byte 0
	req1Retry, _ := http.NewRequest("POST", fmt.Sprintf("%s/%s-s/chunk?conn=tx_retry", ts.URL, slug), bytes.NewReader(chunk1Full))
	req1Retry.Header.Set("X-Session-ID", "sess_retry")
	req1Retry.Header.Set("X-File-Name", "retry.bin")
	req1Retry.Header.Set("X-File-Size", fmt.Sprintf("%d", totalSize))
	req1Retry.Header.Set("X-Chunk-Index", "1")
	req1Retry.Header.Set("X-Total-Chunks", "2")
	req1Retry.Header.Set("X-Chunk-Size", fmt.Sprintf("%d", chunkSize))

	respRetry, err := http.DefaultClient.Do(req1Retry)
	if err != nil {
		t.Fatalf("Retry chunk 1 err: %v", err)
	}
	retryBody, _ := io.ReadAll(respRetry.Body)
	respRetry.Body.Close()
	if respRetry.StatusCode != http.StatusOK {
		t.Fatalf("Retry chunk 1 status: %d, body: %s", respRetry.StatusCode, string(retryBody))
	}

	// 5. Kiểm tra dữ liệu RX nhận được: Không bị trùng lặp 256KB đầu, dữ liệu khớp 100%
	select {
	case err := <-rxErrChan:
		t.Fatalf("RX kéo lỗi: %v", err)
	case received := <-rxDataChan:
		expected := append(chunk0Data, chunk1Full...)
		if len(received) != len(expected) {
			t.Fatalf("Độ dài nhận được %d != kỳ vọng %d (Có thể bị lặp byte do retry!)", len(received), len(expected))
		}
		if !bytes.Equal(received, expected) {
			t.Errorf("Dữ liệu sau khi retry bị lệch byte!")
		}
	case <-time.After(5 * time.Second):
		t.Fatalf("Timeout chờ RX kéo xong")
	}
}

func TestMobileNativeDownloadHeadersAndWakeLock(t *testing.T) {
	handler := createHandler()
	ts := httptest.NewServer(handler)
	defer ts.Close()

	slug := "mobiledl"
	fileName := "video_sample_clip.mp4"
	chunkSize := 512 * 1024 // 512KB
	totalChunks := 2
	totalSize := int64(chunkSize * totalChunks)
	testData := bytes.Repeat([]byte("M"), int(totalSize))

	// 1. Upload chunk 0
	req0, _ := http.NewRequest("POST", fmt.Sprintf("%s/%s-s/chunk?conn=tx_mob", ts.URL, slug), bytes.NewReader(testData[:chunkSize]))
	req0.Header.Set("X-Session-ID", "sess_mob")
	req0.Header.Set("X-File-Name", fileName)
	req0.Header.Set("X-File-Size", fmt.Sprintf("%d", totalSize))
	req0.Header.Set("X-Chunk-Index", "0")
	req0.Header.Set("X-Total-Chunks", fmt.Sprintf("%d", totalChunks))
	req0.Header.Set("X-Chunk-Size", fmt.Sprintf("%d", chunkSize))
	r0, err := http.DefaultClient.Do(req0)
	if err != nil || r0.StatusCode != http.StatusOK {
		t.Fatalf("Upload chunk 0 err: %v", err)
	}
	r0.Body.Close()

	// 2. Giả lập trình duyệt Mobile Safari / Chrome trigger Native Download:
	// GET /mobiledl-r?conn=c_mobile123 với User-Agent Mobile
	rxDataChan := make(chan []byte, 1)
	rxHeadersChan := make(chan http.Header, 1)
	go func() {
		rxReq, _ := http.NewRequest("GET", fmt.Sprintf("%s/%s-r?conn=c_mobile123", ts.URL, slug), nil)
		rxReq.Header.Set("User-Agent", "Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Mobile/15E148 Safari/604.1")
		resp, rxErr := http.DefaultClient.Do(rxReq)
		if rxErr != nil {
			t.Errorf("Native download request error: %v", rxErr)
			return
		}
		defer resp.Body.Close()
		rxHeadersChan <- resp.Header
		data, _ := io.ReadAll(resp.Body)
		rxDataChan <- data
	}()

	// 3. Upload chunk 1
	req1, _ := http.NewRequest("POST", fmt.Sprintf("%s/%s-s/chunk?conn=tx_mob", ts.URL, slug), bytes.NewReader(testData[chunkSize:]))
	req1.Header.Set("X-Session-ID", "sess_mob")
	req1.Header.Set("X-File-Name", fileName)
	req1.Header.Set("X-File-Size", fmt.Sprintf("%d", totalSize))
	req1.Header.Set("X-Chunk-Index", "1")
	req1.Header.Set("X-Total-Chunks", fmt.Sprintf("%d", totalChunks))
	req1.Header.Set("X-Chunk-Size", fmt.Sprintf("%d", chunkSize))
	r1, err := http.DefaultClient.Do(req1)
	if err != nil || r1.StatusCode != http.StatusOK {
		t.Fatalf("Upload chunk 1 err: %v", err)
	}
	r1.Body.Close()

	// 4. Kiểm tra headers để đảm bảo OS Download Manager nhận diện đúng file download
	select {
	case headers := <-rxHeadersChan:
		disp := headers.Get("Content-Disposition")
		if !strings.Contains(disp, "attachment") || !strings.Contains(disp, fileName) {
			t.Errorf("Content-Disposition không chuẩn Native Download: %q", disp)
		}
		if headers.Get("Content-Type") != "application/octet-stream" {
			t.Errorf("Content-Type kỳ vọng application/octet-stream, nhận %q", headers.Get("Content-Type"))
		}
		if headers.Get("Content-Length") != fmt.Sprintf("%d", totalSize) {
			t.Errorf("Content-Length kỳ vọng %d, nhận %q", totalSize, headers.Get("Content-Length"))
		}
	case <-time.After(3 * time.Second):
		t.Fatalf("Timeout chờ headers Native Download")
	}

	// 5. Kiểm tra toàn vẹn dữ liệu
	select {
	case dlData := <-rxDataChan:
		if !bytes.Equal(dlData, testData) {
			t.Errorf("Dữ liệu Native Download không khớp!")
		}
	case <-time.After(3 * time.Second):
		t.Fatalf("Timeout chờ dữ liệu Native Download")
	}
}

func TestShortlinkGatewayRegistration(t *testing.T) {
	// 1. Mock Shortlink Gateway HTTP Server
	mockGateway := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/register" && r.Method == http.MethodPost {
			var body map[string]interface{}
			_ = json.NewDecoder(r.Body).Decode(&body)
			slug := body["slug"].(string)

			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"status":  "ok",
				"slug":    slug,
				"short_r": fmt.Sprintf("https://s.wd.io.vn/%s-r", slug),
				"short_s": fmt.Sprintf("https://s.wd.io.vn/%s-s", slug),
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer mockGateway.Close()

	// 2. Test đăng ký thành công
	shortR, shortS := registerShortlinkWithGateway(mockGateway.URL, "vip99", "https://rapidly-mock-tunnel.trycloudflare.com")
	if shortR != "https://s.wd.io.vn/vip99-r" || shortS != "https://s.wd.io.vn/vip99-s" {
		t.Errorf("Đăng ký shortlink thất bại: shortR=%q, shortS=%q", shortR, shortS)
	}

	// 3. Test trường hợp Gateway bị lỗi hoặc sập (Fallback an toàn, không panic)
	failedR, failedS := registerShortlinkWithGateway("http://127.0.0.1:59999", "vip99", "https://rapidly-mock-tunnel.trycloudflare.com")
	if failedR != "" || failedS != "" {
		t.Errorf("Kỳ vọng trả về rỗng khi gateway sập, nhận shortR=%q, shortS=%q", failedR, failedS)
	}

	// 4. Test khi cấu hình 'none' (Tắt shortlink)
	noneR, noneS := registerShortlinkWithGateway("none", "vip99", "https://rapidly-mock-tunnel.trycloudflare.com")
	if noneR != "" || noneS != "" {
		t.Errorf("Kỳ vọng trả về rỗng khi gateway='none', nhận shortR=%q, shortS=%q", noneR, noneS)
	}
}

func TestTerminalPipeArgsParsing(t *testing.T) {
	// Case 1: 'wiredrop 88' -> TX, slug 88
	opt1 := parseTerminalPipeArgs([]string{"88"})
	if opt1.Slug != "88" || opt1.IsReceiver != false || opt1.UseShortlink != false || opt1.OpenWeb != false {
		t.Errorf("Case 1 lỗi: %+v", opt1)
	}

	// Case 2: 'wiredrop 88 -r' -> RX, slug 88
	opt2 := parseTerminalPipeArgs([]string{"88", "-r"})
	if opt2.Slug != "88" || opt2.IsReceiver != true {
		t.Errorf("Case 2 lỗi: %+v", opt2)
	}

	// Case 3: 'wiredrop 88 -s' -> TX, slug 88, shortlink true
	opt3 := parseTerminalPipeArgs([]string{"88", "-s"})
	if opt3.Slug != "88" || opt3.UseShortlink != true {
		t.Errorf("Case 3 lỗi: %+v", opt3)
	}

	// Case 4: 'wiredrop 88 -w' -> Web browser true
	opt4 := parseTerminalPipeArgs([]string{"88", "-w"})
	if opt4.Slug != "88" || opt4.OpenWeb != true {
		t.Errorf("Case 4 lỗi: %+v", opt4)
	}

	// Case 5: 'wiredrop open testroom --rx --short'
	opt5 := parseTerminalPipeArgs([]string{"open", "testroom", "--rx", "--short"})
	if opt5.Slug != "testroom" || opt5.IsReceiver != true || opt5.UseShortlink != true {
		t.Errorf("Case 5 lỗi: %+v", opt5)
	}

	// Case 6: 'wiredrop 99-r' -> Tự nhận diện hậu tố -r
	opt6 := parseTerminalPipeArgs([]string{"99-r"})
	if opt6.Slug != "99" || opt6.IsReceiver != true {
		t.Errorf("Case 6 lỗi: %+v", opt6)
	}

	// Case 7: 'wiredrop 99-s' -> Tự nhận diện hậu tố -s
	opt7 := parseTerminalPipeArgs([]string{"99-s"})
	if opt7.Slug != "99" || opt7.IsReceiver != false {
		t.Errorf("Case 7 lỗi: %+v", opt7)
	}
}

func TestAPIShortenEndpoint(t *testing.T) {
	// 1. Mock Gateway
	mockGateway := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		_ = json.NewDecoder(r.Body).Decode(&body)
		slug := body["slug"].(string)

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "ok",
			"slug":    slug,
			"short_r": fmt.Sprintf("https://s.wd.io.vn/%s-r", slug),
			"short_s": fmt.Sprintf("https://s.wd.io.vn/%s-s", slug),
		})
	}))
	defer mockGateway.Close()

	os.Setenv("SHORTLINK_GATEWAY", mockGateway.URL)
	defer os.Unsetenv("SHORTLINK_GATEWAY")

	server := httptest.NewServer(createHandler())
	defer server.Close()

	reqBody := `{"slug":"room77","destination":"` + server.URL + `"}`
	resp, err := http.Post(server.URL+"/api/shorten", "application/json", strings.NewReader(reqBody))
	if err != nil {
		t.Fatalf("Lỗi gọi /api/shorten: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Kỳ vọng HTTP 200, nhận %d", resp.StatusCode)
	}

	var res struct {
		Status   string `json:"status"`
		Slug     string `json:"slug"`
		ShortR   string `json:"short_r"`
		Fallback bool   `json:"fallback"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		t.Fatalf("Lỗi decode JSON: %v", err)
	}

	if res.Status != "ok" || res.ShortR != "https://s.wd.io.vn/room77-r" || res.Fallback != false {
		t.Errorf("Kết quả /api/shorten không khớp: %+v", res)
	}
}

func TestSenderClearSlotCommand(t *testing.T) {
	server := httptest.NewServer(createHandler())
	defer server.Close()

	slug := "cleartest"
	// 1. Nạp text vào phòng
	postResp, err := http.Post(fmt.Sprintf("%s/%s-s", server.URL, slug), "text/plain", strings.NewReader("sample script data"))
	if err != nil || postResp.StatusCode != http.StatusOK {
		t.Fatalf("Lỗi nạp text: %v", err)
	}
	postResp.Body.Close()

	// 2. Gửi tín hiệu clear
	clearResp, err := http.Post(fmt.Sprintf("%s/%s-s?clear=1", server.URL, slug), "text/plain", nil)
	if err != nil || clearResp.StatusCode != http.StatusOK {
		t.Fatalf("Lỗi gửi clear: %v", err)
	}
	body, _ := io.ReadAll(clearResp.Body)
	clearResp.Body.Close()

	if !strings.Contains(string(body), "CLEARED") {
		t.Errorf("Kỳ vọng trả về CLEARED, nhận %q", string(body))
	}

	// 3. Đọc từ RX kiểm tra phòng đã rỗng
	rxResp, err := http.Get(fmt.Sprintf("%s/%s-r", server.URL, slug))
	if err != nil || rxResp.StatusCode != http.StatusOK {
		t.Fatalf("Lỗi đọc RX: %v", err)
	}
	rxBody, _ := io.ReadAll(rxResp.Body)
	rxResp.Body.Close()

	if !strings.Contains(string(rxBody), "(empty)") {
		t.Errorf("Phòng chưa rỗng sau khi clear: %q", string(rxBody))
	}
}

