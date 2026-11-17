# TÀI LIỆU ĐẶC TẢ YÊU CẦU VÀ THIẾT KẾ KIẾN TRÚC HỆ THỐNG
# DỰ ÁN: WireDrop (TermPipe)
**Phiên bản:** v0.5.1 (Kiến Trúc Khóa Kết Nối Vật Lý Thuần Túy - Pure Physical Connection Lock)  
**Ngôn ngữ triển khai:** **Go (Golang 1.22+) – 100% Standard Library (Zero-Dependency)**  
**Trạng thái:** Chuẩn hóa Kiến trúc Bản Update (Ready for Implementation)  
**Ngày cập nhật:** 2026-11-16  

---

## MỤC LỤC
1. [Triết Lý Cốt Lõi: Mô Hình "Sợi Dây 2 Đầu Thuần 1 Chiều"](#1-triết-lý-cốt-lõi-mô-hình-sợi-dây-2-đầu-thuần-1-chiều)
2. [Cơ Chế Khóa Kết Nối Vật Lý Thuần Túy (Pure Physical Connection Lock)](#2-cơ-chế-khóa-kết-nối-vật-lý-thuần-túy-pure-physical-connection-lock)
3. [Đơn Vị Chuyển Đổi Nguyên Tử (Single Atomic Payload Slot)](#3-đơn-vị-chuyển-đổi-nguyên-tử-single-atomic-payload-slot)
4. [Phân Định Giao Diện Web Theo Đầu Cắm (`-s` vs `-r`)](#4-phân-định-giao-diện-web-theo-đầu-cắm--s-vs--r)
5. [Quy Chuẩn Dòng Lệnh Tối Giản Cho Máy B (Terminal)](#5-quy-chuẩn-dòng-lệnh-tối-giản-cho-máy-b-terminal)
6. [Chiến Lược Truyền File: Store-and-Forward vs Zero-Storage Pipe](#6-chiến-lược-truyền-file-store-and-forward-vs-zero-storage-pipe)
7. [Đặc Tả API & Định Tuyến (Go 1.22+ ServeMux)](#7-đặc-tả-api--định-tuyến-go-122-servemux)
8. [Bộ Lọc Phòng Vệ & Ổn Định Vận Hành](#8-bộ-lọc-phòng-vệ--ổn-định-vận-hành)
9. [Cấu Trúc Mã Nguồn & Kế Hoạch Cập Nhật](#9-cấu-trúc-mã-nguồn--kế-hoạch-cập-nhật)

---

## 1. TRIẾT LÝ CỐT LÕI: MÔ HÌNH "SỢI DÂY 2 ĐẦU THUẦN 1 CHIỀU"

Khắc phục hoàn toàn sự nhập nhằng của mô hình "Notepad dùng chung", WireDrop v4.0.0 tuân thủ triệt để nguyên lý: **Mỗi phòng là một đường ống 1 chiều có 2 đầu cắm riêng biệt**.

```
[ ĐẦU PHÁT: 1-s ] ══════════════ SỢI DÂY PHÒNG 1 ══════════════> [ ĐẦU NHẬN: 1-r ]
  (Máy A cầm phát)                                                   (Máy B cắm nhận)
```

### Các nguyên tắc bất di bất dịch:
1. **Bên Phát (Máy A - Đầu `*-s`):** Là Master duy nhất. Toàn quyền viết, sửa, xóa kịch bản hoặc nạp file.
2. **Bên Nhận (Máy B - Đầu `*-r`):** Là Worker / Consumer thuần túy. Chỉ có quyền **ĐỌC / NHẬN (Read-Only)**. Tuyệt đối không thể can thiệp hay sửa đổi nội dung của Master A.
3. **Chiều ngược lại (B muốn gửi về A):** **Mở một phòng khác độc lập!** (Ví dụ Máy B mở đầu phát `2-s`, Máy A cắm đầu nhận `2-r`). Không dùng chung hay làm bẩn đường ống cũ.

---

## 2. CƠ CHẾ KHÓA KẾT NỐI VẬT LÝ THUẦN TÚY (PURE PHYSICAL CONNECTION LOCK)

Trong phiên bản v4.0.0, hệ thống **bỏ hoàn toàn việc khóa bằng IP**. Thay vào đó, hệ thống quản lý theo **Kết nối mạng thực tế (Active Physical Connection)**:

```
                                  SỢI DÂY (ROOM 1)
                                         │
                 ┌───────────────────────┴───────────────────────┐
                 ▼                                               ▼
       [ ĐẦU PHÁT: 1-s ]                               [ ĐẦU NHẬN: 1-r ]
  Chỉ duy nhất 1 Active Conn                      Chỉ duy nhất 1 Active Conn
  ┌───────────────┬──────────────┐                ┌───────────────┬──────────────┐
  ▼               ▼              ▼                ▼               ▼              ▼
[Cửa sổ 1]   [Cửa sổ 2]      [CLI]             [Cửa sổ 1]   [Cửa sổ 2]      [CLI]
  ✅ GIỮ DÂY   ❌ 409 BUSY   ❌ 409 BUSY         ✅ GIỮ DÂY   ❌ 409 BUSY   ❌ 409 BUSY
```

### 2.1. Định nghĩa "Chiếm Giữ Đầu Dây" (Holding the Wire End)
* **Khóa theo kết nối mở (Connection Lock):** Mỗi đầu dây (`-s` và `-r`) chỉ chấp nhận **DUY NHẤT 1 kết nối mạng còn sống (Active HTTP/SSE/Stream Connection)** tại một thời điểm.
* **Xử lý mở Double Cửa Sổ trên cùng thiết bị:**
  * Nếu người dùng mở 2 tab Web trên cùng một trình duyệt (hoặc 2 cửa sổ khác nhau trên cùng máy tính):
    * Tab 1 kết nối trước $\rightarrow$ Chiếm giữ đầu dây.
    * Tab 2 kết nối sau $\rightarrow$ Server phát hiện đầu dây đang có kết nối sống $\rightarrow$ **Trả về ngay lập tức `409 Conflict: Line Busy`**.
    * Tab 2 hiển thị màn hình khóa với biểu tượng 🔒: *"Đầu dây đang bị chiếm bởi một cửa sổ khác! Vui lòng đóng cửa sổ kia để kết nối."*
* **Xử lý xung đột giữa Web và Terminal (Mutual Exclusion):**
  * **Nếu đang mở Web Nhận:** Cửa sổ Web giữ kết nối SSE sống. Bất kỳ lệnh CLI nào (`curl` hay `irm`) cố truy cập vào `1-r` sẽ nhận ngay mã `409 Line Busy: Receiver window is currently open. Close web window first.`
  * **Nếu muốn dùng CLI:** Chỉ cần đóng cửa sổ Web nhận. Lúc này kết nối TCP ngắt $\rightarrow$ server giải phóng đầu dây tức thì (`ctx.Done()`) $\rightarrow$ Terminal có thể cắm vào nhận lệnh/file ngay lập tức.
  * **Nếu Terminal chạy Live Stream (`curl -sN .../1-r | bash`):** Kết nối stream được giữ mở liên tục. Cửa sổ Web hoặc Terminal thứ hai mở song song sẽ bị chặn `409 Line Busy`.

### 2.2. Nhận Diện Kết Nối Tự Động (Auto Connection ID)
* Không bắt người dùng nhập mật khẩu hay token thủ công.
* Khi mỗi tab Web mở ra, ứng dụng tự sinh ngẫu nhiên một mã định danh kết nối nội bộ `conn_id` (lưu trong bộ nhớ RAM của tab).
* Khi tab mở kết nối giữ nhịp SSE `/api/status/:room-(s|r)?conn=conn_id`:
  * Nếu đầu dây trống: Server đăng ký `ActiveConnID = conn_id` và giám sát kênh `r.Context()`.
  * Nếu đầu dây đã có người giữ: Server ngắt kết nối với mã lỗi HTTP 409.
* Mọi thao tác nội bộ của tab đó (Auto-save POST, Upload file, Download file) gửi kèm `?conn=conn_id` để chứng minh quyền sở hữu hợp lệ của kết nối đang giữ dây.
* Khi đóng tab (hoặc tắt tiến trình): Kết nối mạng đứt, server giải phóng `ActiveConnID = ""` ngay lập tức trong mili-giây.

---

## 3. ĐƠN VỊ CHUYỂN ĐỔI NGUYÊN TỬ (SINGLE ATOMIC PAYLOAD SLOT)

Đường ống không lưu trữ phân mảnh nhiều file hay danh sách hỗn tạp. Tại bất kỳ thời điểm nào, một phòng chỉ chứa **ĐÚNG 1 VẬT PHẨM DUY NHẤT**:
* **Hoặc là: 1 đoạn Lệnh / Script / Text**.
* **Hoặc là: 1 File nhị phân duy nhất**.

```go
type PayloadType int

const (
    PayloadEmpty PayloadType = iota // Đường ống rỗng
    PayloadText                     // Đang chứa Text/Script
    PayloadFile                     // Đang chứa File
)

type PipeSlot struct {
    Type      PayloadType
    TextData  string    // Dùng nếu Type == PayloadText
    FileName  string    // Tên file (nếu Type == PayloadFile)
    FileData  []byte    // Buffer RAM (nếu file <= 15MB)
    TempPath  string    // Đường dẫn file tạm trên đĩa (nếu file > 15MB)
    FileSize  int64
    CreatedAt time.Time
}
```

* **Hành vi Nạp Mới (Replace):** Khi Bên A nạp nội dung mới vào `1-s` $\rightarrow$ Vật phẩm cũ trong ống tự động bị hủy và thay thế bằng vật phẩm mới.
* **Hành vi Rút Cạn (Burn):** Sau khi Máy B tải xong qua `1-r` $\rightarrow$ Vật phẩm tự động xóa khỏi bộ nhớ, đường ống trở về trạng thái Rỗng (`PayloadEmpty`).

---

## 4. PHÂN ĐỊNH GIAO DIỆN WEB THEO ĐẦU CẮM (`-s` vs `-r`)

Trình duyệt tự động mở giao diện chuyên biệt tùy theo URL:

### 4.1. Giao diện Đầu Phát (`domain.com/:room-s`) - Dành riêng cho Master A
* **Khung Soạn Thảo (Editor):** Textarea hỗ trợ thụt lề Tab, tự động lưu ngầm (Debounce 300ms) kèm theo `conn_id`.
* **Vùng Thả File (Dropzone):** Thả 1 file vào sẽ tự động chuyển Slot sang chế độ File.
* **Đèn Báo Trạng Thái Sợi Dây:**
  * `Đầu Phát: [Bạn đang giữ ●]`
  * `Đầu Nhận: [Chờ kết nối B cắm vào...]` $\rightarrow$ Chuyển sang xanh: `[Đã nối với B (1-to-1) ✓]` khi B bắt đầu kết nối.
* **Màn hình Khóa 409 (Lockout Screen):** Nếu người dùng mở tab thứ 2 của `*-s`, tab thứ 2 lập tức hiển thị màn hình cảnh báo khóa 🔒 và vô hiệu hóa ô soạn thảo.

### 4.2. Giao diện Đầu Nhận (`domain.com/:room-r`) - Dành cho Worker B (Nếu B mở Web)
* **Giao diện THUẦN CHỈ ĐỌC (Strictly Read-Only):**
  * **TUYỆT ĐỐI KHÔNG CÓ Ô SOẠN THẢO** $\rightarrow$ Không thể làm bẩn hay ghi đè lệnh của Master A!
  * Nếu phòng đang chứa Text: Hiển thị đoạn text với phông chữ monospace, tự động nhảy chữ khi A sửa bên đầu `-s`.
  * Nếu phòng đang chứa File: Hiển thị tên file, dung lượng và nút **[Tải Về]**.
* **Nút Bấm To Rõ:** `[📋 Copy Lệnh]` hoặc `[💾 Tải File Về Máy]`.
* **Màn hình Khóa 409:** Nếu mở double tab nhận, tab sau bị khóa ngay lập tức và không nhận dữ liệu.

---

## 5. QUY CHUẨN DÒNG LỆNH TỐI GIẢN CHO MÁY B (TERMINAL)

Nhờ mô hình khe cắm đơn nhất, Máy B **chỉ cần nhớ đúng 1 địa chỉ duy nhất: `domain.com/:room-r`**:

| Trường hợp | Lệnh ngắn nhất trên Linux | Lệnh ngắn nhất trên Windows PowerShell |
| :--- | :--- | :--- |
| **Kéo lệnh về chạy** | `curl -sL domain.com/1-r \| sh` | `irm domain.com/1-r \| iex` |
| **Tải file về máy** | `curl -sLJO domain.com/1-r` *(Tự lưu đúng tên file gốc)* | `iwr domain.com/1-r -OutFile setup.zip` |
| **Live Terminal Pipe** | `curl -sN domain.com/1-r \| bash` | *(Chạy script lắng nghe)* |

*Ghi chú quan trọng:* Khi phòng đang rỗng, Server tự động trả về comment `# WireDrop: (empty)\n` $\rightarrow$ Lệnh `irm ... | iex` của PowerShell **hoàn toàn không bị lỗi `EmptyStringNotAllowed`**.

---

## 6. CHIẾN LƯỢC TRUYỀN FILE: STORE-AND-FORWARD VS ZERO-STORAGE PIPE

Hệ thống xử lý linh hoạt 2 tình huống truyền file:

1. **Case 1: File Nhỏ & Vừa ($\le$ 150MB) – Chế độ Lưu Tạm (Store-and-Forward)**
   * A upload trước vào `1-s`. Server lưu tạm vào `/tmp` (hoặc RAM nếu $\le$ 15MB).
   * B vào `1-r` tải sau bất cứ lúc nào.
   * Tải xong 1 lần $\rightarrow$ Server tự động xóa file vĩnh viễn (Burn-after-download).
2. **Case 2: File Khổng Lồ (500MB đến 10GB+) – Chế độ Khớp Nối Trực Tiếp (Zero-Storage Pipe)**
   * B cắm ống đón trước: `curl -sLJO domain.com/1-r`.
   * A bắt đầu bơm file: kéo thả file trên web hoặc `curl -T big.iso domain.com/1-s`.
   * Server dùng `io.Pipe()` nối trực tiếp Socket mạng của A sang Socket mạng của B:
     * **RAM tiêu tốn:** Đúng **32KB** buffer mạng.
     * **Ổ cứng tiêu tốn:** **0 Byte** (không lưu đĩa).
     * Bơm xong là kết thúc, an toàn tuyệt đối cho server nhỏ.

---

## 7. ĐẶC TẢ API & ĐỊNH TUYẾN (GO 1.22+ SERVEMUX)

| Endpoint | Method | Vai trò | Mô tả hành vi |
| :--- | :---: | :---: | :--- |
| `GET /{$}` | `GET` | Mặc định | Chuyển hướng hoặc mở Web đầu phát của phòng mặc định `/1-s`. |
| `GET /{room}-s` | `GET` | Web Master A | Trả về Giao diện Web Soạn thảo (Sender UI). |
| `POST /{room}-s` | `POST` | Máy A (Web/CLI)| Bơm Text hoặc File vào khe cắm. Kiểm tra quyền sở hữu kết nối active. |
| `GET /{room}-r` | `GET` | Máy B (CLI/Web)| **Đầu Nhận Duy Nhất:** Tự động trả về Text thô hoặc Binary File. Kiểm tra quyền kết nối active. |
| `GET /api/status/{room}` | `GET` | Web A & B | Kênh SSE giữ nhịp kết nối vật lý và phát tín hiệu trạng thái sợi dây. Trả về `409` nếu đầu dây đã có người giữ. |
| `GET /health` | `GET` | Monitor | Trả về `OK 200`. |

---

## 8. BỘ LỌC PHÒNG VỆ & ỔN ĐỊNH VẬN HÀNH

1. **Chuẩn hóa CRLF:** Mọi dữ liệu text được làm sạch `\r\n` $\rightarrow$ `\n` bảo vệ bash Linux.
2. **Heartbeat Keep-Alive:** Gửi `: keep-alive\n` mỗi 15 giây giữ thông kết nối SSE và phát hiện socket đứt.
3. **Empty String Protection:** Trả về `# WireDrop: (empty)\n` khi phòng chưa có lệnh, chống sập PowerShell `iex`.
4. **Physical Connection Lockout:** Trả về `409 Conflict` kèm thông điệp rõ ràng khi có kết nối thứ hai cố gắng chen vào đầu dây đang bận.

---

## 9. CẤU TRÚC MÃ NGUỒN & KẾ HOẠCH CẬP NHẬT

* `room.go`: Xóa bỏ `SenderIP` và `ReceiverIP`. Bổ sung quản lý kết nối `SenderConn` và `ReceiverConn` (gắn chặt với `context.CancelFunc` và `conn_id`).
* `handler_pipe.go`: Cập nhật logic kiểm tra kết nối active cho `-s` và `-r`. Từ chối yêu cầu thứ 2 với mã `409`. Tự động nhả khóa khi socket đứt (`<-ctx.Done()`).
* `public/app.js`: Tự sinh `conn_id`, truyền tham số `?conn=...`, xử lý giao diện khóa 409 trực quan khi bị trùng kết nối.
* `public/index.html` & `public/style.css`: Thêm màn hình cảnh báo khóa 🔒 nổi bật khi bị 409 Line Busy.
