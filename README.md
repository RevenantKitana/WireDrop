# ⚡ WireDrop

> **Ultra-lightweight, zero-dependency, pure unidirectional two-ended wire relay for scripts, commands, and files between machines (Web $\rightarrow$ Terminal / VM).**

[![Go Version](https://img.shields.io/badge/Go-1.22+-00ADD8?style=flat&logo=go)](https://golang.org)
[![Dependencies](https://img.shields.io/badge/Dependencies-Zero%20(Standard%20Library)-brightgreen)](https://golang.org/pkg/)
[![Version](https://img.shields.io/badge/Version-v0.6.2-blue)](changelog/README.md)
[![Architecture](https://img.shields.io/badge/Architecture-Two--Ended%20Wire%20(Physical%20Lock)-purple)](spec/README.md)
[![License](https://img.shields.io/badge/License-MIT-orange.svg)](LICENSE)

---

## 🎯 Vấn Đề Giải Quyết (The Problem)

Khi bạn quản trị máy ảo (VM), VPS, máy cứu hộ (Live ISO), hoặc thiết bị nhúng không có sẵn chia sẻ clipboard, việc copy một câu lệnh dài, script cài đặt, token bí mật hay file nhị phân từ máy chính (Host) sang máy đích (Guest/VM) thường rất phiền toái:
* Không thể copy-paste trực tiếp qua cửa sổ console máy ảo (VNC, NoVNC, Proxmox, VMware Console).
* Gõ tay từng ký tự rất dễ sai sót và mất thời gian.
* Dùng các dịch vụ pastebin công cộng vừa rườm rà, vừa lộ dữ liệu nhạy cảm, lại không hỗ trợ kéo thả file nhị phân hay streaming trực tiếp.
* Các công cụ chia sẻ 2 chiều (shared notepad) thường gây xung đột ghi đè dữ liệu giữa 2 bên.

**WireDrop** ra đời để giải quyết triệt để vấn đề này với mô hình: **"Một sợi dây vật lý 2 đầu thuần 1 chiều"**.

```
[ ĐẦU PHÁT: 1-s ] ══════════════ SỢI DÂY PHÒNG 1 ══════════════> [ ĐẦU NHẬN: 1-r ]
  (Máy A - Master)                                                 (Máy B - Worker)
  • Web Soạn thảo & Thả File                                       • PowerShell: irm .../1-r | iex
  • Toàn quyền chỉnh sửa                                           • Linux bash: curl .../1-r | sh
  • Khóa 1 kết nối phát                                            • Khóa 1 kết nối nhận (409 Busy)
```

---

## ✨ Tính Năng Nổi Bật (Key Features)

* 🔌 **Sợi dây 2 đầu thuần 1 chiều (Unidirectional Two-Ended Wire):** Phân tách tuyệt đối giữa Bên Phát (`*-s`) và Bên Nhận (`*-r`). Bên nhận chỉ thuần túy đọc/chạy lệnh, không có ô soạn thảo để làm bẩn hay ghi đè kịch bản gốc.
* 🔒 **Khóa kết nối vật lý 1-1 (Pure Physical Connection Lock):** Không cần tài khoản, mật khẩu hay token thủ công. Mỗi đầu dây chỉ chấp nhận **DUY NHẤT 1 kết nối mạng active**. Nếu mở double tab trên cùng máy hay cùng mạng, tab thứ hai lập tức bị chặn với mã `HTTP 409 Line Busy`.
* 🛡️ **Loại trừ tuyệt đối giữa Web và CLI (Mutual Exclusion):** Nếu đang mở cửa sổ Web nhận, Terminal CLI sẽ bị chặn `409` để chống xung đột; khi đóng cửa sổ Web, khóa tự giải phóng tức thì trong vài mili-giây để CLI cắm vào tiêu thụ.
* 📦 **Đơn vị dữ liệu nguyên tử (Single Atomic Slot):** Một phòng tại một thời điểm chỉ chứa đúng **1 Script/Text HOẶC 1 File duy nhất**. Gửi cái mới sẽ tự động thay thế cái cũ.
* 💥 **Tự hủy sau khi nhận (Burn-After-Read):** Dữ liệu tự động biến mất khỏi bộ nhớ RAM của server ngay sau khi máy nhận tải về 1 lần, bảo mật tối đa.
* 🛡️ **Chống sập PowerShell (Empty Slot Protection):** Khi phòng đang trống, server tự trả về comment `# WireDrop: (empty)\n` $\rightarrow$ Lệnh `irm ... | iex` trên Windows thoát êm xuôi mã 0, không bao giờ bị lỗi `EmptyStringNotAllowed`.
* 🌊 **Live Terminal Pipe:** Hỗ trợ cắm thẳng luồng lệnh trực tiếp từ Web sang bash qua lệnh `curl -sN .../1-r | bash`.
* 🪶 **100% Go Standard Library (Zero-Dependency):** Biên dịch thành 1 file nhị phân duy nhất (~6.8 MB), nhúng sẵn toàn bộ Web SPA Dark Mode (`//go:embed`), tiêu tốn chỉ ~10MB RAM.

---

## 🚀 Khởi Động Nhanh (Quick Start)

### 1. Tải về hoặc Biên dịch từ mã nguồn

Yêu cầu: Go 1.22 trở lên.

```bash
# Clone repository
git clone https://github.com/RevenantKitana/WireDrop.git
cd WireDrop

# Biên dịch binary siêu nhẹ (Stripped symbols)
go build -ldflags="-s -w" -o wiredrop .
```

### 2. Chạy Server

```bash
# Chạy với cổng mặc định 8080:
./wiredrop

# Hoặc chỉ định cổng tùy ý:
PORT=18080 ./wiredrop
```

---

## 📖 Hướng Dẫn Sử Dụng (Usage)

### Chiều truyền: Máy A (Phát) $\rightarrow$ Máy B (Nhận)

#### Bước 1: Máy A (Bên Phát / Master)
Mở trình duyệt: `http://<IP_Server>:8080/1-s`  
*(Giao diện Dark Mode có ô soạn thảo auto-save và vùng kéo thả file).*

#### Bước 2: Máy B (Bên Nhận / Worker)

Chỉ cần nhớ duy nhất 1 địa chỉ: `domain.com/:room-r`

| Nhu cầu của Máy B | Lệnh trên Linux / macOS | Lệnh trên Windows PowerShell |
| :--- | :--- | :--- |
| **Kéo script về chạy ngay** | `curl -sL http://<IP>:8080/1-r \| sh` | `irm http://<IP>:8080/1-r \| iex` |
| **Tải file nhị phân về** | `curl -sLJO http://<IP>:8080/1-r` | `iwr http://<IP>:8080/1-r -OutFile file.zip` |
| **Live Stream Pipe** | `curl -sN http://<IP>:8080/1-r \| bash` | *(Lắng nghe thời gian thực)* |
| **Xem qua Trình duyệt** | Mở `http://<IP>:8080/1-r` *(Giao diện Read-Only, chỉ xem & copy)* | |

---

### Khi Máy B muốn gửi kết quả ngược lại cho Máy A?
Áp dụng nguyên lý đối xứng: **Máy B mở một phòng mới (ví dụ phòng `2`)**:
* Máy B là bên phát: Mở `http://<IP>:8080/2-s`
* Máy A là bên nhận: Cắm vào `http://<IP>:8080/2-r`
* *Hai sợi dây hoàn toàn độc lập, không làm bẩn ô lệnh gốc tại phòng 1.*

---

### ⚡ Phím Tắt Tiện Ích (Quick Aliases)
WireDrop tích hợp sẵn script tạo phím tắt `r [room_id]` cực ngắn trên terminal máy khách:
* **Linux:** `curl -sL http://<IP>:8080/i | sh` $\rightarrow$ Từ sau chỉ cần gõ `r` hoặc `r 2`.
* **Windows PowerShell:** `irm http://<IP>:8080/i.ps1 | iex` $\rightarrow$ Từ sau chỉ cần gõ `r` hoặc `r 2`.

---

## 🚢 Hướng Dẫn Deploy (Deployment)

### 1. Deploy bằng Systemd trên Linux VPS

Biên dịch cho Linux:
```powershell
$env:GOOS="linux"; $env:GOARCH="amd64"; go build -ldflags="-s -w" -o wiredrop .
```

Tạo file dịch vụ `/etc/systemd/system/wiredrop.service`:
```ini
[Unit]
Description=WireDrop Service
After=network.target

[Service]
Type=simple
User=root
Environment="PORT=8080"
ExecStart=/usr/local/bin/wiredrop
Restart=always

[Install]
WantedBy=multi-user.target
```

Khởi chạy:
```bash
sudo systemctl enable --now wiredrop
```

> **Lưu ý cấu hình Nginx Reverse Proxy (Quan trọng cho SSE & Streaming):**  
> Phải thêm `proxy_buffering off;` và `proxy_read_timeout 86400s;` để không bị ngắt luồng giữ kết nối vật lý.

### 2. Deploy bằng Docker (Image ~7MB `FROM scratch`)

```bash
docker build -t wiredrop .
docker run -d --name wiredrop -p 8080:8080 --restart unless-stopped wiredrop
```

### 3. Public nhanh từ máy cá nhân qua Cloudflare Tunnel (Không cần VPS)

```bash
cloudflared tunnel --url http://localhost:8080
```

---

## 📂 Cấu Trúc Dự Án (Project Structure)

```text
WireDrop/
├── main.go            # Entry point, routing Go 1.22+ ServeMux, embedded assets
├── room.go            # Quản lý PipeSlot, ChunkSession, khóa ActiveConn, Mutex RAM, TTL cleaner
├── handler_pipe.go    # Điều phối đầu *-s / *-r, 16MB chunked pipe, streaming flusher, SSE events
├── host.go            # Công cụ FastHost tích hợp tự động Cloudflare Tunnel cho bản Portable
├── go.mod             # Module definition (Zero third-party dependencies)
├── Dockerfile         # Multi-stage build (Alpine -> Scratch ~7MB)
├── public/            # Giao diện Web SPA Retro Monochrome Minimalist được nhúng tĩnh
│   ├── index.html     # HTML thống nhất Home / Sender / Receiver View & Multi-Batch Dashboard
│   ├── style.css      # Giao diện đơn sắc tối giản, lưới mảnh .batch-grid
│   └── app.js         # Chunked Ping-Pong Upload, Direct-to-Disk RX Stream, SSE & Hybrid Sync
├── spec/              # Kho tài liệu Đặc tả Kiến trúc (APPROVED SPEC) & Chuẩn bị Đặc tả (SPEC-PREPARE)
└── changelog/         # Lịch sử phát triển chi tiết từng mốc phiên bản (Sort-Safe Naming)
```

---

## 📄 Bản Quyền (License)

Dự án được phân phối dưới giấy phép **MIT License**. Mọi đóng góp (Pull Request, Issue) đều được chào đón nồng nhiệt!
