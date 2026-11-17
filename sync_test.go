package main

import (
	"bufio"
	"encoding/json"
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
	if ct := resp.Header.Get("Content-Type"); !strings.Contains(ct, "text/event-stream") {
		t.Errorf("Kỳ vọng Content-Type text/event-stream, nhận: %s", ct)
	}

	// Đọc chunk đầu tiên: phải có 2KB padding để phá đệm
	reader := bufio.NewReader(resp.Body)
	paddingLine, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("Không thể đọc dòng đệm padding: %v", err)
	}

	if !strings.HasPrefix(paddingLine, ": ") || len(paddingLine) < 2000 {
		t.Errorf("Kỳ vọng dòng padding dài >= 2000 bytes, nhận: %d bytes", len(paddingLine))
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
