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

### 1.3. Cấu Trúc Chuẩn Của File Changelog (Chuẩn Keep a Changelog Tinh Khiết)
Mỗi file changelog tuân thủ nghiêm ngặt tinh thần tinh khiết (Pure Technical Changelog) — đúng với tác dụng ghi nhận thay đổi kỹ thuật, triệt tiêu văn mẫu và hình thức không cần thiết:
* **Tuyệt đối không ghi:** Tác giả, slogan/kim chỉ nam, bài luận phân tích bối cảnh/điểm mù (nội dung này thuộc về `spec/`), kế hoạch tương lai (thuộc về Roadmap), hay báo cáo kiểm thử dài dòng.
* **Cấu trúc bắt buộc (Chỉ gồm 2 phần ngắn gọn):**
  1. **Tiêu đề & Ngày:** `# WireDrop v0.X.Y - <Tên Tính Năng>` kèm ngày phát hành (`**Ngày:** YYYY-MM-DD`).
  2. **Danh mục thay đổi kỹ thuật (Bullet points trực diện):**
     * `Added`: Thêm mới tính năng, file, endpoint, cờ lệnh, UI element.
     * `Changed`: Cải tiến logic, refactor, cập nhật cấu trúc dữ liệu.
     * `Fixed`: Sửa lỗi crash, bug logic, sửa CSS MIME/cache, lỗi tương thích OS.
     * `Removed`: Gọt tỉa, xóa bỏ tính năng thừa, đóng băng phạm vi.

### 1.4. Single Source of Truth cho Phiên Bản
* Hằng số phiên bản duy nhất được định nghĩa trong [main.go](main.go): `const Version = "v0.X.Y"`.
* Toàn bộ banner terminal, log khởi động, giao diện Web và các tài liệu đều phải đồng bộ chính xác với hằng số này.

### 1.5. Quy Tắc Quản Lý Tài Liệu Đặc Tả Kiến Trúc (`spec/`)
* Toàn bộ tài liệu thiết kế kiến trúc và chuẩn bị đặc tả (`SPEC-PREPARE` / `APPROVED SPEC`) được gom tập trung trong thư mục `spec/` (tuyệt đối không để rời rạc ngoài thư mục gốc).
* **Định dạng đặt tên (Sort-Safe):** `spec/NN-<slug-kien-truc>.md` (ví dụ: `spec/03-specprepare-turbo-transfer-optimization.md`).
* **Quy trình 2 bước trước mọi tính năng lớn (`MINOR v0.X.0`):**
  1. Viết `spec/NN-specprepare-<slug>.md` để phân tích điểm nghẽn, các ca biên (edge cases) và bảng câu hỏi quyết định (*Decision Matrix*).
  2. Sau khi thống nhất phương án, cập nhật trạng thái sang `APPROVED SPEC` và liên kết đầy đủ trong [spec/README.md](spec/README.md).

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
