# ĐẶC TẢ KIẾN TRÚC: HỖ TRỢ LUỒNG NGHỊCH (GUEST TX → HOST RX) & SHORTLINK UPLINK CHO MÁY KHÁCH
# (REVERSE FLOW UPLINK & GUEST TX SHORTLINK)
# HỆ THỐNG: WireDrop v0.8.3

**Loại tài liệu:** `✅ APPROVED ARCHITECTURAL SPECIFICATION` (Đặc Tả Kiến Trúc Đã Được Phê Duyệt & Triển Khai)  
**Ngày khởi tạo:** 2026-11-19  
**Ngày phê duyệt:** 2026-11-19  
**Trạng thái:** 🚀 IMPLEMENTED (Đã Triển Khai Hoàn Tất & Kiểm Thử trong v0.8.3)  
**Tác giả:** Đội ngũ Kiến trúc WireDrop  
**Kim chỉ nam tối thượng:**  
> **"A giao dịch với B để ít quy trình nhất — Chân lý tối giản để vừa đạt."**

---

## MỤC LỤC
1. [Bối Cảnh & Vấn Đề Kiến Trúc: Điểm Mù Của Luồng Nghịch (Reverse Flow)](#1-bối-cảnh--vấn-đề-kiến-trúc-điểm-mù-của-luồng-nghịch-reverse-flow)
2. [Chi Tiết 3 Phân Hệ Nâng Cấp](#2-chi-tiết-3-phân-hệ-nâng-cấp)
   - [2.1. Phân hệ 1: Cheatsheet Uplink Trong Terminal Receiver (`wiredrop <room> -r [-s]`)](#21-phân-hệ-1-cheatsheet-uplink-trong-terminal-receiver-wiredrop-room--r--s)
   - [2.2. Phân hệ 2: Nút Sao Chép & Rút Gọn Link Mời Gửi TX Trên Web RX (`/room-r`)](#22-phân-hệ-2-nút-sao-chép--rút-gọn-link-mời-gửi-tx-trên-web-rx-room-r)
   - [2.3. Phân hệ 3: Đồng Bộ Hóa Cổng Rút Gọn Link Tuyển Chọn (`s.wd.io.vn`)](#23-phân-hệ-3-đồng-bộ-hóa-cổng-rút-gọn-link-tuyển-chọn-swdiovn)
3. [Giao Thức Dữ Liệu & Ma Trận Cú Pháp Máy Khách](#3-giao-thức-dữ-liệu--ma-trận-cú-pháp-máy-khách)
4. [Kế Hoạch Kiểm Thử & Nghiệm Thu](#4-kế-hoạch-kiểm-thử--nghiệm-thu)

---

## 1. BỐI CẢNH & VẤN ĐỀ KIẾN TRÚC: ĐIỂM MÙ CỦA LUỒNG NGHỊCH (REVERSE FLOW)

Trong mô hình truyền thống của WireDrop:
* **Luồng Thuận (Forward Flow - Host TX $\rightarrow$ Guest RX):**  
  Máy chủ Host mở đầu phát TX (`wiredrop <room>` hoặc `/room-s`), Terminal/Web hiển thị cheatsheet hướng dẫn máy khách (Guest VM/Phone) kéo lệnh về (`curl -sL host/:room-r | sh` hoặc `wget -qO- host/:room-r | sh`). Luồng này đã được tối ưu hóa toàn diện tại `v0.8.0` - `v0.8.2`.
* **Luồng Nghịch (Reverse Flow - Guest TX $\rightarrow$ Host RX):**  
  Người dùng trên máy Host chạy lệnh `wiredrop <room> -r` để **ngồi chờ đón dữ liệu** (như đón file backup, file log, kết quả build từ máy ảo VMware Alpine, máy chủ CI/CD, hoặc điện thoại). Trong kịch bản này:
  * Máy Host đóng vai trò là **Bên Nhận (RX)**.
  * Máy Khách / Máy Ảo đóng vai trò là **Bên Phát (TX)**.

### Điểm Mù Đã Phát Hiện:
1. **Thiếu Cheatsheet Uplink trên Terminal RX:**  
   Khi người dùng gõ `wiredrop 88 -r`, màn hình console chỉ in:
   ```
   • Kết nối tới: http://localhost:8080/88-r?live=1
   • Đang chờ dữ liệu từ Đầu Phát (TX)... (Nhấn Ctrl+C để thoát)
   ```
   Người dùng ngồi ở máy Host hoàn toàn không có sẵn câu lệnh mẫu để sao chép sang Máy Khách / Máy Ảo nhằm đẩy dữ liệu về!
2. **Cờ `-s` bị bỏ rơi trong Terminal RX:**  
   Bộ phân tích cú pháp `parseTerminalPipeArgs` có bóc tách cờ `-s`, nhưng hàm `runTerminalReceiver(serverURL, slug, isQuiet, useShortlink)` hoàn toàn không sử dụng tham số `useShortlink`, bỏ lỡ khả năng tạo link ngắn `https://s.wd.io.vn/88-s`.
3. **Web RX thiếu nút mời gửi (TX Invite):**  
   Khi người dùng mở giao diện Web RX (`/88-r`), thanh công cụ phía dưới chỉ có nút `[COPY DỮ LIỆU]`. Muốn mời đồng nghiệp hoặc thiết bị khác gửi dữ liệu vào phòng này, người dùng phải tự tay chỉnh sửa URL trên thanh địa chỉ thành `/88-s`.

---

## 2. CHI TIẾT 3 PHÂN HỆ NÂNG CẤP

### 2.1. Phân hệ 1: Cheatsheet Uplink Trong Terminal Receiver (`wiredrop <room> -r [-s]`)

* **Mục tiêu:** Khi gõ `wiredrop <room> -r` (hoặc có thêm cờ `-s`), màn hình console lập tức in ra **Bảng Cheatsheet Uplink** gồm các lệnh gửi mẫu cho mọi loại máy khách (Linux, Alpine Linux, Windows, Web, binary).
* **Cơ chế kỹ thuật:**
  1. Xác định địa chỉ phát `urlDirectTx` (`http://<LAN_IP>:<Port>/<slug>-s`).
  2. Nếu có cờ `-s` (`useShortlink = true`):
     - Gọi `registerShortlinkWithGateway("https://s.wd.io.vn", slug, serverURL)`.
     - Nhận về bộ đôi `(shortR, shortS)` và gán `displayTxWeb = shortS`.
  3. Nếu không có shortlink hoặc rút gọn thất bại: Fallback về `displayTxWeb = urlDirectTx`.
  4. Hiển thị khối Cheatsheet Uplink trực quan:
     ```
     ================================================================
       ⚡ WIREDROP PIPE: 88 [ĐẦU NHẬN RX ĐANG LẮNG NGHE]
     ================================================================
       Chạy 1 trong các lệnh sau tại Máy Khách / Máy Ảo (TX) để đẩy dữ liệu về:

       📱 Web / Điện thoại (TX):     https://s.wd.io.vn/88-s
       🐧 Linux/macOS curl đẩy file: curl -s -F "file=@<path>" https://s.wd.io.vn/88-s
       📟 Linux/macOS curl đẩy text: echo "log" | curl -s -X POST --data-binary @- https://s.wd.io.vn/88-s
       🏔️ Alpine Linux (wget file):  wget -qO- --post-file=<path> https://s.wd.io.vn/88-s

       (Nếu máy khách ĐÃ CÓ WireDrop binary: cat <file> | wiredrop 88)
     ----------------------------------------------------------------
       Đang chờ dữ liệu từ Đầu Phát (TX)... (Nhấn Ctrl+C để thoát)
     ================================================================
     ```
  5. **Bảo toàn chế độ Silent (`-q`):** Nếu stdout bị pipe sang chương trình khác (ví dụ `wiredrop 88 -r | bash`), cờ `isQuiet` được kích hoạt tự động, triệt tiêu toàn bộ Cheatsheet để không gây ô nhiễm dòng dữ liệu thô.

---

### 2.2. Phân hệ 2: Nút Sao Chép & Rút Gọn Link Mời Gửi TX Trên Web RX (`/room-r`)

* **Mục tiêu:** Cho phép người dùng đang ở giao diện Web RX lấy ngay đường link đầu phát TX chỉ với 1 cú nhấp chuột để gửi cho đối tác hoặc thiết bị khác.
* **Giao diện người dùng (`public/index.html` & `public/style.css`):**
  * Trong thanh cố định phía dưới (`.rx-fixed-bar` của `#receiverView`), bố trí nhóm nút điều khiển:
    * `[⚡ COPY LINK MỜI GỬI (TX)]` (`#btnCopyTxInvite`): Nút màu trắng tối giản, nhấp vào là sao chép URL đầu phát TX (`<origin>/<slug>-s` hoặc `s.wd.io.vn/<slug>-s`).
    * `[⚡ RÚT GỌN]` (`#btnShortenRx`): Nút tạo link ngắn tuyển chọn qua Gateway `s.wd.io.vn` ngay tại giao diện bên nhận.
* **Hành vi tương tác (`public/app.js`):**
  * Khi bấm `btnCopyTxInvite`:
    * Sao chép link TX vào Clipboard.
    * Kích hoạt toast: `[✓ ĐÃ SAO CHÉP LINK MỜI GỬI (TX)]`.
  * Khi bấm `btnShortenRx`:
    * Gửi request `POST /api/shorten` với `{ slug: slug }`.
    * Lưu `activeShortTxURL = data.short_s`.
    * Tự động sao chép `data.short_s` vào Clipboard và đổi nhãn nút thành `✓ ĐÃ RÚT GỌN`.

---

### 2.3. Phân hệ 3: Đồng Bộ Hóa Cổng Rút Gọn Link Tuyển Chọn (`s.wd.io.vn`)

* Endpoint `/api/shorten` trong `handler_pipe.go` vốn đã trả về cả hai trường:
  ```json
  {
    "status": "ok",
    "slug": "88",
    "short_r": "https://s.wd.io.vn/88-r",
    "short_s": "https://s.wd.io.vn/88-s",
    "fallback": false
  }
  ```
* Ở phiên bản `v0.8.3`, client Web và Terminal thống nhất sử dụng trường `short_s` cho mọi tình huống phục vụ Luồng Nghịch (Reverse Flow), đảm bảo tính nhất quán tuyệt đối giữa Web và CLI.

---

## 3. GIAO THỨC DỮ LIỆU & MA TRẬN CÚ PHÁP MÁY KHÁCH

Bảng tra cứu các phương thức đẩy dữ liệu từ Máy Khách (Guest TX) về Máy Chủ (Host RX) đã được hỗ trợ native bởi WireDrop:

| Hệ điều hành / Công cụ | Loại dữ liệu | Lệnh đẩy dữ liệu (Uplink Command) | Ghi chú tương thích |
| :--- | :--- | :--- | :--- |
| **Bất kỳ (Browser / Mobile)** | File hoặc Text | Mở link: `https://s.wd.io.vn/<room>-s` (hoặc `http://<IP>:<Port>/<room>-s`) | Kéo thả file hoặc gõ văn bản |
| **Linux / macOS (`curl`)** | File nhị phân | `curl -s -F "file=@<path>" <TX_URL>` | Đẩy file qua Multipart Form |
| **Linux / macOS (`curl`)** | Dòng text / Stream | `echo "data" \| curl -s -X POST --data-binary @- <TX_URL>` | Stream text hoặc log trực tiếp |
| **Alpine Linux (`wget`)** | File hoặc Text | `wget -qO- --post-file=<path> <TX_URL>` | Tương thích 100% Alpine / BusyBox |
| **Windows PowerShell** | File nhị phân | `irm -Uri <TX_URL> -Method Post -InFile <path>` | Zero-admin PowerShell native |
| **Đã cài WireDrop Binary** | File hoặc Pipe | `cat <path> \| wiredrop <room>` | Tự động nhận diện Pipe Stdin |

---

## 4. KẾ HOẠCH KIỂM THỬ & NGHIỆM THU

1. **Unit Test Backend (`sync_test.go`):**
   - Kiểm tra endpoint `/api/shorten` trả về đúng định dạng có cả `short_r` và `short_s`.
   - Kiểm tra việc nạp dữ liệu từ các phương thức đẩy file/text (`multipart/form-data`, `--data-binary`, `--post-file`).
2. **Terminal Handtest:**
   - Chạy `wiredrop 88 -r` $\rightarrow$ Xác nhận hiển thị đầy đủ Cheatsheet Uplink cho máy khách với IP LAN.
   - Chạy `wiredrop 88 -r -s` $\rightarrow$ Xác nhận đăng ký shortlink và hiển thị link `s.wd.io.vn/88-s`.
   - Chạy `wiredrop 88 -r -q` hoặc `wiredrop 88 -r | Out-Null` $\rightarrow$ Xác nhận không in Cheatsheet (Silent Mode).
3. **Web RX Handtest:**
   - Truy cập `http://localhost:8080/88-r`.
   - Kiểm tra sự xuất hiện của nút `[⚡ COPY LINK MỜI GỬI (TX)]` và `[⚡ RÚT GỌN]`.
   - Bấm nút $\rightarrow$ Xác nhận sao chép đúng link và hiển thị toast thông báo.
