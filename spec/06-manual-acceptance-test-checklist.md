# MA TRẬN KIỂM THỬ THỰC CHIẾN 360° & BẢNG NGHIỆM THU TOÀN DIỆN
# (360° REAL-WORLD ACCEPTANCE TEST MATRIX & CHECKLIST)
# HỆ THỐNG: WireDrop v0.8.3

**Kim chỉ nam tối thượng:**  
> **"A giao dịch với B để ít quy trình nhất — Chân lý tối giản để vừa đạt."**

**Tôn chỉ nghiệm thu:** Không chỉ kiểm thử các kịch bản thuận lợi ("Happy Path"), tài liệu này được thiết kế theo **Ma Trận Thực Chiến 7 Trục (7-Axis Matrix)** bao phủ toàn bộ các góc khuất: thứ tự kết nối ngược, rào cản tường lửa máy ảo, chuỗi lệnh liên tiếp, ký tự xuống dòng Windows/Linux, file lớn vượt giới hạn Cloudflare, và phục hồi khi đứt cáp.

---

## 📋 MỤC LỤC 7 TRỤC THỰC CHIẾN

1. [Trục 1: Thứ Tự Kết Nối (Connection Timing & Sequence)](#-trục-1-thứ-tự-kết-nối-connection-timing--sequence)
2. [Trục 2: Chuỗi Lệnh Liên Hoàn & Đa Thể Loại (Sequential Multi-Turn Workflow)](#-trục-2-chuỗi-lệnh-liên-hoàn--đa-thể-loại-sequential-multi-turn-workflow)
3. [Trục 3: Rào Cản Mạng Thực Tế & Tường Lửa (Network Topology & Firewalls)](#-trục-3-rào-cản-mạng-thực-tế--tường-lửa-network-topology--firewalls)
4. [Trục 4: Dữ Liệu Khó & Ký Tự Đặc Biệt (Payload Edge Cases & Shell Escapes)](#-trục-4-dữ-liệu-khó--ký-tự-đặc-biệt-payload-edge-cases--shell-escapes)
5. [Trục 5: Kích Thước File Lớn & Vượt Giới Hạn Cloudflare >100MB (Stress Test)](#-trục-5-kích-thước-file-lớn--vượt-giới-hạn-cloudflare-100mb-stress-test)
6. [Trục 6: Sự Cố, Đứt Cáp & Tự Động Phục Hồi (Fault Injection & Resilience)](#-trục-6-sự-cố-đứt-cáp--tự-động-phục-hồi-fault-injection--resilience)
7. [Trục 7: Khóa Vật Lý 1-1 & Bảo Vệ Đường Dây (Physical Lock & Security)](#-trục-7-khóa-vật-lý-1-1--bảo-vệ-đường-dây-physical-lock--security)

---

## ⏱️ TRỤC 1: THỨ TỰ KẾT NỐI (CONNECTION TIMING & SEQUENCE)

> **Vấn đề:** Trong thực tế, không phải lúc nào bên phát cũng nạp dữ liệu trước. Người dùng máy ảo thường mở terminal hoặc web bên nhận ngồi chờ trước rồi mới qua máy thật soạn script.

### [ ] Case 1.1: TX Đến Trước Nạp Dữ Liệu $\rightarrow$ RX Đến Sau Lấy (Store & Forward)
- [ ] **Bước 1:** Tại máy thật (TX), mở `wiredrop 88` (hoặc Web TX `/88-s`), nạp một đoạn script.
- [ ] **Bước 2:** 5 phút sau, tại máy ảo (RX), mới mở terminal gõ: `wget -qO- <URL>/88-r | sh`.
- [ ] **Tiêu chí Đạt:**
  - RX lấy trọn vẹn dữ liệu ngay lập tức dù đến muộn.
  - Phía TX nhận tín hiệu Delivered ACK: `✓ ĐÃ ĐẾN MÁY ĐÍCH [RX CLI]`.

### [ ] Case 1.2: RX Đến Trước Cắm Dây Đợi $\rightarrow$ TX Đến Sau Nạp Hàng (Live Waiting)
- [ ] **Bước 1:** Mở Web RX `http://.../88-r` trên máy ảo trước khi TX mở. Trình duyệt hiển thị: `# Chờ dữ liệu từ TX...`.
- [ ] **Bước 2:** Tại máy thật, mở Web TX `http://.../88-s`, gõ nội dung văn bản.
- [ ] **Tiêu chí Đạt:**
  - Web RX tự động nhảy nội dung mới tức thì qua SSE mà **hoàn toàn không cần nhấn F5 reload trang**.
  - Đèn trạng thái `● TX • RX` trên cả 2 máy cùng sáng xanh.

### [ ] Case 1.3: Đường Ống Live Stream Từng Dòng (Live Pipe Streaming)
- [ ] **Bước 1:** Trên máy ảo Linux / Alpine, mở luồng thực thi trực tiếp ngồi đợi:
  - `wget -qO- http://.../88-r?live=1 | sh` (hoặc `wiredrop 88 -r | bash`).
- [ ] **Bước 2:** Trên máy thật, mở Terminal Pipe: `wiredrop 88`.
- [ ] **Bước 3:** Gõ dòng 1: `mkdir -p /tmp/wiredrop_test` $\rightarrow$ Nhấn Enter.
- [ ] **Bước 4:** Gõ dòng 2: `echo "SUCCESS" > /tmp/wiredrop_test/result.txt` $\rightarrow$ Nhấn Enter.
- [ ] **Tiêu chí Đạt:**
  - Máy ảo thực thi từng dòng lệnh ngay khi Enter (server flush từng dòng).
  - Kiểm tra trên máy ảo: file `/tmp/wiredrop_test/result.txt` đã được tạo thành công.
  - Phía TX in phản hồi Delivered ACK sau mỗi dòng.

---

## 🔄 TRỤC 2: CHUỖI LỆNH LIÊN HOÀN & ĐA THỂ LOẠI (SEQUENTIAL MULTI-TURN WORKFLOW)

> **Vấn đề:** Quá trình cài đặt máy ảo cần gửi liên tiếp 3-5 lệnh shell hoặc kết hợp vừa gửi script vừa gửi file cấu hình trong cùng 1 phòng.

### [ ] Case 2.1: Gửi Chuỗi 3 Lệnh Shell Liên Tiếp (Multi-Turn Scripting)
- [ ] **Bước 1:** TX gửi Lệnh 1 (`echo "TASK 1 DONE"`) $\rightarrow$ RX chạy và in kết quả.
- [ ] **Bước 2:** TX gửi tiếp Lệnh 2 (`echo "TASK 2 DONE"`) $\rightarrow$ RX chạy lệnh 2.
- [ ] **Bước 3:** TX gửi tiếp Lệnh 3 (`echo "TASK 3 DONE"`) $\rightarrow$ RX chạy lệnh 3.
- [ ] **Tiêu chí Đạt:**
  - RX không bị kẹt cache của Lệnh 1.
  - Phía TX: Hàng chip lịch sử hiển thị đủ 3 chip: `[#1: TASK 3...]` `[#2: TASK 2...]` `[#3: TASK 1...]`.
  - Bấm vào chip `#3`: Lệnh 1 được nạp lại vào ô nhập và gửi lại thành công.

### [ ] Case 2.2: Chuyển Đổi Thể Loại Hàng Hóa Linh Hoạt (Text $\rightarrow$ File $\rightarrow$ Text)
- [ ] **Bước 1:** TX nạp Text $\rightarrow$ RX nhận Text.
- [ ] **Bước 2:** TX chuyển tab sang **GỬI FILE**, kéo thả 1 file `setup.zip` $\rightarrow$ RX tự động chuyển sang giao diện Tải File `[TẢI FILE VỀ MÁY]`.
- [ ] **Bước 3:** RX tải xong file $\rightarrow$ TX chuyển lại tab **VĂN BẢN**, gõ lệnh giải nén `unzip setup.zip` $\rightarrow$ RX tự động trở lại khung xem Text.
- [ ] **Tiêu chí Đạt:** Hệ thống chuyển đổi mượt mà giữa các chế độ dữ liệu, không bị crash state.

---

## 🌐 TRỤC 3: RÀO CẢN MẠNG THỰC TẾ & TƯỜNG LỬA (NETWORK TOPOLOGY & FIREWALLS)

> **Vấn đề:** Máy ảo VMware không dùng được `localhost`. Khi gọi IP máy thật, Windows Firewall thường chặn cổng 8080. Cần kiểm chứng mọi con đường kết nối.

### [ ] Case 3.1: Mạng LAN Máy Thật ↔ Máy Ảo VMware (IP Nội Bộ & Firewall)
- [ ] **Bước 1:** Lấy IP mạng LAN của máy thật (ví dụ: `192.168.1.50` hoặc IP card VMware `192.168.137.1`).
- [ ] **Bước 2:** Trên máy ảo, thử kết nối: `wget -qO- http://192.168.x.x:8080/88-r`.
- [ ] **Xử lý nếu bị Timeout / Connection Refused (Do Windows Firewall):**
  - Mở PowerShell quyền Admin trên máy thật chạy:  
    `New-NetFirewallRule -DisplayName "WireDrop" -Direction Inbound -LocalPort 8080 -Protocol TCP -Action Allow`
- [ ] **Tiêu chí Đạt:** Máy ảo kết nối thẳng tới IP máy thật trơn tru qua mạng LAN.

### [ ] Case 3.2: Mạng Internet Vượt Tường Lửa (Cloudflare Quick Tunnel)
- [ ] **Bước 1:** Trên máy thật, chạy `wiredrop host`, chọn `[1] Quick Tunnel Trực Tiếp`.
- [ ] **Bước 2:** Lấy đường dẫn công khai `https://xxxx.trycloudflare.com`.
- [ ] **Bước 3:** Tắt mạng nội bộ trên máy ảo (hoặc dùng điện thoại bật 4G), truy cập URL Cloudflare.
- [ ] **Tiêu chí Đạt:** Vượt qua hoàn toàn NAT/Router gia đình, kết nối thành công 100% qua Internet.

### [ ] Case 3.3: Rút Gọn Cực Hạn Bằng Gateway s.wd.io.vn (12 Ký Tự)
- [ ] **Bước 1:** Bấm nút **`[⚡ RÚT GỌN]`** trên Web TX (hoặc chạy `wiredrop 88 -s`).
- [ ] **Bước 2:** Trên máy ảo VMware Alpine, gõ đúng **12 ký tự**:  
  `wget -qO- s.wd.io.vn/88-r | sh`
- [ ] **Tiêu chí Đạt:** Gateway chuyển tiếp chính xác về tunnel, lệnh thực thi ngay lập tức. Người gõ trên máy ảo chỉ tốn đúng 12 phím bấm.

### [ ] Case 3.4: Luồng Nghịch (Reverse Flow: Guest VM / Máy Khách TX → Host RX)
- [ ] **Bước 1:** Tại máy thật (Host), mở Terminal Receiver: `wiredrop 88 -r` (hoặc `wiredrop 88 -r -s` để có shortlink).
- [ ] **Bước 2:** Quan sát bảng **Cheatsheet Uplink** hiển thị trên terminal máy thật (đầy đủ các lệnh mẫu curl/wget/web đẩy file và stream log về Host).
- [ ] **Bước 3:** Tại máy ảo Guest (Alpine / Linux), chạy lệnh đẩy file hoặc stream log về Host:
  - Stream log: `echo "GUEST_BACKUP_COMPLETED" | curl -s -X POST --data-binary @- http://<LAN_IP>:8080/88-s` (hoặc shortlink `https://s.wd.io.vn/88-s`)
  - Hoặc đẩy file: `curl -s -F "file=@/var/log/messages" http://<LAN_IP>:8080/88-s`
  - Hoặc Alpine wget: `wget -qO- --post-file=/etc/issue http://<LAN_IP>:8080/88-s`
- [ ] **Bước 4 (Web RX):** Hoặc mở Web RX `http://localhost:8080/88-r`, bấm nút **`[⚡ COPY LINK MỜI GỬI (TX)]`** hoặc **`[⚡ RÚT GỌN]`**, dán sang máy khách để mở Web TX kéo thả file về Host.
- [ ] **Tiêu chí Đạt:**
  - Host RX nhận dữ liệu tức thì không trễ, không lỗi 409.
  - Phía Guest nhận phản hồi thành công `SENT` hoặc `HTTP 200`.

---

## 📝 TRỤC 4: DỮ LIỆU KHÓ & KÝ TỰ ĐẶC BIỆT (PAYLOAD EDGE CASES & SHELL ESCAPES)

> **Vấn đề:** Các lỗi kinh điển khi copy-paste giữa Windows và Linux: lỗi dấu xuống dòng `\r\n` phá hỏng bash, lỗi dấu nháy kép `"` làm vỡ cú pháp shell.

### [ ] Case 4.1: Thử Thách Ký Tự Xuống Dòng Windows CRLF (`\r\n` $\rightarrow$ `\n`)
- [ ] **Bước 1:** Trên Windows, dùng Notepad soạn một script 5 dòng:
  ```sh
  echo "Line 1"
  echo "Line 2"
  echo "Line 3"
  ```
  *(Notepad mặc định lưu dấu xuống dòng kiểu Windows `\r\n`).*
- [ ] **Bước 2:** Paste vào WireDrop TX.
- [ ] **Bước 3:** Trên máy ảo Linux/Alpine, chạy: `wget -qO- .../88-r | sh`.
- [ ] **Tiêu chí Đạt:**
  - Script chạy từ dòng 1 đến dòng 3 trơn tru.
  - **TUYỆT ĐỐI KHÔNG xuất hiện lỗi:** `sh: syntax error: unexpected "\r"` hoặc `command not found: $'\r'`.

### [ ] Case 4.2: Script Phức Tạp Chứa Ký Tự Thoát, Biến Môi Trường & Dấu Nháy
- [ ] **Bước 1:** Nạp đoạn script chứa đủ các ký tự nhạy cảm:
  ```sh
  VAR="WireDrop"
  echo "Test '$VAR' and \"$VAR\" with backslash \\ and pipe |"
  cat << 'EOF' > /tmp/app.env
  DATABASE_URL="postgres://user:p@ss$w0rd@localhost:5432/db?ssl=true"
  API_KEY='xyz$123"abc\'
  EOF
  ```
- [ ] **Bước 2:** Kéo về trên máy ảo và chạy `cat /tmp/app.env`.
- [ ] **Tiêu chí Đạt:** Nội dung file `.env` được tạo ra nguyên vẹn từng ký tự `$`, `"`, `'`, `\`, không bị bash nuốt biến trước khi ghi file.

### [ ] Case 4.3: Tên File & Nội Dung Tiếng Việt Có Dấu (RFC 5987 UTF-8)
- [ ] **Bước 1:** Đặt tên file: `Tài liệu_Báo cáo_2026 (Bản chuẩn) #1.pdf`.
- [ ] **Bước 2:** Nạp file lên TX.
- [ ] **Bước 3:** Trên máy nhận tải về bằng:
  - Linux: `curl -sLJO http://.../88-r`
  - Alpine: `wget http://.../88-r`
  - Trình duyệt Web: Bấm `[TẢI FILE VỀ MÁY]`
- [ ] **Tiêu chí Đạt:** File tải về giữ nguyên vẹn 100% tiếng Việt có dấu, không biến thành ký tự rác `???` hoặc `_`.

### [ ] Case 4.4: File Rỗng (0-Byte File) & Text Trống
- [ ] **Bước 1:** Nạp file có dung lượng đúng 0 byte.
- [ ] **Bước 2:** Nạp text rỗng.
- [ ] **Tiêu chí Đạt:** RX tải về an toàn, không bị crash server, không làm vỡ lệnh `irm | iex` trên PowerShell.

---

## 🚀 TRỤC 5: KÍCH THƯỚC FILE LỚN & VƯỢT GIỚI HẠN CLOUDFLARE >100MB (STRESS TEST)

> **Vấn đề:** Cloudflare Free Tunnel có giới hạn cứng **100MB cho mỗi HTTP Request**. WireDrop phải vượt qua giới hạn này mà tiêu tốn 0 byte đĩa cứng server.

### [ ] Case 5.1: File Nhỏ Dưới 16MB (Single Batch Fast Stream)
- [ ] Nạp file 5MB $\rightarrow$ Stream tức thì qua RAM, RX nhận về trọn vẹn trong vòng 1-2 giây.

### [ ] Case 5.2: Thử Thách Vượt Giới Hạn 100MB Cloudflare (Test Với File 150MB - 300MB)
- [ ] **Bước 1:** Tạo một file test 150MB trên máy thật:
  - PowerShell: `fsutil file createnew test150mb.dat 157286400`
- [ ] **Bước 2:** Khởi chạy FastHost Quick Tunnel (`*.trycloudflare.com`).
- [ ] **Bước 3:** Kéo file `test150mb.dat` vào Web TX.
- [ ] **Bước 4:** Trên máy nhận (qua 4G hoặc mạng ngoài), bấm tải file.
- [ ] **Tiêu chí Đạt:**
  - **KHÔNG BỊ LỖI:** `HTTP 413 Payload Too Large` của Cloudflare (nhờ cơ chế tự động chia mảnh 16MB và vi khung 256KB).
  - Lưới Batch Chips nhảy tiến độ liên tục từ mảnh 1 đến mảnh 10.
  - File tải về có mã băm SHA256 khớp 100% với file gốc (`Get-FileHash test150mb.dat`).

### [ ] Case 5.3: Giám Sát Bộ Nhớ RAM & Ổ Cứng Server Trong Khi Truyền File 500MB
- [ ] Nạp file 500MB qua WireDrop.
- [ ] Mở Task Manager / Resource Monitor quan sát tiến trình `wiredrop.exe`.
- [ ] **Tiêu chí Đạt:**
  - RAM chỉ dao động nhẹ trong khoảng 30MB - 60MB (nhờ `sync.Pool` tái sử dụng buffer).
  - Dung lượng ổ cứng server tiêu tốn **đúng 0 byte** (Zero-Disk Footprint).

---

## ⚡ TRỤC 6: SỰ CỐ, ĐỨT CÁP & TỰ ĐỘNG PHỤC HỒI (FAULT INJECTION & RESILIENCE)

> **Vấn đề:** Trong thế giới thực, mạng chập chờn, người dùng bấm nhầm nút hủy, hoặc tắt ngang cửa sổ khi đang truyền dở.

### [ ] Case 6.1: Hủy Truyền File Từ Phía Phát (TX Abort)
- [ ] Khi đang nạp file lớn đến mảnh 3/10, TX bấm nút đỏ **`[HỦY BỎ]`**.
- [ ] **Tiêu chí Đạt:**
  - TX dừng nạp ngay lập tức.
  - RX nhận sự kiện SSE `payload_cleared`, giao diện thông báo phiên truyền đã bị hủy.
  - Toàn bộ buffer RAM liên quan được thu hồi ngay lập tức, không rò rỉ bộ nhớ.

### [ ] Case 6.2: Máy Nhận Rút Cáp Giữa Chừng (Backpressure Safety Timeout)
- [ ] TX đang truyền file lớn, RX đang tải thì đột ngột tắt trình duyệt (hoặc ngắt máy ảo).
- [ ] **Tiêu chí Đạt:**
  - Bộ điều tiết áp lực ngược (Backpressure) phát hiện đầu đọc đã đóng kết nối.
  - Server kích hoạt timeout an toàn sau 30 giây để tự động hủy phiên treo, không giữ khóa vô tận.

### [ ] Case 6.3: Rớt Mạng Đột Ngột & Tự Động Kết Nối Lại (SSE Auto-Reconnect)
- [ ] Đang mở Web TX và RX, rút dây mạng / tắt Wi-Fi trong 5 giây rồi bật lại.
- [ ] **Tiêu chí Đạt:** SSE tự động thử kết nối lại ngầm trong nền, khi có mạng trở lại đèn `● TX • RX` tự sáng xanh mà không cần người dùng F5.

---

## 🔒 TRỤC 7: KHÓA VẬT LÝ 1-1 & BẢO VỆ ĐƯỜNG DÂY (PHYSICAL LOCK & SECURITY)

> **Vấn đề:** WireDrop là sợi dây 2 đầu thuần túy. Phải bảo đảm tính duy nhất tuyệt đối của mỗi đầu dây, không cho phép đầu dây thứ 3 can thiệp làm lẫn lộn dữ liệu.

### [ ] Case 7.1: Khóa Vật Lý Đầu Phát TX (`HTTP 409 Line Busy`)
- [ ] Tab 1 đang giữ quyền TX phòng `test`.
- [ ] Tab 2 mở cùng địa chỉ `http://.../test-s`.
- [ ] **Tiêu chí Đạt:**
  - Tab 2 lập tức bị chặn với Modal: `TX ĐANG BẬN (409)`.
  - Bấm nút `[THỬ KẾT NỐI LẠI]` trên Tab 2 $\rightarrow$ Tab 2 cướp quyền cắm dây, Tab 1 bị đá văng ra modal lockout.

### [ ] Case 7.2: Khóa Vật Lý Đầu Nhận RX (`HTTP 409`)
- [ ] Máy ảo 1 đang kết nối nhận dữ liệu phòng `test`.
- [ ] Máy ảo 2 cố tình kết nối cùng lúc vào đầu nhận.
- [ ] **Tiêu chí Đạt:** Máy ảo 2 nhận mã lỗi `HTTP 409 Line Busy`, bảo vệ tính toàn vẹn 1-to-1 của dữ liệu.

### [ ] Case 7.3: Cô Lập Tuyệt Đối Giữa Các Phòng Khác Nhau (Room Isolation)
- [ ] Phòng `room1` đang truyền file 200MB.
- [ ] Cùng lúc đó, phòng `room2` đang truyền text.
- [ ] **Tiêu chí Đạt:** Hai phòng hoạt động hoàn toàn song song, không ảnh hưởng tốc độ hay làm lẫn dữ liệu của nhau.

### [ ] Case 7.4: Hết Hạn Phòng Sau Thời Gian Chờ (TTL Inactivity Expiration)
- [ ] Mở 1 phòng nhưng không ai cắm dây hoặc không truyền dữ liệu sau 30 phút.
- [ ] **Tiêu chí Đạt:** Bộ dọn dẹp nền tự động giải phóng đối tượng phòng khỏi RAM, đảm bảo server có thể chạy liên tục nhiều tháng mà không đầy bộ nhớ.

---

## 🎯 BẢNG TỔNG KẾT NGHIỆM THU 7 TRỤC (ACCEPTANCE SIGN-OFF)

| Trục kiểm thử thực chiến | Số ca | Đạt (PASS) | Chưa đạt (FAIL) | Đánh giá thực tế |
| :--- | :---: | :---: | :---: | :--- |
| **Trục 1: Thứ Tự Kết Nối (RX trước vs TX trước)** | 3 | [ ] | [ ] | Hoạt động tốt cả Store & Forward lẫn Live Stream. |
| **Trục 2: Chuỗi Lệnh Liên Hoàn (Multi-Turn)** | 2 | [ ] | [ ] | Chuyển đổi Text $\leftrightarrow$ File mượt mà, lưu 3 chip. |
| **Trục 3: Rào Cản Mạng (VMware, Firewall, s.wd)** | 3 | [ ] | [ ] | Vượt NAT/Firewall, link 12 ký tự gõ cực nhanh. |
| **Trục 4: Dữ Liệu Khó (CRLF, Escapes, UTF-8)** | 4 | [ ] | [ ] | Không lỗi `\r`, bảo toàn nguyên vẹn dấu nháy. |
| **Trục 5: File Lớn Vượt Ngưỡng 100MB Cloudflare** | 3 | [ ] | [ ] | Vượt qua HTTP 413, RAM ổn định, 0 Byte Disk. |
| **Trục 6: Sự Cố, Đứt Cáp & Phục Hồi Lỗi** | 3 | [ ] | [ ] | Abort an toàn, Backpressure chống treo RAM. |
| **Trục 7: Khóa Vật Lý 1-1 & Bảo Vệ Đường Dây** | 4 | [ ] | [ ] | Khóa 409 chuẩn xác, cướp dây Reacquire minh bạch. |
| **TỔNG CỘNG** | **22** | **[ ]** | **[ ]** | **HỆ THỐNG ĐẠT CHUẨN XUẤT XƯỞNG TOÀN DIỆN.** |
