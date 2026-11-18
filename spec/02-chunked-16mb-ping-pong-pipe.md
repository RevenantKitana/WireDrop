# ĐẶC TẢ KIẾN TRÚC: TRUYỀN FILE LỚN QUA BỘ ĐỆM ĐÔI 16MB CÓ ĐIỀU KHIỂN LUỒNG NGƯỢC
# (16MB BACKPRESSURE PING-PONG BUFFER PIPE SPECIFICATION)
# HỆ THỐNG: WireDrop v0.6.0+

**Tác giả:** Đội ngũ Kỹ sư WireDrop  
**Trạng thái:** Đặc Tả Kiến Trúc Chính Thức (Approved Architectural Specification)  
**Ngày phê duyệt:** 2026-11-17  
**Phiên bản:** v2.0  
**Phạm vi áp dụng:** Go Backend (`handler_pipe.go`, `room.go`), Web Client SPA (`app.js`, `index.html`, `style.css`), CLI Tooling (`curl`, `irm`, `fasthost`).

---

## 1. TỔNG QUAN & NGUYÊN LÝ THIẾT KẾ (DESIGN PHILOSOPHY)

Bản đặc tả này nâng cấp toàn diện cơ chế truyền file của **WireDrop**, giải quyết triệt để 3 bài toán lớn:
1. **Khống chế tuyệt đối tài nguyên Server:** Bộ nhớ RAM tiêu thụ cho mỗi luồng truyền file không vượt quá **32 MB** ($2 \times 16\text{MB}$), lưu trữ ổ cứng máy chủ = **0 Byte**.
2. **Vượt qua giới hạn 100MB của Cloudflare:** Mọi request tải lên đều được đóng gói thành các khối **16 MB**, loại bỏ hoàn toàn lỗi `413 Request Entity Too Large` của Cloudflare Quick/Free Tunnel.
3. **Cơ chế Điều Khiển Luồng Ngược (Backpressure Flow Control):** Phía gửi (TX) chỉ nạp tối đa **Chunk 0 (16MB)** vào RAM. Nếu phía nhận (RX) chưa cắm dây đón, quá trình nạp sẽ **TẠM DỪNG (PAUSE)**, triệt tiêu 100% lãng phí băng thông và tài nguyên.

```text
[MÁY GỬI (TX)]                   [SERVER WIREDROP]                  [MÁY NHẬN (RX)]
  Web / CLI                   (RAM Cap: Tối đa 32MB)                Web / curl
      │                                │                                 │
      │── 1. Nạp Chunk 0 (16MB) ──────>│ [Slot A: 16MB]                  │
      │                                │                                 │
      │   [TX DỪNG, CHỜ RX ĐÓN]        │                                 │
      │   "Đang chờ RX nhận..."        │                                 │
      │                                │                                 │
      │                                │<── 2. RX cắm dây bắt đầu kéo ───┤
      │                                │    (Kéo Slot A)                 │
      │                                │                                 │
      │<── 3. Tín hiệu Mở Slot B ──────┤                                 │
      │                                │                                 │
      │── 4. Bơm trước Chunk 1 (16MB) ─>│ [Slot B: 16MB]                 │
      │   (Trong lúc RX đang kéo A)    │                                 │
      │                                │── 5. Xả Slot B sang RX ────────>│
      │                                │    (Ngay khi A xả xong)         │
      │   ... Lặp lại Ping-Pong Buffer liên tục đến khi hết file ...     │
```

---

## 2. BỘ THÔNG SỐ VÀNG CỐ ĐỊNH (GOLDEN PARAMETERS)

| Thông Số | Giá Trị Chuẩn | Diễn Giải & Cơ Sở Kỹ Thuật |
| :--- | :---: | :--- |
| **Kích thước Khối (Chunk Size)** | **16 MB** ($16,777,216$ bytes) | Bội số nhị phân chuẩn $2^{24}$ bytes. Tối ưu hóa phân bổ bộ nhớ OS (Kernel memory page alignment). Nằm an toàn dưới trần 100MB của Cloudflare. |
| **Kích thước Cửa Sổ (Window Size)** | **2 Slots (Ping-Pong)** | Đệm đôi: 1 slot đang phục vụ RX, 1 slot đón trước từ TX. Triệt tiêu thời gian chết (zero dead-time), tốc độ đạt tối đa công suất mạng. |
| **Trần RAM Server tối đa** | **32 MB** / phòng | $2 \times 16\text{MB}$. Server không bao giờ tốn quá 32MB RAM cho 1 phiên truyền dù file 10GB hay 50GB. |
| **Lưu trữ ổ cứng Server** | **0 Byte** | Dữ liệu luân chuyển trực tiếp qua bộ đệm RAM rồi xả sang client, không ghi đĩa tạm. |
| **Thời gian chờ RX (Chunk 0 Idle Timeout)** | **10 Phút** (600 giây) | Nếu TX nạp xong Chunk 0 mà sau 10 phút RX không vào cắm dây nhận, server tự động hủy phiên để giải phóng RAM. |
| **Thời gian chờ truyền 1 Chunk (Stall Timeout)**| **45 Giây** | Nếu một chunk đang truyền bị nghẽn hoặc đứng im quá 45 giây, hệ thống kích hoạt cơ chế xử lý lỗi. |
| **Số lần tự động thử lại (Retry Count)** | **3 Lần / Chunk** | Tự động thử lại chunk bị lỗi mạng theo cấp số nhân ($1\text{s}, 2\text{s}, 4\text{s}$) trước khi kết luận đứt kết nối. |
| **Nhịp tim giữ kết nối (SSE Heartbeat)** | **10 Giây** | Gửi comment `: heartbeat\n\n` định kỳ để ngăn chặn Cloudflare Edge hoặc Proxy ngắt kết nối nhàn rỗi. |

---

## 3. THIẾT KẾ GIAO THỨC & API ENDPOINTS

### 3.1. Phân loại Tầng Tải lên (Upload Tiers)

1. **Tier 1 (File Nhỏ $\le 16\text{MB}$):**
   - Giữ nguyên cơ chế kinh điển của WireDrop: Nạp trọn vẹn 1 lần trong RAM qua `POST /{room}-s`.
   - Phục vụ tức thì qua `GET /{room}-r`.
2. **Tier 2 (File Lớn $> 16\text{MB}$):**
   - Kích hoạt giao thức **16MB Chunked Dual-Buffering Pipe**.

### 3.2. Endpoint Nạp Chunk: `POST /{room}-s/chunk`

Mỗi khối 16MB được gửi lên dưới dạng HTTP POST nhị phân thuần túy (`application/octet-stream`):

#### Headers bắt buộc:
* `X-Session-ID`: Chuỗi định danh duy nhất của phiên truyền (ví dụ: `sess_pxtq_1731844000`).
* `X-File-Name`: Tên file gốc (đã URL-encoded UTF-8).
* `X-File-Size`: Tổng dung lượng file gốc tính bằng byte (ví dụ: `1073741824` cho 1GB).
* `X-Chunk-Index`: Số thứ tự của chunk, bắt đầu từ `0` (ví dụ: `0`, `1`, `2`...).
* `X-Total-Chunks`: Tổng số chunk của file (ví dụ: `64`).
* `X-Chunk-Size`: Kích thước thực tế của chunk hiện tại (16777216 bytes, chunk cuối cùng có thể nhỏ hơn).
* `X-Chunk-Checksum`: Mã băm CRC32 hoặc SHA-256 của chunk để kiểm tra toàn vẹn dữ liệu.

#### Phản hồi từ Server:
* **`200 OK` (Sẵn sàng nhận tiếp):**
  ```json
  {
    "status": "ok",
    "session_id": "sess_pxtq_1731844000",
    "received_chunk": 0,
    "next_allowed": 1,
    "window_available": true
  }
  ```
* **`202 Accepted` (Buffer đầy, yêu cầu TX chờ RX kéo bớt):**
  Khi cả 2 slot RAM đều đang chứa dữ liệu chưa được RX đọc hết, server trả về cờ yêu cầu TX tạm ngừng:
  ```json
  {
    "status": "waiting_rx",
    "received_chunk": 1,
    "window_available": false
  }
  ```
* **`409 Conflict`:** Phòng đang bị chiếm bởi kết nối khác hoặc Session ID không khớp.
* **`410 Gone`:** Bên nhận RX đã đóng kết nối hoặc hủy tải giữa chừng.

---

### 3.3. Endpoint Kéo File: `GET /{room}-r`

#### Trường hợp A: Bên Nhận dùng Web SPA (File System Access API)
1. Khi bấm `[TẢI FILE VỀ MÁY]`, Web SPA gọi `GET /{room}/metadata` để lấy tên file và tổng dung lượng.
2. Trình duyệt hiển thị hộp thoại chọn thư mục lưu: `const handle = await window.showSaveFilePicker({ suggestedName: filename })`.
3. Mở luồng ghi trực tiếp: `const writable = await handle.createWritable()`.
4. Web RX thực hiện kéo từng chunk tuần tự: `GET /{room}-r/chunk?session=...&index=k`:
   * Nhận chunk $k \rightarrow$ Gọi `await writable.write(chunkBuffer)`.
   * Giải phóng `chunkBuffer` ngay lập tức khỏi bộ nhớ RAM của trình duyệt.
   * Gửi ACK báo server đã nhận xong chunk $k$.
5. Khi nhận đủ $N$ chunks $\rightarrow$ Gọi `await writable.close()`. Hoàn tất tải file an toàn tuyệt đối.

*Chế độ dự phòng (Fallback):* Nếu trình duyệt (Safari hoặc bản cũ) không có `showSaveFilePicker`, hệ thống kiểm tra nếu file $< 1\text{GB}$ thì gom `Blob` trong RAM; nếu file $\ge 1\text{GB}$ thì hiển thị thông báo khuyến nghị dùng Chrome/Edge hoặc chuyển sang dùng lệnh CLI `curl`.

#### Trường hợp B: Bên Nhận dùng lệnh CLI (`curl` / `PowerShell`)
* Người dùng chỉ gõ đúng 1 dòng lệnh duy nhất:
  ```bash
  curl -sLJO http://domain.com/room-r
  ```
* **Cơ chế Aggregator Stream tại Server:**
  * Server giữ kết nối HTTP của `curl` ở dạng luồng mở (`Transfer-Encoding: chunked` hoặc `Content-Length: X-File-Size`).
  * Server dùng `io.Pipe()` nội bộ:
    * Khi TX nạp Chunk 0 vào RAM $\rightarrow$ Server ghi Chunk 0 vào pipe xả sang `curl`.
    * Khi `curl` đọc xong Chunk 0 $\rightarrow$ Server lập tức phát tín hiệu ACK cho TX nạp tiếp Chunk 1.
    * Tiếp tục bơm liên tục cho tới khi đủ 100% dung lượng.
  * Phía người dùng `curl` hoàn toàn không nhận thấy sự ngắt quãng, file tải về trơn tru như một file tải đơn khối thông thường!

---

## 4. MA TRẬN XỬ LÝ MỌI TRƯỜNG HỢP BIÊN (EXHAUSTIVE EDGE CASES)

### Case 1: File nhỏ $\le 16\text{MB}$
* **Xử lý:** Chạy theo Tier 1 cổ điển. Nạp nguyên khối vào RAM server trong 1 request. Phục vụ ngay lập tức cho bất kỳ bên nào vào sau.

### Case 2: File lớn $> 16\text{MB}$ khi RX đã mở trang / cắm dây sẵn
* **Xử lý:** Kích hoạt chế độ **Full-Duplex Ping-Pong**. 
  * TX nạp Chunk 0 $\rightarrow$ Server báo RX kéo Chunk 0.
  * Trong lúc RX đang kéo Chunk 0, TX tranh thủ nạp trước Chunk 1 vào Slot B.
  * RX vừa kéo xong Chunk 0 thì Chunk 1 đã nằm sẵn trong RAM, RX kéo tiếp không có độ trễ nghỉ.

### Case 3: File lớn $> 16\text{MB}$ khi RX chưa vào cắm dây (Backpressure kích hoạt)
* **Xử lý:** 
  * TX nạp xong Chunk 0 (16MB) lên server.
  * Server nhận thấy phòng chưa có kết nối RX $\rightarrow$ Không cấp phát thêm slot.
  * TX nhận phản hồi: Tạm dừng upload, hiển thị thông báo trên Web TX:  
    `[⏸ ĐÃ SẴN SÀNG CHUNK 0 (16MB) • CHỜ BÊN NHẬN MỞ LINK ĐỂ BƠM TIẾP...]`
  * Đồng hồ đếm ngược **10 phút (Idle Hold Timeout)** bắt đầu chạy.
  * Nếu quá 10 phút RX không vào, Server xóa Chunk 0, TX báo `[HẾT THỜI GIAN CHỜ RX]`.

### Case 4: RX vào cắm dây sau khi TX đã tạm dừng tại Chunk 0
* **Xử lý:**
  * Ngay khi RX mở link `/{room}-r` và bấm nhận, Server lập tức phát sự kiện SSE `rx_connected` sang cho TX.
  * RX bắt đầu tải Chunk 0.
  * Server phát ACK cho TX $\rightarrow$ TX tự động mở khóa (Unpause) và bắt đầu nạp Chunk 1. Quá trình truyền tiếp diễn trơn tru.

### Case 5: TX bị tắt tab / mất kết nối giữa chừng (ví dụ ở chunk 14/50)
* **Xử lý:**
  * Sau **45 giây (Stall Timeout)** không thấy TX gửi tiếp chunk 15, Server phát sự kiện SSE sang RX: `tx_disconnected`.
  * Phía Web RX đóng luồng ghi file, hiển thị lỗi rõ ràng: `[LỖI: BÊN GỬI ĐÃ NGẮT DÂY GIỮA CHỪNG]`.
  * Server dọn dẹp sạch toàn bộ các chunk đang nằm trong RAM.

### Case 6: RX bị tắt tab / bấm Hủy tải giữa chừng
* **Xử lý:**
  * Kết nối kéo file của RX bị ngắt $\rightarrow$ Server phát hiện qua context `r.Context().Done()`.
  * Server phát sự kiện SSE sang TX: `rx_cancelled`.
  * Phía Web TX dừng ngay lập tức việc đọc file từ đĩa và dừng các request nạp tiếp theo, hiển thị: `[ĐÃ DỪNG: BÊN NHẬN ĐÃ HỦY TẢI]`.
  * Không tốn thêm bất kỳ byte băng thông nào của TX.

### Case 7: Mạng chập chờn / rớt gói tin (Network Glitch)
* **Xử lý:**
  * Khi 1 request chunk bị rớt kết nối hoặc checksum không khớp:
  * Client tự động kích hoạt **Retry Mechanism (tối đa 3 lần)**:
    - Lần 1: Thử lại sau 1 giây.
    - Lần 2: Thử lại sau 2 giây.
    - Lần 3: Thử lại sau 4 giây.
  * Nếu thành công: Quá trình truyền tiếp tục bình thường, người dùng không bị gián đoạn.
  * Nếu thất bại cả 3 lần: Báo lỗi kết nối mạng và dừng phiên.

### Case 8: Tự hủy an toàn (Safe Burn-After-Read)
* **Xử lý:**
  * Server chỉ đánh dấu hoàn tất và giải phóng hoàn toàn trạng thái phiên sau khi **Chunk cuối cùng ($N-1$) đã được RX tải về trọn vẹn 100% số byte**.
  * Sau khi hoàn tất: Phát sự kiện SSE `payload_cleared`, đưa phòng trở lại trạng thái rỗng an toàn.

---

## 5. THIẾT KẾ GIAO DIỆN WEB (UI/UX SPECIFICATION)

### 5.1. Bảng Đo Tiến Trình Nạp File Phía TX (Upload Dashboard)
* Khi file $> 16\text{MB}$, giao diện hiển thị bảng đồng hồ:
  ```text
  ┌──────────────────────────────────────────────────────────────┐
  │  ĐANG NẠP: ubuntu-24.04-desktop-amd64.iso                    │
  │  [████████████████████░░░░░░░░░░░░░░░░] 52%                  │
  │                                                              │
  │  • Tiến độ:   2.6 GB / 5.0 GB (Chunk 165 / 313)              │
  │  • Tốc độ:    18.4 MB/s                                      │
  │  • Dự kiến:   Còn ~2 phút 10 giây                            │
  │  • Trạng thái:[PING-PONG BUFFER: ĐANG TRUYỀN THỜI GIAN THỰC] │
  │                                                              │
  │  [✕ HỦY TẢI LÊN]                                             │
  └──────────────────────────────────────────────────────────────┘
  ```

### 5.2. Bảng Tiến Trình Phía RX (Download Dashboard)
* Hiển thị tương ứng tốc độ tải về, số chunk đã ghi xuống đĩa, thanh tiến trình % và nút `[✕ DỪNG TẢI]`.

---

## 6. KẾ HOẠCH TRIỂN KHAI THEO CÁC BƯỚC (STEP-BY-STEP IMPLEMENTATION ROADMAP)

* **Bước 1 (Backend Core):** Nâng cấp `room.go` với cấu trúc `ChunkBuffer` (Dual Slot 16MB Ping-Pong) và đồng bộ qua Go channel/mutex.
* **Bước 2 (Backend Endpoints):** Triển khai handler `POST /{room}-s/chunk` và luồng stream liên tục cho `GET /{room}-r`.
* **Bước 3 (Frontend Slicer & File System API):** Xây dựng module chia file 16MB trên `app.js` cho TX và ghi file trực tiếp bằng `FileSystemWritableFileStream` cho RX.
* **Bước 4 (Testing & Verification):** Viết unit/integration test trong `sync_test.go` mô phỏng truyền file 100MB+ qua các chunk 16MB và kiểm tra giới hạn RAM $\le 32\text{MB}$.
* **Bước 5 (Release v0.6.0):** Cập nhật changelog và đóng gói binary `wiredrop.exe` và `dist/wiredrop-portable.exe`.
