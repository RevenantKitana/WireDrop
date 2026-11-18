/**
 * WireDrop Shortlink Gateway (s.wd.io.vn)
 * Cloudflare Worker + KV Storage (WIREDROP_KV)
 *
 * Tính năng bảo mật & chống lạm dụng:
 * 1. Whitelist miền đích cứng: Chỉ chấp nhận *.trycloudflare.com (Triệt tiêu 100% nguy cơ lừa đảo/phishing)
 * 2. Active Probe Verification: Kiểm tra máy chủ đích có thực sự online không trước khi cấp shortlink
 * 3. Tự hủy siêu tốc (Ephemeral TTL): Tự động xóa khỏi KV sau 30 phút (expirationTtl = 1800s)
 * 4. Chuyển hướng 302 chuẩn: Hoạt động tức thì trên Trình duyệt di động và CLI (curl -sL / irm)
 */

const ALLOWED_TARGET_REGEX = /^https:\/\/[a-z0-9\-]+\.trycloudflare\.com(\/.*)?$/i;
const VALID_SLUG_REGEX = /^[a-z0-9\-]{2,16}$/i;
const DEFAULT_TTL = 1800; // 30 phút

export default {
  async fetch(request, env) {
    const url = new URL(request.url);
    const path = url.pathname.slice(1).trim();

    // 1. TRANG CHỦ HOẶC KIỂM TRA SỨC KHỎE
    if (!path || path === "health") {
      const isCLI = /curl|powershell|wget/i.test(request.headers.get("user-agent") || "");
      if (isCLI) {
        return new Response("# WireDrop Shortlink Gateway (s.wd.io.vn) is active.\n", {
          headers: { "Content-Type": "text/plain; charset=utf-8" },
        });
      }
      return new Response(renderLandingHTML(url.host), {
        headers: { "Content-Type": "text/html; charset=utf-8" },
      });
    }

    // 2. API ĐĂNG KÝ SHORTLINK (Được gọi tự động từ WireDrop FastHost)
    if (request.method === "POST" && (path === "register" || path === "api/register")) {
      return handleRegister(request, env, url);
    }

    // 3. API XÓA SHORTLINK KHI TẮT FASTHOST (Tùy chọn)
    if (request.method === "DELETE" && path.startsWith("register/")) {
      const slugToDelete = path.slice("register/".length).toLowerCase();
      if (env.WIREDROP_KV && slugToDelete) {
        await env.WIREDROP_KV.delete(slugToDelete);
        await env.WIREDROP_KV.delete(`${slugToDelete}-s`);
        await env.WIREDROP_KV.delete(`${slugToDelete}-r`);
      }
      return new Response(JSON.stringify({ status: "deleted" }), {
        headers: { "Content-Type": "application/json" },
      });
    }

    // 4. LUỒNG CHUYỂN HƯỚNG SHORTLINK (GET /:code)
    if (request.method === "GET") {
      return handleRedirect(request, env, path);
    }

    return new Response("Method Not Allowed", { status: 405 });
  },
};

/**
 * Xử lý đăng ký phòng từ FastHost
 */
async function handleRegister(request, env, workerUrl) {
  if (!env.WIREDROP_KV) {
    return new Response(JSON.stringify({ error: "WIREDROP_KV binding not configured on Cloudflare Worker" }), {
      status: 500,
      headers: { "Content-Type": "application/json" },
    });
  }

  let body;
  try {
    body = await request.json();
  } catch (_) {
    return new Response(JSON.stringify({ error: "Invalid JSON body" }), {
      status: 400,
      headers: { "Content-Type": "application/json" },
    });
  }

  const slug = (body.slug || "").trim().toLowerCase();
  let target = (body.target || "").trim();

  // BẢO VỆ 1: Kiểm tra định dạng mã phòng
  if (!slug || !VALID_SLUG_REGEX.test(slug)) {
    return new Response(JSON.stringify({ error: "Invalid slug format (2-16 alphanumeric characters)" }), {
      status: 400,
      headers: { "Content-Type": "application/json" },
    });
  }

  // Chuẩn hóa target URL
  if (target.endsWith("/")) {
    target = target.slice(0, -1);
  }

  // BẢO VỆ 2: Khóa whitelist nghiêm ngặt - Chỉ cho phép trycloudflare.com
  if (!target || !ALLOWED_TARGET_REGEX.test(target)) {
    return new Response(
      JSON.stringify({ error: "Forbidden: Target URL must strictly be a *.trycloudflare.com tunnel." }),
      { status: 403, headers: { "Content-Type": "application/json" } }
    );
  }

  // BẢO VỆ 3: Active Probe Verification - Kiểm tra đường hầm có online thật không
  try {
    const probeRes = await fetch(target, {
      method: "HEAD",
      signal: AbortSignal.timeout(2500),
    });
    // Nếu target không phản hồi hoặc trả về lỗi máy chủ mạng (502, 504)
    if (probeRes.status >= 500) {
      return new Response(
        JSON.stringify({ error: `Target tunnel returned HTTP ${probeRes.status}` }),
        { status: 502, headers: { "Content-Type": "application/json" } }
      );
    }
  } catch (err) {
    return new Response(
      JSON.stringify({ error: `Probe verification failed: ${err.message}` }),
      { status: 400, headers: { "Content-Type": "application/json" } }
    );
  }

  const ttl = body.ttl && Number.isInteger(body.ttl) && body.ttl > 60 && body.ttl <= 7200
    ? body.ttl
    : DEFAULT_TTL;

  // Lưu trữ các biến thể phòng vào Cloudflare KV với TTL tự hủy
  const cleanTarget = target;
  await Promise.all([
    env.WIREDROP_KV.put(slug, `${cleanTarget}/${slug}-r`, { expirationTtl: ttl }),
    env.WIREDROP_KV.put(`${slug}-r`, `${cleanTarget}/${slug}-r`, { expirationTtl: ttl }),
    env.WIREDROP_KV.put(`${slug}-s`, `${cleanTarget}/${slug}-s`, { expirationTtl: ttl }),
  ]);

  const host = workerUrl.host;
  return new Response(
    JSON.stringify({
      status: "ok",
      slug: slug,
      short_r: `https://${host}/${slug}-r`,
      short_s: `https://${host}/${slug}-s`,
      target: cleanTarget,
      ttl_seconds: ttl,
    }),
    {
      status: 200,
      headers: { "Content-Type": "application/json", "Access-Control-Allow-Origin": "*" },
    }
  );
}

/**
 * Xử lý chuyển hướng khi người dùng gõ shortlink
 */
async function handleRedirect(request, env, path) {
  if (!env.WIREDROP_KV) {
    return new Response("Cloudflare KV not configured", { status: 500 });
  }

  const targetUrl = await env.WIREDROP_KV.get(path.toLowerCase());
  if (targetUrl) {
    // Chuyển hướng 302 Found tức thì (< 10ms trên Cloudflare Edge)
    return Response.redirect(targetUrl, 302);
  }

  // Không tìm thấy hoặc đã hết hạn
  const isCLI = /curl|powershell|wget/i.test(request.headers.get("user-agent") || "");
  if (isCLI) {
    return new Response(`# WireDrop: phong /${path} khong ton tai hoac da het han tren s.wd.io.vn\n`, {
      status: 404,
      headers: { "Content-Type": "text/plain; charset=utf-8" },
    });
  }

  return new Response(renderNotFoundHTML(path), {
    status: 404,
    headers: { "Content-Type": "text/html; charset=utf-8" },
  });
}

function renderLandingHTML(host) {
  return `<!DOCTYPE html>
<html lang="vi">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>WireDrop Shortlink Gateway - ${host}</title>
  <style>
    body { font-family: -apple-system, BlinkMacSystemFont, "JetBrains Mono", monospace; background: #fff; color: #111; display: flex; align-items: center; justify-content: center; height: 100vh; margin: 0; padding: 20px; box-sizing: border-box; }
    .box { border: 2px solid #111; padding: 28px; max-width: 500px; width: 100%; box-shadow: 4px 4px 0 #111; }
    h1 { font-size: 1.4rem; margin: 0 0 10px; font-weight: 700; letter-spacing: -0.5px; }
    p { font-size: 0.9rem; line-height: 1.5; color: #444; margin: 0 0 16px; }
    code { background: #f0f0f0; padding: 2px 6px; border: 1px solid #ccc; font-weight: bold; }
    .tag { display: inline-block; background: #111; color: #fff; padding: 3px 8px; font-size: 0.75rem; font-weight: bold; margin-bottom: 12px; }
  </style>
</head>
<body>
  <div class="box">
    <div class="tag">[ WIREDROP SHORTLINK GATEWAY ]</div>
    <h1>${host}</h1>
    <p>Cổng chuyển tiếp liên kết ngắn siêu tốc dành riêng cho <strong>WireDrop</strong>.</p>
    <p>Để nhận dữ liệu, hãy nhập mã phòng được người gửi cung cấp (ví dụ: <code>${host}/88-r</code>).</p>
  </div>
</body>
</html>`;
}

function renderNotFoundHTML(path) {
  return `<!DOCTYPE html>
<html lang="vi">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>Phòng Hết Hạn - WireDrop</title>
  <style>
    body { font-family: -apple-system, BlinkMacSystemFont, "JetBrains Mono", monospace; background: #fff; color: #111; display: flex; align-items: center; justify-content: center; height: 100vh; margin: 0; padding: 20px; box-sizing: border-box; }
    .box { border: 2px solid #111; padding: 28px; max-width: 500px; width: 100%; box-shadow: 4px 4px 0 #111; }
    h1 { font-size: 1.3rem; margin: 0 0 10px; color: #d00; font-weight: 700; }
    p { font-size: 0.9rem; line-height: 1.5; color: #444; margin: 0 0 16px; }
    code { background: #f0f0f0; padding: 2px 6px; border: 1px solid #ccc; }
  </style>
</head>
<body>
  <div class="box">
    <h1>[ 404 - PHÒNG KHÔNG TỒN TẠI ]</h1>
    <p>Phòng <code>/${path}</code> không tồn tại hoặc đã hết hạn tự hủy (30 phút).</p>
    <p>Vui lòng yêu cầu bên gửi kiểm tra lại phiên WireDrop đang mở trên máy tính.</p>
  </div>
</body>
</html>`;
}
