#!/usr/bin/env bash
# ==============================================================================
# WireDrop FastHosting CLI (Linux, macOS, Windows Git Bash / WSL)
# Tự động tải Cloudflare Portable, tự cấu hình .env và điều phối đường hầm
# ==============================================================================

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BIN_DIR="$SCRIPT_DIR/bin"
ENV_FILE="$SCRIPT_DIR/.env"

mkdir -p "$BIN_DIR"

# Màu hiển thị Terminal
BOLD="\033[1m"
GREEN="\033[32m"
YELLOW="\033[33m"
CYAN="\033[36m"
RED="\033[31m"
RESET="\033[0m"

# 1. Nhận diện Hệ điều hành & Kiến trúc
detect_platform() {
    OS_NAME="$(uname -s)"
    ARCH_NAME="$(uname -m)"

    case "$OS_NAME" in
        MINGW*|MSYS*|CYGWIN*)
            PLATFORM_OS="windows"
            BIN_EXT=".exe"
            ;;
        Linux*)
            PLATFORM_OS="linux"
            BIN_EXT=""
            ;;
        Darwin*)
            PLATFORM_OS="darwin"
            BIN_EXT=""
            ;;
        *)
            PLATFORM_OS="linux"
            BIN_EXT=""
            ;;
    esac

    case "$ARCH_NAME" in
        x86_64|amd64) PLATFORM_ARCH="amd64" ;;
        aarch64|arm64) PLATFORM_ARCH="arm64" ;;
        *) PLATFORM_ARCH="amd64" ;;
    esac
}

# 2. Đọc cấu hình từ .env
load_env() {
    PORT="8080"
    CLOUDFLARE_TUNNEL_TOKEN=""
    CUSTOM_DOMAIN=""

    if [ -f "$ENV_FILE" ]; then
        while IFS='=' read -r key val || [ -n "$key" ]; do
            [[ "$key" =~ ^[[:space:]]*# ]] && continue
            [[ -z "$key" ]] && continue
            key="$(echo "$key" | tr -d '[:space:]')"
            val="$(echo "$val" | sed -e 's/^[[:space:]]*//' -e 's/[[:space:]]*$//' -e 's/^["'"'"']//' -e 's/["'"'"']$//')"
            case "$key" in
                PORT) PORT="$val" ;;
                CLOUDFLARE_TUNNEL_TOKEN) CLOUDFLARE_TUNNEL_TOKEN="$val" ;;
                CUSTOM_DOMAIN) CUSTOM_DOMAIN="$val" ;;
            esac
        done < "$ENV_FILE"
    fi
    PORT="${PORT:-8080}"
}

# 3. Quét tìm file thực thi WireDrop đã có
find_wiredrop_bin() {
    CANDIDATES=(
        "$SCRIPT_DIR/wiredrop$BIN_EXT"
        "$SCRIPT_DIR/wiredrop"
        "$SCRIPT_DIR/wiredrop.exe"
        "$BIN_DIR/wiredrop$BIN_EXT"
        "$BIN_DIR/wiredrop"
        "$BIN_DIR/wiredrop.exe"
    )

    for CANDIDATE in "${CANDIDATES[@]}"; do
        if [ -f "$CANDIDATE" ]; then
            chmod +x "$CANDIDATE" 2>/dev/null || true
            WIREDROP_BIN="$CANDIDATE"
            return 0
        fi
    done

    # Kiểm tra trong system PATH
    if command -v wiredrop >/dev/null 2>&1; then
        WIREDROP_BIN="$(command -v wiredrop)"
        return 0
    fi

    echo -e "${RED}[✕] Không tìm thấy file thực thi WireDrop (wiredrop / wiredrop.exe)!${RESET}"
    echo -e "${YELLOW}>> Vui lòng đặt file thực thi WireDrop vào thư mục dự án này.${RESET}"
    exit 1
}

# 4. Tự động tải Cloudflare Portable nếu chưa có
ensure_cloudflared() {
    CF_BIN="$BIN_DIR/cloudflared$BIN_EXT"

    if [ -f "$CF_BIN" ]; then
        return 0
    fi

    if command -v cloudflared >/dev/null 2>&1; then
        CF_BIN="$(command -v cloudflared)"
        return 0
    fi

    echo -e "${CYAN}[*] Đang tự động tải Cloudflare Portable (${PLATFORM_OS}-${PLATFORM_ARCH})...${RESET}"

    BASE_URL="https://github.com/cloudflare/cloudflared/releases/latest/download"

    if [ "$PLATFORM_OS" = "windows" ]; then
        DOWNLOAD_URL="${BASE_URL}/cloudflared-windows-${PLATFORM_ARCH}.exe"
    elif [ "$PLATFORM_OS" = "linux" ]; then
        DOWNLOAD_URL="${BASE_URL}/cloudflared-linux-${PLATFORM_ARCH}"
    elif [ "$PLATFORM_OS" = "darwin" ]; then
        DOWNLOAD_URL="${BASE_URL}/cloudflared-darwin-${PLATFORM_ARCH}.tgz"
    fi

    if command -v curl >/dev/null 2>&1; then
        if [ "$PLATFORM_OS" = "darwin" ]; then
            curl -fsSL "$DOWNLOAD_URL" -o "$BIN_DIR/cf.tgz"
            tar -xzf "$BIN_DIR/cf.tgz" -C "$BIN_DIR"
            rm -f "$BIN_DIR/cf.tgz"
        else
            curl -fsSL "$DOWNLOAD_URL" -o "$CF_BIN"
        fi
    elif command -v wget >/dev/null 2>&1; then
        if [ "$PLATFORM_OS" = "darwin" ]; then
            wget -qO "$BIN_DIR/cf.tgz" "$DOWNLOAD_URL"
            tar -xzf "$BIN_DIR/cf.tgz" -C "$BIN_DIR"
            rm -f "$BIN_DIR/cf.tgz"
        else
            wget -qO "$CF_BIN" "$DOWNLOAD_URL"
        fi
    else
        echo -e "${RED}[✕] Cần có curl hoặc wget để tự động tải Cloudflare Portable!${RESET}"
        exit 1
    fi

    chmod +x "$CF_BIN" 2>/dev/null || true
    echo -e "${GREEN}[✓] Đã tải Cloudflare Portable về: ${CF_BIN}${RESET}"
}

# 5. Tự động cấu hình .env qua Terminal nếu chọn Option 2 mà chưa có
setup_custom_domain_env() {
    echo ""
    echo -e "${BOLD}================================================================${RESET}"
    echo -e "${BOLD}        THIẾT LẬP DOMAIN CÁ NHÂN VỚI CLOUDFLARE TUNNEL          ${RESET}"
    echo -e "${BOLD}================================================================${RESET}"
    echo -e "Để dùng Domain riêng cố định, bạn cần Token từ Cloudflare Dashboard:"
    echo -e " 1. Mở: ${CYAN}https://dash.cloudflare.com/${RESET} > Zero Trust > Networks > Tunnels"
    echo -e " 2. Bấm ${BOLD}Add a tunnel${RESET} > Chọn Cloudflared > Copy chuỗi ${YELLOW}TOKEN${RESET}."
    echo -e " 3. Tại tab Public Hostname, trỏ subdomain của bạn về: ${GREEN}http://localhost:${PORT}${RESET}"
    echo -e "----------------------------------------------------------------"
    echo ""

    read -rp ">> Dán Cloudflare Tunnel Token của bạn (eyJh...): " INPUT_TOKEN
    INPUT_TOKEN="$(echo "$INPUT_TOKEN" | tr -d '[:space:]')"

    if [ -z "$INPUT_TOKEN" ]; then
        echo -e "${RED}[!] Token không được để trống. Hủy cấu hình.${RESET}"
        return 1
    fi

    read -rp ">> Nhập tên domain cá nhân của bạn (ví dụ: w.yourdomain.com): " INPUT_DOMAIN
    INPUT_DOMAIN="$(echo "$INPUT_DOMAIN" | tr -d '[:space:]')"

    cat <<EOF > "$ENV_FILE"
# Cấu hình WireDrop FastHosting (Tự động tạo bởi fasthost CLI)
PORT=${PORT}
CLOUDFLARE_TUNNEL_TOKEN=${INPUT_TOKEN}
CUSTOM_DOMAIN=${INPUT_DOMAIN}
EOF

    echo -e "${GREEN}[✓] Đã lưu cấu hình vào .env thành công!${RESET}"
    load_env
    return 0
}

# 6. Dọn dẹp tiến trình khi bấm Ctrl+C hoặc thoát
CLEANED_UP=0
cleanup() {
    if [ "$CLEANED_UP" -eq 1 ]; then return 0; fi
    CLEANED_UP=1
    echo ""
    echo -e "${YELLOW}[*] Đang đóng đường hầm và dừng máy chủ WireDrop...${RESET}"
    if [ -n "$PID_TUNNEL" ]; then kill "$PID_TUNNEL" 2>/dev/null || true; fi
    if [ -n "$PID_WIRE" ]; then kill "$PID_WIRE" 2>/dev/null || true; fi
    rm -f "$SCRIPT_DIR/.cf_quick.log"
    echo -e "${GREEN}[✓] Đã giải phóng cổng ${PORT}. Thoát an toàn.${RESET}"
}
trap cleanup INT TERM EXIT

# 7. Chạy Option 1: Quick Tunnel (Domain ngẫu nhiên)
run_quick_tunnel() {
    echo -e "${CYAN}[*] Khởi động WireDrop trên cổng ${PORT}...${RESET}"
    "$WIREDROP_BIN" -port "$PORT" >/dev/null 2>&1 &
    PID_WIRE=$!

    sleep 1

    echo -e "${CYAN}[*] Đang mở đường hầm Cloudflare Quick Tunnel (protocol: http2)...${RESET}"
    rm -f "$SCRIPT_DIR/.cf_quick.log"
    "$CF_BIN" tunnel --protocol http2 --url "http://localhost:${PORT}" > "$SCRIPT_DIR/.cf_quick.log" 2>&1 &
    PID_TUNNEL=$!

    TUNNEL_URL=""
    for i in {1..30}; do
        if [ -f "$SCRIPT_DIR/.cf_quick.log" ]; then
            TUNNEL_URL=$(grep -o 'https://[-a-zA-Z0-9.]*\.trycloudflare\.com' "$SCRIPT_DIR/.cf_quick.log" | head -n 1 || true)
            if [ -n "$TUNNEL_URL" ]; then break; fi
        fi
        sleep 0.5
    done

    if [ -z "$TUNNEL_URL" ]; then
        echo -e "${RED}[✕] Không bắt được URL Cloudflare. Log chi tiết:${RESET}"
        cat "$SCRIPT_DIR/.cf_quick.log"
        exit 1
    fi

    RAND_SLUG=$(head /dev/urandom | tr -dc a-z0-9 | head -c 4 2>/dev/null || echo "drop")

    echo ""
    echo -e "${GREEN}================================================================${RESET}"
    echo -e "${BOLD}       ⚡ WIREDROP FASTHOST ONLINE (DOMAIN NGẪU NHIÊN)           ${RESET}"
    echo -e "${GREEN}================================================================${RESET}"
    echo -e " • ${BOLD}Web Soạn Thảo (TX):${RESET}  ${CYAN}${TUNNEL_URL}/${RAND_SLUG}-s${RESET}"
    echo -e " • ${BOLD}Lệnh Máy Đích (RX):${RESET}"
    echo -e "   Linux/macOS:        ${YELLOW}curl -sL ${TUNNEL_URL}/${RAND_SLUG}-r | sh${RESET}"
    echo -e "   Windows PowerShell: ${YELLOW}irm ${TUNNEL_URL}/${RAND_SLUG}-r | iex${RESET}"
    echo -e " • ${BOLD}Tải File (RX):${RESET}      ${YELLOW}curl -sLJO ${TUNNEL_URL}/${RAND_SLUG}-r${RESET}"
    echo -e "----------------------------------------------------------------"
    echo -e " ${BOLD}[Nhấn Ctrl + C để dừng và đóng đường hầm bất cứ lúc nào]${RESET}"
    echo -e "${GREEN}================================================================${RESET}"
    echo ""

    wait "$PID_TUNNEL" 2>/dev/null || true
}

# 8. Chạy Option 2: Custom Domain (Named Tunnel)
run_custom_tunnel() {
    if [ -z "$CLOUDFLARE_TUNNEL_TOKEN" ]; then
        setup_custom_domain_env || exit 1
    fi

    echo -e "${CYAN}[*] Khởi động WireDrop trên cổng ${PORT}...${RESET}"
    "$WIREDROP_BIN" -port "$PORT" >/dev/null 2>&1 &
    PID_WIRE=$!

    sleep 1

    echo -e "${CYAN}[*] Đang kết nối Cloudflare Named Tunnel theo Token (protocol: http2)...${RESET}"
    "$CF_BIN" tunnel --protocol http2 run --token "$CLOUDFLARE_TUNNEL_TOKEN" >/dev/null 2>&1 &
    PID_TUNNEL=$!

    RAND_SLUG=$(head /dev/urandom | tr -dc a-z0-9 | head -c 4 2>/dev/null || echo "drop")
    DISPLAY_HOST="${CUSTOM_DOMAIN:-<your-domain>}"

    echo ""
    echo -e "${GREEN}================================================================${RESET}"
    echo -e "${BOLD}       ⚡ WIREDROP FASTHOST ONLINE (DOMAIN CÁ NHÂN CỐ ĐỊNH)       ${RESET}"
    echo -e "${GREEN}================================================================${RESET}"
    echo -e " • ${BOLD}Web Soạn Thảo (TX):${RESET}  ${CYAN}https://${DISPLAY_HOST}/${RAND_SLUG}-s${RESET}"
    echo -e " • ${BOLD}Lệnh Máy Đích (RX):${RESET}"
    echo -e "   Linux/macOS:        ${YELLOW}curl -sL https://${DISPLAY_HOST}/${RAND_SLUG}-r | sh${RESET}"
    echo -e "   Windows PowerShell: ${YELLOW}irm https://${DISPLAY_HOST}/${RAND_SLUG}-r | iex${RESET}"
    echo -e " • ${BOLD}Tải File (RX):${RESET}      ${YELLOW}curl -sLJO https://${DISPLAY_HOST}/${RAND_SLUG}-r${RESET}"
    echo -e "----------------------------------------------------------------"
    echo -e " ${BOLD}[Nhấn Ctrl + C để dừng và ngắt kết nối domain cá nhân]${RESET}"
    echo -e "${GREEN}================================================================${RESET}"
    echo ""

    wait "$PID_TUNNEL" 2>/dev/null || true
}

# --- MAIN ---
detect_platform
load_env
find_wiredrop_bin
ensure_cloudflared

# Trực tiếp nhận cờ tham số nếu truyền vào
if [ "$1" = "1" ] || [ "$1" = "--quick" ]; then
    run_quick_tunnel
    exit 0
elif [ "$1" = "2" ] || [ "$1" = "--custom" ]; then
    run_custom_tunnel
    exit 0
fi

# Định dạng nhãn menu theo đúng yêu cầu
if [ -n "$CLOUDFLARE_TUNNEL_TOKEN" ]; then
    if [ -n "$CUSTOM_DOMAIN" ]; then
        LABEL_2="Domain cá nhân (${GREEN}${CUSTOM_DOMAIN}${RESET})"
    else
        LABEL_2="Domain cá nhân (${GREEN}Đã cấu hình Token${RESET})"
    fi
else
    LABEL_2="Domain cá nhân (${YELLOW}Chưa cấu hình${RESET})"
fi

echo ""
echo -e "${BOLD}================================================================${RESET}"
echo -e "${BOLD}          ⚡ WIREDROP FASTHOSTING CLI - CLOUDFLARE TUNNEL       ${RESET}"
echo -e "${BOLD}================================================================${RESET}"
echo -e " [1] ${BOLD}Domain ngẫu nhiên${RESET} (Mặc định)"
echo -e " [2] ${BOLD}${LABEL_2}${RESET}"
echo -e " [0] Thoát"
echo -e "----------------------------------------------------------------"
read -rp "Lựa chọn của bạn [1/2/0] (Mặc định: 1): " USER_CHOICE
USER_CHOICE="$(echo "$USER_CHOICE" | tr -d '\r' | tr -d '[:space:]')"
USER_CHOICE="${USER_CHOICE:-1}"

case "$USER_CHOICE" in
    1)
        run_quick_tunnel
        ;;
    2)
        run_custom_tunnel
        ;;
    0)
        echo "Tạm biệt!"
        exit 0
        ;;
    *)
        echo -e "${YELLOW}[!] Lựa chọn không hợp lệ, chuyển về mặc định [1].${RESET}"
        run_quick_tunnel
        ;;
esac
