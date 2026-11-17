# CLAUDE.md - Quy Tắc Quản Lý Dự Án & Chuẩn Phát Triển WireDrop

---

## 1. QUY TẮC QUẢN LÝ PHIÊN BẢN & CHANGELOG (BẮT BUỘC)

### 1.1. Chuẩn Semantic Versioning Trong Hệ Tiền Phát Hành (`v0.X.Y`)
Dự án tuân thủ nghiêm ngặt chuẩn Semantic Versioning:
* **MINOR (`v0.X.0`):** Nảy số khi ra mắt một **Giai đoạn Kiến trúc hoặc Tính năng Độc Lập Lớn** (ví dụ: `v0.4.0` Đại tu Retro Monochrome UI, `v0.5.0` FastHost CLI Tooling, `v0.6.0` Large File Chunked Transfer).
* **PATCH (`v0.X.Y` với $Y > 0$):** Nảy số cho các **Bản vá lỗi, Tinh chỉnh Giao diện, Cải thiện UX hoặc Tối ưu Hiệu năng** trong cùng một giai đoạn tính năng (ví dụ: `v0.5.1` Tinh chỉnh UX Hub Link).
* **MAJOR (`v1.0.0`):** Sẽ được kích hoạt khi WireDrop đạt độ hoàn thiện 100% production-ready toàn diện (bao gồm cả Chunked Transfer, Zero-Storage Streaming và đóng gói nhị phân độc lập).

### 1.2. Quy Tắc Đặt Tên File Changelog (Sort-Safe Naming)
Để đảm bảo toàn bộ lịch sử thay đổi luôn được sắp xếp theo đúng trình tự thời gian trên mọi hệ điều hành (Windows, Linux, macOS) và GitHub:
* **Định dạng bắt buộc:** `changelog/NN-v0.X.Y-<slug-ngan-gon>.md`
  * `NN`: Số thứ tự tăng dần 2 chữ số (`01`, `02`, ..., `17`, `18`, `19`...).
  * `v0.X.Y`: Phiên bản tương ứng của đợt cập nhật.
  * `<slug-ngan-gon>`: Mô tả ngắn gọn bằng tiếng Anh hoặc tiếng Việt không dấu (dùng dấu gạch ngang `-`).
  * Ví dụ: `changelog/17-v0.5.1-fasthost-ux-hub-link-clean-sample.md`.
* **Cập nhật Bảng Mục Lục:** Mọi file changelog mới PHẢI được liên kết ngay vào [changelog/README.md](changelog/README.md).

### 1.3. Cấu Trúc Chuẩn Của File Changelog (Chuẩn Keep a Changelog)
Mỗi file changelog bắt buộc phải có các mục sau:
1. **Tiêu đề & Phiên bản:** `# WireDrop Changelog - v0.X.Y (<Tên Tính Năng>)`
2. **Thông tin metadata:** Thời gian phát hành, phạm vi áp dụng.
3. **Mục tiêu thay đổi (Objective / Context):** Vấn đề cần giải quyết, lý do thay đổi.
4. **Chi tiết thay đổi:**
   * `Added`: Thêm mới file, hàm, giao diện, endpoint hoặc công cụ.
   * `Changed`: Cải tiến logic, refactor, thay đổi cấu trúc dữ liệu.
   * `Fixed`: Sửa lỗi crash, sửa bug logic, sửa CSS MIME/cache, sửa tương thích OS.
   * `Security`: Cải tiến bảo mật hoặc an toàn dữ liệu.
5. **Hướng dẫn kiểm thử (Verification):** Các lệnh chạy thử cụ thể để kiểm tra tính năng.

### 1.4. Single Source of Truth cho Phiên Bản
* Hằng số phiên bản duy nhất được định nghĩa trong [main.go](main.go): `const Version = "v0.X.Y"`.
* Toàn bộ banner terminal, log khởi động, giao diện Web và các tài liệu đều phải đồng bộ chính xác với hằng số này.

---

## 2. QUY TẮC LẬP TRÌNH & KIẾN TRÚC CODE (GOLANG)

* **Ngôn ngữ & Runtime:** Go 1.22+ trở lên.
* **Thư viện chuẩn (Zero-Dependency):**
  * Sử dụng 100% Go Standard Library (`net/http`, `embed`, `sync`, `io`, `time`, `bytes`, `os`, `flag`).
  * Tuyệt đối không thêm thư viện bên ngoài (third-party dependencies) trừ khi có yêu cầu cụ thể từ người dùng.
* **Tổ chức mã nguồn:**
  * Ngắt file logic rõ ràng, tránh dồn hàng nghìn dòng vào `main.go`.
  * `main.go`: Khởi động server, đọc flag/env, middleware, định tuyến ServeMux.
  * `room.go`: Quản lý struct `RoomState`, mutex locking, TTL cleaner.
  * `handler_pipe.go`: Xử lý toàn bộ logic chuyển tiếp của Sợi dây 2 đầu (`*-s` vs `*-r`).
  * `public/`: Chứa Web SPA (`index.html`, `style.css`, `app.js`) được nhúng bằng `//go:embed`.
* **An toàn bộ nhớ và Concurrency:**
  * Mọi truy cập vào trạng thái phòng trong RAM phải được bảo vệ bằng `sync.RWMutex`.
  * Tránh rò rỉ Goroutine: Mọi Goroutine chạy nền (như Ticker, SSE, Stream) phải có cơ chế thoát an toàn (context cancellation hoặc channel close).
