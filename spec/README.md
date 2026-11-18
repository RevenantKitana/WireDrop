# KHO TÀI LIỆU ĐẶC TẢ KIẾN TRÚC (ARCHITECTURAL SPECIFICATIONS)

Thư mục `spec/` lưu trữ toàn bộ các tài liệu **Chuẩn bị Đặc tả (`SPEC-PREPARE` / RFC)** và **Đặc tả Kiến trúc Chính thức (`APPROVED SPEC`)** của dự án **WireDrop**.

Mọi tài liệu được đặt tên theo quy chuẩn sắp xếp tự nhiên (Sort-Safe Naming): `NN-<slug-kien-truc>.md`.

---

## 📌 Vòng Đời Của Một Tài Liệu Đặc Tả (Spec Lifecycle)

1. **`📝 SPEC-PREPARE` (Chuẩn bị & Thẩm định thiết kế):**
   * Phân tích bài toán, giải phẫu điểm nghẽn, so sánh các phương án kiến trúc, rà soát các ca biên (edge cases) và lập bảng câu hỏi quyết định (*Decision Matrix*).
   * Chưa viết code tính năng cho đến khi các câu hỏi quyết định trong `SPEC-PREPARE` được chốt.
2. **`✅ APPROVED` (Đã phê duyệt thiết kế):**
   * Các thông số kỹ thuật, giao thức, cấu trúc dữ liệu đã được khóa để tiến hành lập trình và viết Unit/Integration Test.
3. **`🚀 IMPLEMENTED` (Đã triển khai thực tế):**
   * Đã hoàn thiện mã nguồn, vượt qua kiểm thử tự động và phát hành vào nhánh chính (`main`).

---

## 📚 Danh Mục Tài Liệu Đặc Tả Hiện Có

| Mã số | Tài liệu Đặc tả | Giai đoạn / Phiên bản | Trạng thái | Tóm tắt Nội dung |
| :-: | :--- | :---: | :---: | :--- |
| **01** | [01-core-two-ended-wire-and-physical-lock.md](01-core-two-ended-wire-and-physical-lock.md) | `v0.1.0` $\rightarrow$ `v0.5.x` | 🚀 **IMPLEMENTED** | Kiến trúc nền tảng: Mô hình "Sợi dây 2 đầu thuần 1 chiều" (`*-s` / `*-r`), Khóa kết nối vật lý 1-1 (`HTTP 409`), Single Atomic Slot và Burn-After-Read. |
| **02** | [02-chunked-16mb-ping-pong-pipe.md](02-chunked-16mb-ping-pong-pipe.md) | `v0.6.0` $\rightarrow$ `v0.6.2` | 🚀 **IMPLEMENTED** | Truyền file lớn Zero-Disk qua bộ đệm đôi `16MB` Ping-Pong có điều tiết áp lực ngược (Backpressure), vượt giới hạn 100MB Cloudflare, kèm Multi-Batch Dashboard TX/RX. |
| **03** | [03-specprepare-turbo-transfer-optimization.md](03-specprepare-turbo-transfer-optimization.md) | `v0.7.0` | ✅ **APPROVED** | Kế hoạch tăng tốc truyền file thế hệ mới: Pha 1 (`256KB` Sub-Chunk Streaming + `2-Stream` Parallel Pipelining + Tunnel `HTTP/2` + Bậc mảnh 8MB/16MB/32MB). |
| **04** | [04-specprepare-terminal-pipe-and-selective-shortlink.md](04-specprepare-terminal-pipe-and-selective-shortlink.md) | `v0.8.0-dev` | 📝 **SPEC-PREPARE** | Đặc tả Terminal-Native Pipe (`wiredrop open <room>` như SSH/netcat pipe trực tiếp) & Cơ chế Rút gọn Link Tuyển chọn (Opt-In Shortlink `s.wd.io.vn` trên CLI và Web). |

---

*Ghi chú: Quy tắc quản lý thư mục `spec/` được quy định đồng bộ tại [CLAUDE.md](../CLAUDE.md).*
