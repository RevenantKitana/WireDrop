package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
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
