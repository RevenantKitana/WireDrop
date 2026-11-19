# ĐẶC TẢ KIẾN TRÚC: TỐI ƯU HÓA THỰC CHIẾN MÁY ẢO, PHẢN HỒI NHẬN TIN TX & HỖ TRỢ ALPINE LINUX
# (REAL-WORLD VM HARDENING, DELIVERY ACK & ALPINE LINUX SUPPORT)
# HỆ THỐNG: WireDrop v0.8.2

**Loại tài liệu:** `✅ APPROVED ARCHITECTURAL SPECIFICATION` (Đặc Tả Kiến Trúc Đã Được Phê Duyệt & Đang Triển Khai)  
**Ngày khởi tạo:** 2026-11-19  
**Ngày phê duyệt:** 2026-11-19  
**Trạng thái:** 🚀 IMPLEMENTED (Đã Triển Khai Hoàn Tất & Kiểm Thử trong v0.8.2)  
**Tác giả:** Đội ngũ Kiến trúc WireDrop  
**Kim chỉ nam tối thượng:**  
> **"A giao dịch với B để ít quy trình nhất — Chân lý tối giản để vừa đạt."**

---

## MỤC LỤC
1. [Bối Cảnh & Động Lực: Kết Quả Từ Đợt Kiểm Thử Thực Tế (Handtest Insights)](#1-bối-cảnh--động-lực-kết-quả-từ-đợt-kiểm-thử-thực-tế-handtest-insights)
2. [Chi Tiết 3 Phân Hệ Nâng Cấp](#2-chi-tiết-3-phân-hệ-nâng-cấp)
   - [2.1. Phân hệ 1: Nhận diện Windows Command Toàn Cục (`wiredrop install`)](#21-phân-hệ-1-nhận-diện-windows-command-toàn-cục-wiredrop-install)
   - [2.2. Phân hệ 2: Cơ Chế Báo Nhận Tin (Delivered ACK) & Ngăn Xếp 3 Gói Text (Client-Side)](#22-phân-hệ-2-cơ-chế-báo-nhận-tin-delivered-ack--ngăn-xếp-3-gói-text-client-side)
   - [2.3. Phân hệ 3: Hỗ Trợ OS Tối Giản Alpine Linux / BusyBox (`wget -qO-`)](#23-phân-hệ-3-hỗ-trợ-os-tối-giản-alpine-linux--busybox-wget--qo-)
3. [Giao Thức Dữ Liệu & API Thay Đổi](#3-giao-thức-dữ-liệu--api-thay-đổi)
4. [Kế Hoạch Kiểm Thử & Nghiệm Thu](#4-kế-hoạch-kiểm-thử--nghiệm-thu)

---

## 1. BỐI CẢNH & ĐỘNG LỰC: KẾT QUẢ TỪ ĐỢT KIỂM THỬ THỰC TẾ (HANDTEST INSIGHTS)

Trong quá trình đưa WireDrop vào kiểm thử thực tế trên máy ảo trắng tinh (`alpine-virt-3.24.4-x86_64`) và môi trường Windows hàng ngày, 3 điểm cấn trải nghiệm thực chiến đã được phát hiện:

1. **Rào cản dòng lệnh Windows:** Người dùng gõ `wiredrop 88 -r` nhưng Windows báo không nhận diện được lệnh vì file thực thi chưa nằm trong `PATH`, hoặc do chính sách bảo mật của PowerShell yêu cầu tiền tố `.\`.
2. **Bên phát (TX) bị "mù thông tin":** Khi máy ảo gõ `curl` hoặc `wget` để kéo lệnh về thực thi, CLI lấy xong dữ liệu rồi ngắt kết nối luôn. Phía TX ngồi chờ mà không hề biết máy ảo đã nhận được hay chưa. Đồng thời, bên phát thường có nhu cầu bắn một chuỗi 2-3 lệnh liên tiếp và muốn xem lại/tái sử dụng lệnh vừa gửi mà không tốn tài nguyên server.
3. **OS tối giản không có `curl`:** Các hệ điều hành siêu nhẹ nguyên bản (Alpine Linux, OpenWrt, BusyBox) chỉ có sẵn **`wget`**, hoàn toàn không cài sẵn `curl`. Lệnh hướng dẫn mặc định `curl -sL` bị báo lỗi `curl: not found`.

Tài liệu này đặc tả phương án giải quyết dứt điểm 3 điểm cấn trên mà **vẫn giữ nguyên 100% Go Standard Library, 0 Dependency ngoài, và 0 Byte ghi đĩa server**.

---

## 2. CHI TIẾT 3 PHÂN HỆ NÂNG CẤP

### 2.1. Phân hệ 1: Nhận diện Windows Command Toàn Cục (`wiredrop install`)

* **Mục tiêu:** Cho phép người dùng Windows mở bất kỳ cửa sổ CMD hoặc PowerShell ở bất kỳ thư mục nào cũng gõ được trực tiếp: `wiredrop <room>` hoặc `wiredrop <room> -r`.
* **Cơ chế kỹ thuật Zero-Admin Elevation:**
  * Windows 10 & 11 có một thư mục User PATH mặc định:  
    `%LOCALAPPDATA%\Microsoft\WindowsApps` (thường là `C:\Users\<user>\AppData\Local\Microsoft\WindowsApps`).
  * Thư mục này **luôn có sẵn trong biến môi trường `PATH` của mọi người dùng** và **không yêu cầu quyền Run as Administrator** để ghi file vào.
* **Lệnh bổ sung:**
  * `wiredrop install`: Tự động sao chép chính file `wiredrop.exe` hiện tại vào `%LOCALAPPDATA%\Microsoft\WindowsApps\wiredrop.exe`.
  * `wiredrop uninstall`: Xóa file `wiredrop.exe` khỏi thư mục trên.
* **Tệp hỗ trợ 1-click:** Bổ sung file `install.bat` ở thư mục gốc dự án để người dùng chỉ cần nhấp đúp là hoàn tất cài đặt toàn hệ thống.

---

### 2.2. Phân hệ 2: Cơ Chế Báo Nhận Tin (Delivered ACK) & Ngăn Xếp 3 Gói Text (Client-Side)

#### A. Tín Hiệu Báo Nhận (Delivered ACK) Từ Máy Đích
* **Khi Máy Nhận (RX) là CLI (`curl` hoặc `wget`):**
  * Trong `handler_pipe.go`, hàm `handleReceiver()`: Ngay sau khi hoàn tất việc ghi dòng text hoặc stream file cho CLI, backend phát sự kiện SSE:
    ```
    event: payload_consumed
    data: {"by":"cli","time":"18:05:22"}
    ```
* **Khi Máy Nhận (RX) là Trình Duyệt Web:**
  * Bổ sung endpoint nhẹ: `POST /{room}/ack?action=copy` (khi bấm SAO CHÉP) hoặc `POST /{room}/ack?action=download` (khi bấm TẢI VỀ).
  * Server nhận request này và chuyển tiếp qua SSE tới bên phát:
    ```
    event: payload_consumed
    data: {"by":"web","action":"copy","time":"18:05:22"}
    ```
* **Biểu diễn trực quan bên Phát (TX):**
  * **Trên Web UI (TX):** Huy hiệu đồng bộ trạng thái:
    * Khi đang đợi: `[ĐANG CHỜ RX...]`
    * Khi nhận được SSE `payload_consumed`: Chuyển sang màu xanh lá rực rỡ kèm hiệu ứng nhấp nháy:  
      `✓ ĐÃ ĐẾN MÁY ĐÍCH (RX CLI lúc 18:05:22)` hoặc `✓ RX ĐÃ SAO CHÉP (Web lúc 18:05:22)`.
  * **Trên Terminal TX REPL:** Lắng nghe SSE và in thông báo tức thì ra console:  
    `[✓] Máy đích (CLI) đã nhận dữ liệu lúc 18:05:22!`

#### B. Ngăn Xếp 3 Gói Text Gần Nhất (Client-Side History)
* **Nguyên tắc bất biến:** **0 Byte Server Footprint**. Server hoàn toàn không lưu trữ lịch sử này.
* **Cơ chế:** Lưu trong `localStorage` của trình duyệt TX dưới key `wd_tx_history_<slug>` (danh sách tối đa 3 chuỗi gần nhất, cơ chế FIFO).
* **Giao diện Web:**
  * Bên dưới textarea, hiển thị hàng 3 nút chip retro:
    * `[1. "curl -sL..."]` `[2. "docker run..."]` `[3. "chmod +x..."]`
  * Bấm vào chip nào sẽ tự động nạp chuỗi đó vào textarea để sẵn sàng gửi lại ngay lập tức.

---

### 2.3. Phân hệ 3: Hỗ Trợ OS Tối Giản Alpine Linux / BusyBox (`wget -qO-`)

* **Phân tích:** Trên Alpine Linux và BusyBox, công cụ tải web mặc định duy nhất là `wget`.
* **Cú pháp vàng cho Alpine:**
  ```sh
  wget -qO- https://s.wd.io.vn/88-r | sh
  ```
  *(Chỉ dài 29 ký tự, hơn `curl` đúng 1 ký tự, hoạt động 100% out-of-the-box trên Alpine Linux trắng tinh).*
* **Cải tiến giao diện:**
  * Trong thanh mã lệnh RX Bar ở chân trang Web, tab `LINUX` được bổ sung nút toggle hoặc sub-tab:
    * `[CURL (Linux/macOS)]`: `curl -sL <url> | sh`
    * `[WGET (Alpine/BusyBox)]`: `wget -qO- <url> | sh`
  * Trang chủ và Cheat Sheet của Terminal Pipe được cập nhật bổ sung cú pháp này.

---

## 3. GIAO THỨC DỮ LIỆU & API THAY ĐỔI

1. **SSE Event Mới:**
   * Tên sự kiện: `payload_consumed`
   * Dữ liệu JSON: `{"by":"cli"|"web", "action":"read"|"copy"|"download", "time":"HH:mm:ss"}`
2. **Endpoint Mới:**
   * `POST /{room}/ack`:
     * Query param: `action=copy` hoặc `action=download`
     * Headers: Phải khớp mã phòng.
     * Response: `{"status":"ok"}`.
3. **Subcommand CLI Mới:**
   * `wiredrop install`
   * `wiredrop uninstall`

---

## 4. KẾ HOẠCH KIỂM THỬ & NGHIỆM THU

1. **Test Windows Command:** Chạy `wiredrop install`, mở cửa sổ CMD mới ở thư mục khác và gõ `wiredrop -h`.
2. **Test Delivered ACK:**
   * Mở Web TX, gõ text.
   * Chạy `curl .../88-r` từ terminal $\rightarrow$ Web TX phải nhảy thông báo `✓ ĐÃ ĐẾN MÁY ĐÍCH (RX CLI)`.
   * Mở Web RX, bấm SAO CHÉP $\rightarrow$ Web TX phải nhảy thông báo `✓ RX ĐÃ SAO CHÉP (Web)`.
3. **Test Ngăn xếp 3 Gói Text:** Gõ 3 đoạn text khác nhau, kiểm tra 3 chip hiển thị dưới textarea và nhấp vào để phục hồi.
4. **Test Alpine Linux:** Chạy `wget -qO- http://.../88-r | sh` trên máy ảo Alpine.
