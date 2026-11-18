# Hướng Dẫn Triển Khai WireDrop Shortlink Gateway (`s.wd.io.vn`)

Tài liệu này hướng dẫn cài đặt **Cloudflare Worker** cho tên miền `s.wd.io.vn` hoàn toàn miễn phí 100% trong 2 phút.

---

## Cách 1: Triển Khai Bằng Giao Diện Web (Cloudflare Dashboard) — Đơn giản nhất

1. **Tạo Cloudflare KV Namespace**:
   - Đăng nhập vào [Cloudflare Dashboard](https://dash.cloudflare.com)
   - Chọn menu bên trái: **Workers & Pages** -> **KV**
   - Bấm **Create a namespace** -> Đặt tên: `WIREDROP_KV` -> Bấm **Add**.

2. **Tạo Cloudflare Worker**:
   - Chọn menu **Workers & Pages** -> **Overview** -> Bấm **Create application** -> **Create Worker**.
   - Đặt tên: `wiredrop-shortlink-gateway` -> Bấm **Deploy**.
   - Sau khi deploy xong, bấm **Edit code**:
     - Xóa toàn bộ code mặc định.
     - Copy toàn bộ nội dung trong file [worker.js](worker.js) dán vào.
     - Bấm **Deploy** ở góc trên bên phải.

3. **Gắn KV Namespace vào Worker**:
   - Quay lại trang quản lý Worker `wiredrop-shortlink-gateway`.
   - Vào thẻ **Settings** -> **Variables and Secrets** (hoặc **Bindings** tùy giao diện mới).
   - Tại mục **KV Namespace Bindings** -> Bấm **Add binding**:
     - Variable name: `WIREDROP_KV` (Bắt buộc viết hoa chính xác).
     - KV namespace: Chọn `WIREDROP_KV` vừa tạo ở Bước 1.
   - Bấm **Save and deploy**.

4. **Gắn Custom Domain `s.wd.io.vn`**:
   - Trong trang quản lý Worker, vào thẻ **Settings** -> **Domains & Routes**.
   - Tại mục **Custom Domains** -> Bấm **Add Custom Domain**.
   - Nhập: `s.wd.io.vn` -> Bấm **Add Custom Domain**.
   - Cloudflare sẽ tự động tạo bản ghi DNS và cấp chứng chỉ SSL HTTPS miễn phí trong vài giây!

---

## Cách 2: Triển Khai Bằng Dòng Lệnh (Wrangler CLI)

Nếu bạn có cài đặt Node.js và Wrangler:

```bash
cd gateway

# 1. Tạo KV namespace
npx wrangler kv:namespace create WIREDROP_KV

# 2. Copy ID trả về dán vào file wrangler.toml tại dòng: id = "<YOUR_KV_NAMESPACE_ID>"

# 3. Deploy lên Cloudflare
npx wrangler deploy
```

---

## Kiểm Tra Hoạt Động (Verification)

Mở trình duyệt truy cập:
`https://s.wd.io.vn`
Nếu màn hình hiển thị: **[ WIREDROP SHORTLINK GATEWAY ] - s.wd.io.vn** là đã thành công 100%!
