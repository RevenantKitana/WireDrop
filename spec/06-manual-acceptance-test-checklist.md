# BẢNG CHECKLIST KIỂM THỬ THỰC TẾ & NGHIỆM THU TOÀN DIỆN (ACCEPTANCE TEST)
# HỆ THỐNG: WireDrop v0.8.2

**Mục đích:** Cung cấp quy trình kiểm thử thủ công từng bước (Handtest Checklist) bao phủ 100% tính năng và các kịch bản thực tế của WireDrop, bám sát nỗi đau nguyên bản: *"A giao dịch với B để ít quy trình nhất — Chân lý tối giản để vừa đạt."*

---

## 📋 MỤC LỤC CÁC PHÂN HỆ KIỂM THỬ

1. [Phân Hệ 1: Môi Trường Cục Bộ Web ↔ Web (Local Baseline)](#1-phân-hệ-1-môi-trường-cục-bộ-web--web-local-baseline)
2. [Phân Hệ 2: Terminal-Native Pipe Tương Tác & Lệnh Đường Ống (CLI ↔ CLI)](#2-phân-hệ-2-terminal-native-pipe-tương-tác--lệnh-đường-ống-cli--cli)
3. [Phân Hệ 3: Máy Khách Trắng Tinh / Máy Ảo Xa Lạ (VMware, Alpine & Headless OS)](#3-phân-hệ-3-máy-khách-trắng-tinh--máy-ảo-xa-lạ-vmware-alpine--headless-os)
4. [Phân Hệ 4: FastHost Tunnel & Cổng Rút Gọn s.wd.io.vn (Qua Mạng Internet)](#4-phân-hệ-4-fasthost-tunnel--cổng-rút-gọn-swdiovn-qua-mạng-internet)
5. [Phân Hệ 5: Trải Nghiệm Thiết Bị Di Động & Tải File Nền (Mobile UX)](#5-phân-hệ-5-trải-nghiệm-thiết-bị-di-động--tải-file-nền-mobile-ux)
6. [Phân Hệ 6: Các Ca Biên, Khóa Kết Nối & Phục Hồi Lỗi (Edge Cases & Resilience)](#6-phân-hệ-6-các-ca-biên-khóa-kết-nối--phục-hồi-lỗi-edge-cases--resilience)

---

## 1. PHÂN HỆ 1: MÔI TRƯỜNG CỤC BỘ WEB ↔ WEB (LOCAL BASELINE)

> **Mục tiêu:** Kiểm tra hoạt động của Sợi dây 2 đầu trên trình duyệt thông thường ở mạng nội bộ, bao gồm cơ chế Báo nhận (Delivered ACK) và Ngăn xếp 3 gói text gần nhất.

### ✅ Case 1.1: Truyền Văn Bản Thời Gian Thực (Live Text Streaming)
- [ ] **Bước 1:** Mở Terminal chạy server: `wiredrop server` (hoặc `wiredrop 88 -w`).
- [ ] **Bước 2:** Mở Tab 1 (Bên Phát): `http://localhost:8080/test-s`
- [ ] **Bước 3:** Mở Tab 2 (Bên Nhận): `http://localhost:8080/test-r`
- [ ] **Kiểm tra Đèn Cắm Dây:**
  - Tab 1 hiển thị thẻ trạng thái: `TX: /test-s` kèm biểu tượng `● TX • RX` (cả hai sáng xanh).
  - Toast thông báo `[RX ĐÃ KẾT NỐI]` xuất hiện trên Tab 1.
- [ ] **Kiểm tra Nhập liệu:**
  - Gõ văn bản trên khung textarea ở Tab 1.
  - Tab 2 tự động cập nhật nội dung tức thì qua SSE (không giật lag, không cần F5).
  - Nút `[SAO CHÉP]` ở Tab 2 hoạt động tốt, đổi trạng thái sang `✓ ĐÃ CHÉP`.

---

### ✅ Case 1.2: Phản Hồi Báo Nhận Tin TX (Delivered ACK) & Ngăn Xếp 3 Gói Text
- [ ] **Kiểm tra Báo Nhận Tin (Delivered ACK):**
  - Tại Tab 1 (TX), gõ một đoạn lệnh và quan sát huy hiệu trạng thái: `[ĐANG CHỜ RX...]`.
  - Tại Tab 2 (RX), bấm nút **`[SAO CHÉP]`**.
  - Quay lại Tab 1 (TX): Huy hiệu lập tức chuyển sang màu xanh lá:  
    `✓ ĐÃ ĐẾN MÁY ĐÍCH [RX Web (copy) lúc HH:mm:ss]`.
- [ ] **Kiểm tra Ngăn Xếp 3 Gói Text (Client History - 0 Byte Server):**
  - Tại Tab 1 (TX), gửi lần lượt 3 đoạn text khác nhau (ví dụ: `lệnh 1`, `lệnh 2`, `lệnh 3`).
  - Hàng chip bên dưới textarea xuất hiện 3 nút: `[#1: lệnh 3...]` `[#2: lệnh 2...]` `[#3: lệnh 1...]`.
  - Bấm vào chip `#3`: Textarea tự động khôi phục lại nội dung `lệnh 1` và tự lưu sang dây.
  - Kiểm tra Task Manager / Server: Không có database, không ghi ổ đĩa (lưu thuần `localStorage`).

---

### ✅ Case 1.3: Truyền File Lớn Đa Mảnh Zero-Disk (Turbo 256KB & 16MB Buffer)
- [ ] **Bước 1:** Tại Tab 1 (TX), chuyển sang chế độ **"GỬI FILE"**.
- [ ] **Bước 2:** Kéo thả một file lớn (> 20MB đến 100MB, ví dụ: file zip hoặc video) vào vùng dropzone.
- [ ] **Kiểm tra Dashboard bên phát (TX):**
  - Hiển thị thông tin: Tên file, dung lượng, tổng số mảnh (chunk).
  - Lưới hiển thị các ô mảnh (Batch Chips) hiển thị tiến độ nạp: `Chunk 0 ĐÃ NẠP • CHỜ RX KẾT NỐI`.
- [ ] **Kiểm tra Tải về bên nhận (RX):**
  - Tab 2 (RX) tự động phát hiện file và chuyển sang giao diện nhận file.
  - Bấm nút **`[TẢI VỀ NGAY]`**.
  - Thanh tiến độ tải và tốc độ truyền (MB/s) nhảy mượt mà.
  - File tải về mở lên bình thường, kích thước khớp 100% byte gốc.
- [ ] **Kiểm tra Bộ nhớ Server:**
  - Quan sát Task Manager: RAM tiến trình `wiredrop` chỉ dao động nhẹ (do dùng bộ đệm RAM xoay vòng 256KB / 16MB), ổ cứng server tiêu tốn **0 byte**.

---

## 2. PHÂN HỆ 2: TERMINAL-NATIVE PIPE TƯƠNG TÁC & LỆNH ĐƯỜNG ỐNG (CLI ↔ CLI)

> **Mục tiêu:** Nghiệm thu lệnh toàn cục Windows, trải nghiệm dòng lệnh mượt mà như `netcat` / `ssh pipe` và phản hồi nhận tin trong console.

### ✅ Case 2.1: Nhận Diện Lệnh Toàn Cục Trên Windows (`wiredrop install` & `install.bat`)
- [ ] **Bước 1:** Chạy tệp 1-click `install.bat` (hoặc mở CMD gõ: `.\wiredrop.exe install`).
- [ ] **Kiểm tra:**
  - In thông báo: `✓ ĐÃ CÀI ĐẶT WIREDROP TOÀN CỤC THÀNH CÔNG (v0.8.2)`.
  - File được copy vào `%LOCALAPPDATA%\Microsoft\WindowsApps\wiredrop.exe`.
- [ ] **Bước 2 (Kiểm tra thực tế):**
  - Mở một cửa sổ CMD hoặc PowerShell hoàn toàn mới ở bất kỳ thư mục nào (ví dụ: `C:\Users\...` hoặc ổ `D:\`).
  - Gõ trực tiếp: `wiredrop -h` $\rightarrow$ Nhận diện lệnh ngay lập tức mà không cần tiền tố `.\`.

---

### ✅ Case 2.2: Phiên REPL Tương Tác Bên Phát (`wiredrop <room>`) & Phản Hồi Nhận Tin
- [ ] **Bước 1:** Mở Terminal 1, gõ lệnh: `wiredrop 88`
- [ ] **Kiểm tra Banner & Cheat Sheet:**
  - In banner phiên bản: `WireDrop v0.8.2 - Terminal-Native Pipe & Live Relay`.
  - In Cheat Sheet các câu lệnh sẵn sàng copy cho máy nhận, bao gồm cả:
    - `🐧 Linux / macOS (curl): curl -sL <url> | sh`
    - `🏔️ Alpine Linux (wget): wget -qO- <url> | sh`
    - `🪟 Windows PowerShell: irm <url> | iex`
  - Xuất hiện prompt: `[88-s:TX]> `
- [ ] **Kiểm tra Lệnh Nội Bộ & Phản Hồi Nhận Tin Console:**
  - Gõ văn bản bất kỳ: `echo "Hello WireDrop"` $\rightarrow$ Dữ liệu được nạp vào dây.
  - Khi máy nhận (CLI hoặc Web) kéo dữ liệu về: Terminal TX tự động in thông báo:  
    `[✓] Máy đích (cli - read) đã nhận dữ liệu lúc HH:mm:ss!`.
  - Gõ `:status` $\rightarrow$ In thông báo trạng thái kết nối RX.
  - Gõ `:clear` $\rightarrow$ Xóa sạch dữ liệu phòng (`✓ Đã gửi lệnh xóa bộ đệm`).
  - Gõ `:exit` (hoặc nhấn `Ctrl+C`) $\rightarrow$ Thoát an toàn khỏi phiên làm việc.

---

### ✅ Case 2.3: Nạp File Nhị Phân Ngay Trong REPL (`:file <path>`)
- [ ] **Bước 1:** Tại Terminal 1 (`wiredrop 88`), gõ: `:file C:\path\to\sample.zip`
- [ ] **Kiểm tra:**
  - Terminal hiển thị: `[*] Bắt đầu nạp file: sample.zip (<size>)`
  - Thanh tiến độ tải hiển thị phần trăm nạp lên server.
  - Khi hoàn tất: `[OK] Đã nạp xong file! Đang giữ dây chờ RX tải về...`

---

### ✅ Case 2.4: Chế Độ Đường Ống Tự Động Đầu Vào (Zero-Flag Stdin Piping)
- [ ] **Bước 1:** Mở Terminal 2 (bên nhận): `wiredrop 88 -r`
- [ ] **Bước 2:** Mở Terminal 1 (bên phát), chạy lệnh pipe trực tiếp:
  - Trên Linux/macOS: `cat /etc/hosts | wiredrop 88`
  - Trên Windows PowerShell: `Get-Process | wiredrop 88`
- [ ] **Kiểm tra:**
  - Terminal 1 tự động tắt toàn bộ banner REPL, stream toàn bộ dữ liệu qua HTTP POST và tự thoát khi gặp EOF.
  - Terminal 2 hứng trọn vẹn dữ liệu và in ra màn hình ngay lập tức.

---

### ✅ Case 2.5: Chế Độ Đường Ống Tự Động Đầu Ra (Zero-Flag Stdout Piping)
- [ ] **Bước 1:** Bên phát đẩy 1 đoạn lệnh bash vào phòng 88.
- [ ] **Bước 2:** Bên nhận chạy pipe thẳng sang shell:
  - `wiredrop 88 -r | bash` (trên Linux)
- [ ] **Kiểm tra:**
  - Tiến trình tự động kích hoạt Quiet Mode (`-q`), không in banner ANSI.
  - Dữ liệu thô (raw stream) được đưa trực tiếp vào `bash` để thực thi chính xác.

---

## 3. PHÂN HỆ 3: MÁY KHÁCH TRẮNG TINH / MÁY ẢO XA LẠ (VMWARE, ALPINE & HEADLESS OS)

> **Mục tiêu:** Thẩm định bài toán cốt lõi: Máy nhận không cài WireDrop, không có trình duyệt, gõ tay ít ký tự nhất trên cả Debian/Ubuntu, Alpine Linux và Windows.

### ✅ Case 3.1: Máy Khách Linux / macOS (Chỉ dùng `curl` có sẵn)
- [ ] **Kịch bản Lấy Script & Thực Thi Luôn:**
  - Trên máy ảo Linux (Debian, Ubuntu, CentOS...), gõ: `curl -sL http://<IP>:8080/88-r | sh`
  - Kiểm tra: Kịch bản được kéo về và thực thi ngay lập tức.
  - Bên TX nhận ngay thông báo: `[✓] Máy đích (cli - read) đã nhận dữ liệu`.
- [ ] **Kịch bản Xem Nội Dung Trực Tiếp:**
  - Gõ: `curl -sL http://<IP>:8080/88-r`
  - Kiểm tra: In toàn bộ văn bản hoặc mã nguồn ra console.
- [ ] **Kịch bản Tải File Lớn Giữ Nguyên Tên Gốc:**
  - Gõ: `curl -sLJO http://<IP>:8080/88-r`
  - Kiểm tra: File được lưu đúng tên gốc (nhờ header `Content-Disposition`) và kích thước đầy đủ.

---

### ✅ Case 3.2: Máy Khách Tối Giản Alpine Linux / BusyBox (Chỉ dùng `wget` có sẵn)
- [ ] **Kịch bản Lấy Script & Thực Thi Trên Alpine (`alpine-virt`):**
  - Trên máy ảo Alpine Linux trắng tinh (không cài `curl`), gõ lệnh chuẩn 29 ký tự:  
    `wget -qO- http://<IP>:8080/88-r | sh`
  - Kiểm tra: Script chạy trơn tru, không báo lỗi `curl: not found`.
  - Bên TX nhảy ngay phản hồi: `✓ ĐÃ ĐẾN MÁY ĐÍCH [RX CLI (curl/wget)]`.
- [ ] **Kịch bản Tải File Nhị Phân Trên Alpine:**
  - Gõ: `wget -O app.bin http://<IP>:8080/88-r` (hoặc `wget http://<IP>:8080/88-r`).
  - Kiểm tra: File tải về nguyên vẹn byte gốc.

---

### ✅ Case 3.3: Máy Khách Windows (Chỉ dùng PowerShell có sẵn)
- [ ] **Kịch bản Thực Thi Lệnh Không Trình Duyệt:**
  - Mở PowerShell trên máy nhận, gõ: `irm http://<IP>:8080/88-r | iex`
  - Kiểm tra: Lệnh PowerShell thực thi trơn tru.
- [ ] **Kịch bản Tải File Nhị Phân:**
  - Gõ: `iwr http://<IP>:8080/88-r -OutFile app.exe`
  - Kiểm tra: Tải về file thực thi hoàn chỉnh.

---

## 4. PHÂN HỆ 4: FASTHOST TUNNEL & CỔNG RÚT GỌN s.wd.io.vn (QUA INTERNET)

> **Mục tiêu:** Nghiệm thu khả năng vượt tường lửa NAT/Router, tab ALPINE trên Web RX và tối ưu độ ngắn link nhập liệu.

### ✅ Case 4.1: Khởi Chạy Cloudflare Quick Tunnel Tự Động
- [ ] **Bước 1:** Chạy `wiredrop host` (hoặc mở `dist\wiredrop-portable.exe`).
- [ ] **Bước 2:** Menu xuất hiện, chọn `[1] Domain ngẫu nhiên Quick Tunnel (Khuyên dùng)`.
- [ ] **Kiểm tra:**
  - Tự động tải/kiểm tra binary `cloudflared`.
  - Sinh thành công địa chỉ công khai dạng `https://xxxx.trycloudflare.com`.
  - Tự động in bảng điều khiển Console với đầy đủ link cho máy ngoài.
- [ ] **Bước 3:** Tắt WiFi trên điện thoại, dùng 4G truy cập link `trycloudflare.com`:
  - Web UI tải nhanh chóng, SSE hoạt động ổn định.

---

### ✅ Case 4.2: Cơ Chế Rút Gọn Link Tuyển Chọn & Tab ALPINE Trên Web RX
- [ ] **Bước 1:** Mở trang TX trên Web qua link `trycloudflare.com`.
- [ ] **Kiểm tra Tooltip Gợi Ý:**
  - Tooltip nhấp nháy xuất hiện: `Link quá dài? Rút gọn ngay! 👇`.
  - Bên cạnh nút `COPY` xuất hiện nút **`[⚡ RÚT GỌN]`**.
- [ ] **Bước 2:** Bấm nút **`[⚡ RÚT GỌN]`**:
  - Gửi yêu cầu `POST /api/shorten`.
  - Nút chuyển trạng thái thành `✓ ĐÃ RÚT GỌN`.
  - Thanh lệnh RX tự động đổi link thành: `https://s.wd.io.vn/<room>-r`.
  - Tooltip tự động biến mất.
- [ ] **Bước 3:** Kiểm tra các Tab trên Thanh RX Bar:
  - Bấm tab **`LINUX`**: Hiển thị `curl -sL https://s.wd.io.vn/<room>-r | sh`.
  - Bấm tab **`ALPINE`**: Hiển thị `wget -qO- https://s.wd.io.vn/<room>-r | sh`.
  - Bấm tab **`WIN`**: Hiển thị `irm https://s.wd.io.vn/<room>-r | iex`.
  - Bấm tab **`URL`**: Hiển thị `https://s.wd.io.vn/<room>-r`.

---

### ✅ Case 4.3: Truy Cập Qua Gateway Siêu Ngắn `s.wd.io.vn`
- [ ] **Bước 1:** Trên máy ảo hoặc điện thoại, gõ đường dẫn cực ngắn: `https://s.wd.io.vn/<room>-r`
- [ ] **Kiểm tra:**
  - Gateway Cloudflare Worker tự động chuyển hướng (302) hoặc proxy về đúng tunnel Quick Tunnel đang chạy.
  - Người dùng ở máy ảo chỉ phải gõ đúng **12 ký tự** (`s.wd.io.vn/88-r`) thay vì 60 ký tự phức tạp của Cloudflare.

---

## 5. PHÂN HỆ 5: TRẢI NGHIỆM THIẾT BỊ DI ĐỘNG & TẢI FILE NỀN (MOBILE UX)

> **Mục tiêu:** Đảm bảo trải nghiệm trên smartphone hoàn hảo, không bị ngắt file giữa chừng.

### ✅ Case 5.1: Giao Diện Co Giãn Tương Thích (Responsive Mobile UI)
- [ ] **Bước 1:** Mở link phòng trên Chrome Android / Safari iOS.
- [ ] **Kiểm tra:**
  - Header, bảng mã, textarea và các nút bấm co giãn vừa vặn chiều ngang màn hình.
  - Không xuất hiện thanh cuộn ngang vỡ layout.
  - Nút bấm to bản, khoảng cách bấm chạm ngón tay thoải mái.

---

### ✅ Case 5.2: Tải File Lớn & Chống Tắt Màn Hình (Screen Wake Lock)
- [ ] **Bước 1:** Từ máy tính (TX), nạp file 100MB.
- [ ] **Bước 2:** Trên điện thoại (RX), bấm tải file.
- [ ] **Kiểm tra:**
  - API Screen Wake Lock tự động kích hoạt (màn hình giữ sáng trong lúc tải, không bị sleep làm rớt kết nối mạng).
  - Tải file hoàn tất 100%, file lưu vào thư mục Downloads của điện thoại mở ra nguyên vẹn.

---

## 6. PHÂN HỆ 6: CÁC CA BIÊN, KHÓA KẾT NỐI & PHỤC HỒI LỖI (EDGE CASES & RESILIENCE)

> **Mục tiêu:** Thử thách khả năng chịu lỗi và tính kỷ luật của hệ thống.

### ✅ Case 6.1: Khóa Kết Nối Vật Lý 1-1 (Physical Lock `HTTP 409`)
- [ ] **Bước 1:** Tab A đang mở `http://localhost:8080/room1-s`.
- [ ] **Bước 2:** Mở Tab B truy cập cùng địa chỉ `http://localhost:8080/room1-s`.
- [ ] **Kiểm tra:**
  - Tab B lập tức bị chặn lại với thông báo: `TX ĐANG BẬN (409) - Đầu dây này đã có 1 kết nối khác cắm vào.`
- [ ] **Bước 3:** Bấm nút **`[THỬ KẾT NỐI LẠI]`** trên Tab B:
  - Tab B cướp quyền cắm dây thành công và chuyển sang giao diện TX bình thường.
  - Tab A lập tức bị đẩy văng ra màn hình Lockout (không xảy ra xung đột dữ liệu).

---

### ✅ Case 6.2: Hủy Truyền File Giữa Chừng (Abort Upload Flow)
- [ ] **Bước 1:** Đang truyền file lớn nhiều mảnh (đến chunk thứ 3).
- [ ] **Bước 2:** Phía TX bấm nút đỏ **`[HỦY BỎ]`**.
- [ ] **Kiểm tra:**
  - Phía TX dừng gửi ngay lập tức.
  - Phía RX nhận sự kiện SSE thông báo phiên truyền đã bị hủy.
  - Bộ đệm RAM trên server lập tức được dọn dẹp, không rò rỉ bộ nhớ.

---

### ✅ Case 6.3: Phục Hồi Khi Máy Nhận Rút Dây Giữa Chừng (Backpressure Safety)
- [ ] **Bước 1:** TX bắt đầu truyền một file lớn.
- [ ] **Bước 2:** Đột ngột tắt trình duyệt RX (hoặc nhấn `Ctrl+C` trên máy nhận curl).
- [ ] **Kiểm tra:**
  - Cơ chế điều tiết áp lực ngược (Backpressure) phát hiện đầu đọc đã đóng kết nối.
  - TX nhận mã phản hồi phù hợp (`waiting_rx` hoặc tạm dừng), không bị crash server.
  - Khi RX mở lại, hệ thống có thể kết nối lại bình thường.

---

### ✅ Case 6.4: Truyền File Ký Tự Đặc Biệt & Tiếng Việt Có Dấu (RFC 5987 UTF-8)
- [ ] **Bước 1:** Nạp file có tên: `Tài liệu_Báo cáo_2026 (Bản chuẩn) #1.pdf`.
- [ ] **Bước 2:** Nhận file trên máy Linux bằng `curl -sLJO` hoặc trên trình duyệt.
- [ ] **Kiểm tra:** File tải về giữ nguyên vẹn 100% tiếng Việt có dấu, không bị lỗi font hoặc biến thành ký tự lạ `???`.

---

## 🎯 BẢNG TỔNG KẾT NGHIỆM THU (ACCEPTANCE SIGN-OFF)

| Phân hệ kiểm thử | Tổng số ca | Đạt (PASS) | Chưa đạt (FAIL) | Đánh giá chung |
| :--- | :---: | :---: | :---: | :--- |
| **1. Web ↔ Web Cục Bộ** | 3 | [ ] | [ ] | Text SSE, Delivered ACK xanh lá & 3-Text Stack. |
| **2. Terminal-Native Pipe** | 5 | [ ] | [ ] | `wiredrop install`, REPL ACK & Unix pipe. |
| **3. Máy Khách Xa Lạ (VM)** | 3 | [ ] | [ ] | Linux curl, Alpine `wget -qO-`, Windows irm. |
| **4. FastHost & s.wd.io.vn** | 3 | [ ] | [ ] | Quick Tunnel 4G, Tab ALPINE & shortlink 12 ký tự. |
| **5. Mobile UX & Tải Nền** | 2 | [ ] | [ ] | Responsive, Screen Wake Lock chống sleep. |
| **6. Ca Biên & Khóa 1-1** | 4 | [ ] | [ ] | Khóa 409 chuẩn xác, Abort an toàn, Backpressure. |
| **TỔNG CỘNG** | **20** | **[ ]** | **[ ]** | **Sẵn sàng nghiệm thu thực chiến toàn diện.** |
