# LỊCH SỬ THAY ĐỔI DỰ ÁN (PROJECT CHANGELOGS)

Thư mục này lưu trữ toàn bộ các file changelog kỹ thuật của dự án **WireDrop**, tuân thủ nghiêm ngặt chuẩn **Keep a Changelog Tinh Khiết** (Pure Technical Changelog), phân tách theo từng mốc phát triển `v0.X.Y`.

Mọi file được đặt tên theo quy chuẩn sắp xếp thời gian tự nhiên: `NN-v0.X.Y-<slug>.md`.

---

## Danh Mục Changelog Theo Phiên Bản:

### 🚀 Giai Đoạn 8: Terminal-Native Pipe & Tinh Khiết Hóa Codebase (v0.8.x)
* [31-v0.8.3 - Hỗ Trợ Luồng Nghịch (Guest TX → Host RX) & Shortlink Uplink](31-v0.8.3-reverse-flow-uplink-and-guest-tx-shortlink.md) - *(2026-11-19)* 🚀 **Mới nhất**
* [30-v0.8.2 - Tối Ưu Hóa Máy Ảo, Delivered ACK & Hỗ Trợ Alpine Linux (wget)](30-v0.8.2-realworld-vm-hardening-and-delivery-ack.md) - *(2026-11-19)*
* [29-v0.8.1 - Gọt Tỉa Codebase & Đóng Băng Phạm Vi (Scope Freeze)](29-v0.8.1-codebase-pruning-and-minimalist-hardening.md) - *(2026-11-19)*
* [28-v0.8.0 - Terminal-Native Pipe & Rút Gọn Link Tuyển Chọn](28-v0.8.0-ultra-simplified-terminal-pipe-and-selective-shortlink.md) - *(2026-11-19)*

### ⚡ Giai Đoạn 7: Turbo Transfer Engine - Vi Khung Sub-Chunk & Pipelining (v0.7.x)
* [27-v0.7.2 - Cổng Shortlink s.wd.io.vn & Tích Hợp FastHost](27-v0.7.2-shortlink-gateway-swdio-and-fasthost-integration.md) - *(2026-11-19)*
* [26-v0.7.1 - Tải File Nền Cho Thiết Bị Di Động & Screen Wake Lock](26-v0.7.1-mobile-native-download-and-wakelock.md) - *(2026-11-18)*
* [25-v0.7.0 - Động Cơ Tăng Tốc Turbo HTTP Core](25-v0.7.0-turbo-http-core-subchunk-and-pipelining.md) - *(2026-11-18)*

### 📦 Giai Đoạn 6: 16MB Chunked Dual-Buffering Backpressure Pipe (v0.6.x)
* [24-v0.6.2 - Sửa Lỗi Trắng Trang Chủ (/#), Stream UI Guards & Hỗ Trợ Tên File UTF-8](24-v0.6.2-fix-blank-home-js-syntax-and-stream-ui-guard.md) - *(2026-11-18)*
* [23-v0.6.1 - Sửa Lỗi 0-Byte Single-Batch & Dashboard Đa Mảnh TX/RX](23-v0.6.1-fix-single-batch-and-visual-multi-batch-dashboard.md) - *(2026-11-18)*
* [22-v0.6.0 - 16MB Chunked Dual-Buffering Backpressure Pipe (Zero-Disk Streaming)](22-v0.6.0-chunked-dual-buffering-backpressure-pipe.md) - *(2026-11-17)*

### 🌐 Giai Đoạn 5: FastHost CLI, Windows Portable & Tự Động Hóa Cloudflare Tunnel (v0.5.x)
* [21-v0.5.5 - Upload Dashboard Đo Tiến Trình & Tinh Giản Giao Diện Web Nhận (RX)](21-v0.5.5-upload-dashboard-and-clean-rx-ui.md) - *(2026-11-17)*
* [20-v0.5.4 - Fix Cloudflare Quick Tunnel SSE Buffering & Hybrid Snapshot Sync](20-v0.5.4-fix-sse-buffering-hybrid-sync.md) - *(2026-11-17)*
* [19-v0.5.3 - Phá Đệm Cloudflare SSE Buffering & Đồng Bộ Thời Gian Thực 2 Đầu](19-v0.5.3-fix-cloudtunnel-sse-matching.md) - *(2026-11-17)*
* [18-v0.5.2 - Native Go FastHost Windows & Phân Tách Thư Mục dist/](18-v0.5.2-native-portable-windows-dist.md) - *(2026-11-17)*
* [17-v0.5.1 - Tinh Chỉnh UX Console FastHost & Bổ Sung Cờ -port](17-v0.5.1-fasthost-ux-hub-link-clean-sample.md) - *(2026-11-17)*
* [16-v0.5.0 - Bổ Sung Công Cụ FastHost CLI (Tự Động Hóa Cloudflare Tunnel)](16-v0.5.0-fasthost-cli-tunnel.md) - *(2026-11-17)*

### 🎨 Giai Đoạn 4: Đại Tu Giao Diện Retro Monochrome Minimalist & Form Thống Nhất (v0.4.x)
* [15-v0.4.5 - Bổ Sung Hướng Dẫn TX CLI Trực Quan Trên Web Client](15-v0.4.5-client-tx-cli-guide.md) - *(2026-11-17)*
* [14-v0.4.4 - Sửa Triệt Để Lỗi CSS Khi Deploy Lên Render (MIME & Cache Fix)](14-v0.4.4-fix-render-css-mime-and-cache.md) - *(2026-11-17)*
* [13-v0.4.3 - Tinh Giản Diễn Giải, Slug 4 Ký Tự Ngẫu Nhiên & Cheatsheet 3 Cột Thực Chiến](13-v0.4.3-clean-ui-actionable-cheatsheet.md) - *(2026-11-17)*
* [12-v0.4.2 - Mở Rộng Bề Ngang 92% Toàn Màn Hình & Sửa Lỗi Co Cụm Layout Flexbox](12-v0.4.2-widescreen-layout-flex-fix.md) - *(2026-11-17)*
* [11-v0.4.1 - Form Kế Thừa Thống Nhất & Triệt Tiêu Biến Thiên Kích Thước (Zero Jitter)](11-v0.4.1-unified-form-zero-jitter.md) - *(2026-11-17)*
* [10-v0.4.0 - Đại Tu Giao Diện Retro Monochrome & Tối Giản Cực Hạn Mã Nguồn](10-v0.4.0-retro-minimalist-monochrome-ui.md) - *(2026-11-17)*

### 🔒 Giai Đoạn 3: Khóa Kết Nối Vật Lý 1-1 & Đổi Tên WireDrop (v0.3.x)
* [09-v0.3.4 - Tối Giản Ngôn Ngữ & Trải Nghiệm Giao Diện (TX / RX)](09-v0.3.4-ui-ux-refinement.md) - *(2026-11-16)*
* [08-v0.3.3 - Trang Chủ Điều Phối (/#) & Tối Giản Giao Diện Bên Nhận](08-v0.3.3-landing-page-and-minimal-ui.md) - *(2026-11-16)*
* [07-v0.3.2 - Đổi Tên Dự Án Thành WireDrop](07-v0.3.2-rename-to-wiredrop.md) - *(2026-11-16)*
* [06-v0.3.1 - Triển Khai Khóa Kết Nối Vật Lý Thuần Túy](06-v0.3.1-physical-connection-lock-impl.md) - *(2026-11-16)*
* [05-v0.3.0 - Đặc Tả Khóa Kết Nối Vật Lý Thuần Túy](05-v0.3.0-physical-connection-lock-spec.md) - *(2026-11-16)*

### 🔌 Giai Đoạn 2: Tái Định Hình Kiến Trúc Sợi Dây 2 Đầu Thuần 1 Chiều (v0.2.x)
* [04-v0.2.1 - Hiện Thực Hóa Sợi Dây 2 Đầu Thuần 1 Chiều](04-v0.2.1-unidirectional-pipe-impl.md) - *(2026-11-16)*
* [03-v0.2.0 - Tái Định Hình Kiến Trúc "Sợi Dây 2 Đầu Thuần 1 Chiều"](03-v0.2.0-unidirectional-pipe-spec.md) - *(2026-11-16)*

### ⚙️ Giai Đoạn 1: Khởi Tạo Dự Án & Kiến Trúc Cốt Lõi MVP (v0.1.x)
* [02-v0.1.1 - Hoàn Thiện Mã Nguồn Go Cốt Lõi](02-v0.1.1-core-implementation.md) - *(2026-11-16)*
* [01-v0.1.0 - Khởi Tạo Kiến Trúc & Quy Tắc Phát Triển](01-v0.1.0-init-spec.md) - *(2026-11-16)*

---
*Ghi chú: Mọi bản cập nhật mới bắt buộc phải tuân theo chuẩn Keep a Changelog Tinh Khiết trong [CLAUDE.md](../CLAUDE.md).*
