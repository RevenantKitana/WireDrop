# QUY TẮC PHÁT TRIỂN DỰ ÁN (PROJECT RULES)
*File này đồng bộ với [CLAUDE.md](../CLAUDE.md) tại thư mục gốc.*

## 1. QUY TẮC QUẢN LÝ CHANGELOG (BẮT BUỘC)
* **Thư mục lưu trữ:** Toàn bộ changelog PHẢI được đặt trong thư mục `changelog/`.
* **Quy tắc ngắt file (File Splitting):**
  * Không dồn toàn bộ lịch sử thay đổi vào một file duy nhất.
  * Phải ngắt file theo từng phiên bản / mốc tính năng (milestone) hoặc theo định dạng ngày-tính năng:
    * Ví dụ: `changelog/v0.1.0-init-core.md`, `changelog/v0.2.0-stream-pipe.md`, `changelog/v0.3.0-file-transfer.md`,...
  * Thư mục `changelog/` phải có file `changelog/README.md` làm bảng mục lục (Index) liên kết tới tất cả các file changelog con.
* **Cấu trúc chuẩn của một file changelog (Keep a Changelog Tinh Khiết):**
  * Tuyệt đối không ghi: Tác giả, slogan/kim chỉ nam, bài luận bối cảnh/điểm mù (thuộc về `spec/`), kế hoạch tương lai (thuộc về Roadmap) hay kiểm thử dài dòng.
  * Chỉ gồm 2 phần ngắn gọn:
    1. **Tiêu đề & Ngày:** `# WireDrop v0.X.Y - <Tên Tính Năng>` kèm ngày phát hành (`**Ngày:** YYYY-MM-DD`).
    2. **Danh mục thay đổi kỹ thuật (Bullet points trực diện):**
       * `Added`: File mới, hàm mới, endpoint mới, thành phần UI mới.
       * `Changed`: Cải tiến logic, refactor mã nguồn.
       * `Fixed`: Các lỗi/bug được xử lý.
       * `Removed`: Gọt tỉa, xóa bỏ tính năng hoặc endpoint thừa.

---

## 2. QUY TẮC LẬP TRÌNH VÀ KIẾN TRÚC CODE (GOLANG)
* **Ngôn ngữ & Runtime:** Go 1.22+ trở lên.
* **Thư viện chuẩn (Zero-Dependency):**
  * Sử dụng 100% Go Standard Library (`net/http`, `embed`, `sync`, `io`, `time`, `bytes`, `os`).
  * Tuyệt đối không thêm thư viện ngoài (third-party dependencies) trừ khi có yêu cầu cụ thể từ người dùng.
* **Tổ chức mã nguồn:**
  * Ngắt file logic rõ ràng, tránh dồn hàng nghìn dòng vào `main.go`.
  * Các file quy định:
    * `main.go`: Khởi động server, middleware, định tuyến ServeMux.
    * `room.go`: Quản lý struct `RoomState`, mutex locking, TTL cleaner.
    * `handler_text.go`: Xử lý text/script, Content-Negotiation, chuẩn hóa CRLF.
    * `handler_stream.go`: Xử lý Live Terminal Pipe (`http.Flusher`, heartbeat ping).
    * `handler_file.go`: Xử lý Upload/Download file, Zero-RAM Pipe.
    * `public/`: Chứa Web SPA (`index.html`, `style.css`, `app.js`) được nhúng bằng `//go:embed`.
* **An toàn bộ nhớ và Concurrency:**
  * Mọi truy cập vào trạng thái phòng trong RAM phải được bảo vệ bằng `sync.RWMutex`.
  * Tránh rò rỉ Goroutine: Mọi Goroutine chạy nền (như Ticker, SSE, Stream) phải có cơ chế thoát an toàn (context cancellation hoặc channel close).
