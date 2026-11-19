# ⚡ WireDrop

> **Ultra-lightweight, zero-dependency, pure unidirectional two-ended wire relay for scripts, commands, and files between machines (Web $\rightarrow$ Terminal / VM).**

[![Go Version](https://img.shields.io/badge/Go-1.22+-00ADD8?style=flat&logo=go)](https://golang.org)
[![Dependencies](https://img.shields.io/badge/Dependencies-Zero%20(Standard%20Library)-brightgreen)](https://golang.org/pkg/)
[![Version](https://img.shields.io/badge/Version-v0.8.3-blue)](changelog/README.md)
[![Architecture](https://img.shields.io/badge/Architecture-Two--Ended%20Wire%20(Physical%20Lock)-purple)](spec/README.md)
[![License](https://img.shields.io/badge/License-MIT-orange.svg)](LICENSE)

---

> **Kim chỉ nam tối thượng:**  
> *"A giao dịch với B để ít quy trình nhất — Chân lý tối giản để vừa đạt."*

---

## 🎯 Vấn Đề Giải Quyết (The Problem)

Khi bạn quản trị máy ảo (VMware, Proxmox, VirtualBox, Alpine Linux), máy chủ từ xa (VPS), container Docker hoặc thiết bị di động không có chia sẻ clipboard:
* Không thể copy-paste trực tiếp qua cửa sổ console máy ảo (VNC, NoVNC, Web Console).
* Gõ tay từng lệnh dài, token bí mật hay URL rất dễ sai sót và ức chế.
* Dùng pastebin công cộng vừa rườm rà, vừa lộ bí mật, vừa không hỗ trợ truyền file hay streaming trực tiếp.
* Các công cụ 2 chiều (shared notepad) thường gây xung đột ghi đè dữ liệu.

**WireDrop** giải quyết triệt để vấn đề này với mô hình: **"Sợi dây vật lý 2 đầu thuần 1 chiều"** (`TX` $\rightarrow$ `RX`).

```text
[ ĐẦU PHÁT: TX (*-s) ] ══════════════ SỢI DÂY (SLOT 1-1) ══════════════> [ ĐẦU NHẬN: RX (*-r) ]
  • Web Soạn thảo / Thả file                                               • Alpine: wget -qO- <URL> | sh
  • Terminal Pipe: wiredrop 88                                             • Linux:  curl -sL <URL> | sh
  • Khóa 1 kết nối phát                                                    • Windows: irm <URL> | iex
  • Huy hiệu Delivered ACK khi máy đích nhận                               • Khóa 1 kết nối nhận (409 Busy)
```

---

## ✨ Tính Năng Nổi Bật (Key Features)

* 🔌 **Sợi dây 2 đầu thuần 1 chiều (Unidirectional Two-Ended Wire):** Phân tách tuyệt đối giữa Bên Phát (`*-s`) và Bên Nhận (`*-r`). Bên nhận chỉ thuần túy đọc/chạy lệnh, không có ô soạn thảo để làm bẩn kịch bản gốc.
* 🔒 **Khóa kết nối vật lý 1-1 (Pure Physical Connection Lock):** Không tài khoản, không token. Mỗi đầu dây chỉ chấp nhận **DUY NHẤT 1 kết nối active**. Cửa sổ thứ hai cố cắm vào bị chặn ngay lập tức với mã `HTTP 409 Line Busy`.
* 🛡️ **Loại trừ tuyệt đối Web vs CLI (Mutual Exclusion):** Mở Web nhận thì chặn CLI; đóng Web nhận thì khóa tự giải phóng tức thì cho CLI cắm vào.
* 📦 **Đơn vị dữ liệu nguyên tử (Single Atomic Slot):** Mỗi phòng chỉ chứa đúng **1 Script/Text HOẶC 1 File**. Nạp cái mới tự động thay thế cái cũ.
* 🚀 **Terminal-Native Pipe Siêu Tốc:** Điều khiển trực tiếp từ dòng lệnh như `netcat`/`ssh` pipe mà không cần mở trình duyệt (`cat file | wiredrop 88`).
* ⚡ **Turbo Transfer Engine (Zero-Disk):** Vi khung Sub-Chunk 256KB truyền file lớn vượt giới hạn 100MB Cloudflare Edge qua bộ đệm kép Ping-Pong, 0 byte ghi đĩa server.
* 🏔️ **Hỗ trợ toàn diện Alpine Linux / BusyBox:** Cung cấp sẵn cú pháp `wget -qO-` hoạt động 100% out-of-the-box trên các OS tối giản không có `curl`.
* 📡 **Huy hiệu Delivered ACK:** Bên phát nhận được thông báo thời gian thực khi máy nhận đã kéo lệnh hoặc tải file thành công (`✓ ĐÃ ĐẾN MÁY ĐÍCH`).
* 🔄 **Hỗ trợ Luồng Nghịch (Reverse Flow):** Host RX (`wiredrop 88 -r`) tự động in Cheatsheet Uplink cho Guest VM/Phone đẩy log hoặc file ngược về Host.
* 🪶 **100% Go Standard Library (Zero-Dependency):** 0 thư viện ngoài, đóng gói 1 file binary duy nhất (~8MB), nhúng sẵn Web Retro Monochrome Minimalist (`//go:embed`), RAM tiêu thụ chỉ ~10MB.

---

## 🚀 Cài Đặt Nhanh (Installation)

### Cách 1: Cài đặt 1-Click vào Windows User PATH (Zero-Admin)
Nhấp đúp file **`install.bat`** ở thư mục gốc (hoặc chạy trong PowerShell):
```powershell
.\install.bat
```
Lệnh sẽ tự động biên dịch và sao chép vào `%LOCALAPPDATA%\Microsoft\WindowsApps\wiredrop.exe`.  
Kể từ đó, bạn có thể mở bất kỳ cửa sổ CMD/PowerShell nào và gõ trực tiếp `wiredrop`!

### Cách 2: Tự biên dịch từ mã nguồn (Linux / macOS / Windows)
Yêu cầu: Go 1.22 trở lên.
```bash
git clone https://github.com/RevenantKitana/WireDrop.git
cd WireDrop

# Biên dịch ra thư mục bin/
go build -ldflags="-s -w" -o bin/wiredrop .
```

---

## 📖 Hướng Dẫn Sử Dụng (Usage)

### 1. Sử Dụng Qua Dòng Lệnh (Terminal-Native Pipe)

| Lệnh | Ý nghĩa |
| :--- | :--- |
| `wiredrop 88` | Mở console tương tác phát văn bản / nạp file vào phòng 88 |
| `wiredrop 88 -s` | Mở console phát kèm rút gọn link qua `s.wd.io.vn` (12 ký tự) |
| `wiredrop 88 -r` | Lắng nghe dữ liệu phòng 88 trên terminal (kèm Cheatsheet cho máy khách) |
| `wiredrop 88 -r -s` | Lắng nghe với Cheatsheet Uplink rút gọn `s.wd.io.vn` |
| `cat script.sh \| wiredrop 88` | Bơm trực tiếp file/script từ terminal vào phòng 88 |
| `wiredrop 88 -r \| bash` | Hứng dữ liệu từ phòng 88 và thực thi thẳng qua bash (Silent Mode) |
| `wiredrop host` | Tự động tạo Cloudflare Quick Tunnel public ra Internet |
| `wiredrop install` | Cài đặt `wiredrop` vào Windows PATH toàn cục |

### 2. Sử Dụng Qua Giao Diện Web (Retro Monochrome UI)

* **Trang chủ điều phối:** Mở `http://localhost:8080/#` để nhập mã phòng nhanh.
* **Bên Phát (TX):** Mở `http://localhost:8080/88-s` để dán văn bản, kéo thả file, xem lịch sử 3 gói text gần nhất và nhận huy hiệu Delivered ACK.
* **Bên Nhận (RX):** Mở `http://localhost:8080/88-r` để xem dữ liệu, copy, tải file (Native OS tải ngầm), hoặc bấm nút `[⚡ COPY LINK MỜI GỬI (TX)]` để gửi cho máy khách.

### 3. Câu Lệnh Kéo Về Máy Khách (0 Cần Cài Đặt)

| Hệ điều hành máy khách | Kéo script chạy ngay | Tải file nhị phân về đĩa |
| :--- | :--- | :--- |
| **Linux / macOS (`curl`)** | `curl -sL <URL>/88-r \| sh` | `curl -sLJO <URL>/88-r` |
| **Alpine Linux / BusyBox (`wget`)** | `wget -qO- <URL>/88-r \| sh` | `wget -O <file> <URL>/88-r` |
| **Windows PowerShell** | `irm <URL>/88-r \| iex` | `iwr <URL>/88-r -OutFile <file>` |
| **Trình duyệt / Mobile** | Mở `<URL>/88-r` | Bấm `[TẢI FILE VỀ MÁY]` |

---

## 📂 Cấu Trúc Thư Mục Ngăn Nắp (Clean Project Layout)

```text
WireDrop/
├── changelog/          # Lịch sử 31 phiên bản kỹ thuật súc tích + README.md
├── gateway/            # Cloudflare Worker Gateway cho cổng shortlink s.wd.io.vn
├── public/             # Web SPA Retro Monochrome nhúng tĩnh (HTML, CSS, JS)
├── scripts/            # Script phụ trợ tiện ích (build.bat, install.bat)
├── spec/               # Tài liệu Đặc tả Kiến trúc chính thức + Checklist nghiệm thu 360°
│
├── .dockerignore       # Loại trừ file rác khi build Docker
├── .env.example        # Cấu hình biến môi trường mẫu (PORT, TUNNEL_TOKEN...)
├── .gitignore          # Chặn sạch *.exe, bin/, dist/ không lọt vào git
├── CLAUDE.md           # Bộ quy tắc quản lý dự án & chuẩn Keep a Changelog tinh khiết
├── Dockerfile          # Multi-stage Docker build (~7MB image)
├── LICENSE             # Giấy phép MIT
├── README.md           # Tài liệu hướng dẫn chính thức (v0.8.3)
├── go.mod              # Module definition (100% Go Standard Library)
├── install.bat         # Trình cài đặt 1-click vào Windows PATH
│
├── handler_pipe.go     # Xử lý luồng HTTP Sợi dây 2 đầu (*-s vs *-r, Turbo sub-chunk)
├── host.go             # FastHost native (tự động hóa Cloudflare Tunnel)
├── main.go             # Khởi động server, ServeMux routing, version single source of truth
├── room.go             # Quản lý RAM, mutex, khóa vật lý 1-1, atomic slot
├── sync_test.go        # 22 bài test tự động bao phủ toàn diện
└── term_pipe.go        # Terminal-Native Pipe, REPL console & Live Stream
```

---

## 📄 Bản Quyền (License)

Dự án được phân phối dưới giấy phép **[MIT License](LICENSE)**.
Mọi đóng góp, phản hồi và ý kiến cải tiến luôn được hoan nghênh!
