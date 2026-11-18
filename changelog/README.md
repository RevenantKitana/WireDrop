# LỊCH SỬ THAY ĐỔI DỰ ÁN (PROJECT CHANGELOGS)

Thư mục này lưu trữ toàn bộ các file changelog chi tiết của dự án **WireDrop**, được phân tách theo từng mốc phát triển theo chuẩn Semantic Versioning trong hệ tiền phát hành (`v0.X.Y`).

Mọi file được đặt tên theo quy chuẩn: `NN-v0.X.Y-<slug>.md` để đảm bảo thứ tự sắp xếp thời gian tự nhiên và nhất quán trên mọi hệ điều hành.

---

## Danh Sách Changelog Theo Các Giai Đoạn Phát Triển:

### 📦 Giai Đoạn 6: 16MB Chunked Dual-Buffering Backpressure Pipe (v0.6.x)
* [24-v0.6.2 - Khắc Phục Lỗi Trắng Trang Chủ (/#), Bảo Vệ Giao Diện Stream TX/RX & Hỗ Trợ Tên File Tiếng Việt UTF-8](24-v0.6.2-fix-blank-home-js-syntax-and-stream-ui-guard.md) - *(2026-11-18)* 🚀 **Mới nhất (State hiện tại)**
* [23-v0.6.1 - Sửa Triệt Để Lỗi 0-Byte Single-Batch & Dashboard Trực Quan Hóa Đa Mảnh Cho Cả TX & RX](23-v0.6.1-fix-single-batch-and-visual-multi-batch-dashboard.md) - *(2026-11-18)*
* [22-v0.6.0 - 16MB Chunked Dual-Buffering Backpressure Pipe (Zero-Disk Large File Streaming)](22-v0.6.0-chunked-dual-buffering-backpressure-pipe.md) - *(2026-11-17)*

### 🌐 Giai Đoạn 5: FastHost CLI, Windows Portable & Tự Động Hóa Cloudflare Tunnel (v0.5.x)
* [21-v0.5.5 - Upload Dashboard Đo Tiến Trình & Tinh Giản Giao Diện Web Nhận (RX)](21-v0.5.5-upload-dashboard-and-clean-rx-ui.md) - *(2026-11-17)*
* [20-v0.5.4 - Fix Cloudflare Quick Tunnel SSE Buffering & Hybrid Snapshot Sync](20-v0.5.4-fix-sse-buffering-hybrid-sync.md) - *(2026-11-17)*
* [19-v0.5.3 - Khắc phục nghẽn đệm Cloudflare SSE Buffering & Đồng bộ hóa thời gian thực 2 đầu](19-v0.5.3-fix-cloudtunnel-sse-matching.md) - *(2026-11-17)*
* [18-v0.5.2 - Đóng gói Native Go Portable Windows All-in-One & Phân tách thư mục dist/](18-v0.5.2-native-portable-windows-dist.md) - *(2026-11-17)*
* [17-v0.5.1 - Tinh chỉnh UX Console FastHost & Backend: Link Trang chủ # và chú thích Phòng mẫu ví dụ](17-v0.5.1-fasthost-ux-hub-link-clean-sample.md) - *(2026-11-17)*
* [16-v0.5.0 - Bổ sung công cụ FastHost CLI tự động hóa Cloudflare Tunnel Cross-Platform](16-v0.5.0-fasthost-cli-tunnel.md) - *(2026-11-17)*

### 🎨 Giai Đoạn 4: Đại Tu Giao Diện Retro Monochrome Minimalist & Form Thống Nhất (v0.4.x)
* [15-v0.4.5 - Bổ sung hướng dẫn TX CLI trực quan trên Web Client](15-v0.4.5-client-tx-cli-guide.md) - *(2026-11-17)*
* [14-v0.4.4 - Sửa triệt để lỗi CSS không áp dụng khi deploy lên Render (MIME & Cache Fix)](14-v0.4.4-fix-render-css-mime-and-cache.md) - *(2026-11-17)*
* [13-v0.4.3 - Tinh giản diễn giải, Cheatsheet 3 cột & Slug 4 ký tự ngẫu nhiên](13-v0.4.3-clean-ui-actionable-cheatsheet.md) - *(2026-11-17)*
* [12-v0.4.2 - Mở rộng bề ngang 92% toàn màn hình & Sửa lỗi co cụm layout Flexbox](12-v0.4.2-widescreen-layout-flex-fix.md) - *(2026-11-17)*
* [11-v0.4.1 - Form kế thừa thống nhất, thanh lệnh RX đáy & Triệt tiêu biến thiên kích thước (Zero Jitter)](11-v0.4.1-unified-form-zero-jitter.md) - *(2026-11-17)*
* [10-v0.4.0 - Đại tu giao diện Retro Monochrome Minimalist & Tối giản cực hạn mã nguồn](10-v0.4.0-retro-minimalist-monochrome-ui.md) - *(2026-11-17)*

### 🔒 Giai Đoạn 3: Khóa Kết Nối Vật Lý 1-1 & Đổi Tên WireDrop (v0.3.x)
* [09-v0.3.4 - Tối giản ngôn ngữ & Trải nghiệm giao diện (UI/UX Refinement)](09-v0.3.4-ui-ux-refinement.md) - *(2026-11-16)*
* [08-v0.3.3 - Trang chủ điều phối (/#) & Tối giản giao diện bên nhận](08-v0.3.3-landing-page-and-minimal-ui.md) - *(2026-11-16)*
* [07-v0.3.2 - Đổi tên dự án thành WireDrop (Rebranding to WireDrop)](07-v0.3.2-rename-to-wiredrop.md) - *(2026-11-16)*
* [06-v0.3.1 - Triển khai Khóa kết nối vật lý thuần túy (Pure Physical Connection Lock)](06-v0.3.1-physical-connection-lock-impl.md) - *(2026-11-16)*
* [05-v0.3.0 - Đặc tả kiến trúc Khóa kết nối vật lý thuần túy (Pure Physical Connection Lock)](05-v0.3.0-physical-connection-lock-spec.md) - *(2026-11-16)*

### 🔌 Giai Đoạn 2: Tái Định Hình Kiến Trúc Sợi Dây 2 Đầu Thuần 1 Chiều (v0.2.x)
* [04-v0.2.1 - Hiện thực hóa trọn vẹn "Sợi dây 2 đầu thuần 1 chiều"](04-v0.2.1-unidirectional-pipe-impl.md) - *(2026-11-16)*
* [03-v0.2.0 - Tái định hình kiến trúc "Sợi dây 2 đầu thuần 1 chiều"](03-v0.2.0-unidirectional-pipe-spec.md) - *(2026-11-16)*

### ⚙️ Giai Đoạn 1: Khởi Tạo Dự Án & Kiến Trúc Cốt Lõi MVP (v0.1.x)
* [02-v0.1.1 - Hoàn thiện toàn diện mã nguồn Go (Core Implementation)](02-v0.1.1-core-implementation.md) - *(2026-11-16)*
* [01-v0.1.0 - Khởi tạo dự án & Đặc tả kiến trúc Go ban đầu](01-v0.1.0-init-spec.md) - *(2026-11-16)*

---
*Ghi chú: Mọi bản cập nhật mới bắt buộc phải tuân theo quy tắc quản lý changelog trong [CLAUDE.md](../CLAUDE.md).*
