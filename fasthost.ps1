# ==============================================================================
# WireDrop FastHosting CLI (Windows PowerShell)
# Tự động tải Cloudflare Portable, tự cấu hình .env và điều phối đường hầm
# ==============================================================================

try {
    [Console]::OutputEncoding = [System.Text.Encoding]::UTF8
} catch {}
$OutputEncoding = [System.Text.Encoding]::UTF8

# Xác định thư mục script tuyệt đối chuẩn xác
$ScriptDir = $PSScriptRoot
if (-not $ScriptDir) {
    if ($MyInvocation.MyCommand.Path) {
        $ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
    } elseif ($MyInvocation.MyCommand.Definition) {
        $ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Definition
    } else {
        $ScriptDir = (Get-Location).Path
    }
}

$BinDir = Join-Path $ScriptDir "bin"
$EnvFile = Join-Path $ScriptDir ".env"
$WiredropBin = Join-Path $ScriptDir "wiredrop.exe"
$CfBin = Join-Path $BinDir "cloudflared.exe"

if (-not (Test-Path $BinDir)) {
    New-Item -ItemType Directory -Path $BinDir -Force | Out-Null
}

# 1. Đọc tệp .env
function Load-Env {
    $global:Port = "8080"
    $global:CloudflareToken = ""
    $global:CustomDomain = ""

    if (Test-Path $EnvFile) {
        Get-Content $EnvFile | ForEach-Object {
            $line = $_.Trim()
            if ($line -and -not $line.StartsWith("#") -and $line.Contains("=")) {
                $idx = $line.IndexOf("=")
                $key = $line.Substring(0, $idx).Trim()
                $val = $line.Substring($idx + 1).Trim().Trim('"').Trim("'")
                switch ($key) {
                    "PORT" { $global:Port = $val }
                    "CLOUDFLARE_TUNNEL_TOKEN" { $global:CloudflareToken = $val }
                    "CUSTOM_DOMAIN" { $global:CustomDomain = $val }
                }
            }
        }
    }
    if (-not $global:Port) { $global:Port = "8080" }
}

# 2. Quét tìm file thực thi WireDrop đã có
function Find-WiredropBin {
    $candidates = @(
        (Join-Path $ScriptDir "wiredrop.exe"),
        (Join-Path $ScriptDir "wiredrop"),
        (Join-Path $BinDir "wiredrop.exe"),
        (Join-Path $BinDir "wiredrop")
    )

    foreach ($cand in $candidates) {
        if (Test-Path $cand) {
            $global:WiredropBin = $cand
            return
        }
    }

    $sysWire = Get-Command "wiredrop" -ErrorAction SilentlyContinue
    if ($sysWire) {
        $global:WiredropBin = $sysWire.Source
        return
    }

    Write-Host "[✕] Không tìm thấy file thực thi WireDrop (wiredrop.exe)!" -ForegroundColor Red
    Write-Host ">> Vui lòng đặt file thực thi wiredrop.exe vào thư mục: $ScriptDir" -ForegroundColor Yellow
    throw "Thiếu file thực thi wiredrop.exe"
}

# 3. Tự động tải Cloudflare Portable nếu chưa có
function Ensure-Cloudflared {
    if (Test-Path $CfBin) { return }

    $sysCf = Get-Command "cloudflared" -ErrorAction SilentlyContinue
    if ($sysCf) {
        $global:CfBin = $sysCf.Source
        return
    }

    Write-Host "[*] Đang tự động tải Cloudflare Portable (windows-amd64)..." -ForegroundColor Cyan
    $url = "https://github.com/cloudflare/cloudflared/releases/latest/download/cloudflared-windows-amd64.exe"

    try {
        $curlCmd = Get-Command "curl.exe" -ErrorAction SilentlyContinue
        if ($curlCmd) {
            & curl.exe -fsSL -o $CfBin $url
        } else {
            [Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12
            Invoke-WebRequest -Uri $url -OutFile $CfBin -UseBasicParsing
        }
        Write-Host "[✓] Đã tải Cloudflare Portable về: $CfBin" -ForegroundColor Green
    } catch {
        Write-Host "[✕] Tải Cloudflare Portable thất bại: $_" -ForegroundColor Red
        throw "Không thể tải cloudflared.exe"
    }
}

# 4. Tự động thiết lập .env cho Domain cá nhân
function Setup-CustomDomainEnv {
    Write-Host ""
    Write-Host "================================================================" -ForegroundColor White
    Write-Host "        THIẾT LẬP DOMAIN CÁ NHÂN VỚI CLOUDFLARE TUNNEL          " -ForegroundColor White
    Write-Host "================================================================" -ForegroundColor White
    Write-Host "Để dùng Domain riêng cố định, bạn cần Token từ Cloudflare Dashboard:"
    Write-Host " 1. Mở: " -NoNewline; Write-Host "https://dash.cloudflare.com/" -ForegroundColor Cyan -NoNewline; Write-Host " > Zero Trust > Networks > Tunnels"
    Write-Host " 2. Bấm " -NoNewline; Write-Host "Add a tunnel" -ForegroundColor Yellow -NoNewline; Write-Host " > Chọn Cloudflared > Copy chuỗi TOKEN."
    Write-Host " 3. Tại tab Public Hostname, trỏ subdomain về: " -NoNewline; Write-Host "http://localhost:$Port" -ForegroundColor Green
    Write-Host "----------------------------------------------------------------"
    Write-Host ""

    $token = (Read-Host ">> Dán Cloudflare Tunnel Token của bạn (eyJh...)").Trim()
    if (-not $token) {
        Write-Host "[!] Token không được để trống. Hủy cấu hình." -ForegroundColor Red
        return $false
    }

    $domain = (Read-Host ">> Nhập tên domain cá nhân của bạn (ví dụ: w.yourdomain.com)").Trim()

    $lines = @(
        "# Cấu hình WireDrop FastHosting (Tự động tạo bởi fasthost CLI)",
        "PORT=$Port",
        "CLOUDFLARE_TUNNEL_TOKEN=$token",
        "CUSTOM_DOMAIN=$domain"
    )
    $lines | Set-Content -Path $EnvFile -Encoding UTF8
    Write-Host "[✓] Đã lưu cấu hình vào .env thành công!" -ForegroundColor Green
    Load-Env
    return $true
}

# 5. Khởi chạy Quick Tunnel (Domain ngẫu nhiên)
function Run-QuickTunnel {
    Stop-Process -Name "wiredrop" -Force -ErrorAction SilentlyContinue
    Stop-Process -Name "cloudflared" -Force -ErrorAction SilentlyContinue
    Start-Sleep -Milliseconds 300

    Write-Host "[*] Khởi động WireDrop trên cổng $Port..." -ForegroundColor Cyan
    $wireLog = Join-Path $ScriptDir ".wire_server.log"
    if (Test-Path $wireLog) { Remove-Item $wireLog -Force -ErrorAction SilentlyContinue }

    $wireProc = Start-Process -FilePath $WiredropBin -ArgumentList "-port $Port" -PassThru -WindowStyle Hidden -RedirectStandardError $wireLog -RedirectStandardOutput $wireLog

    Start-Sleep -Milliseconds 600

    if ($wireProc.HasExited) {
        Write-Host "[✕] Không thể khởi động WireDrop (cổng $Port bị chiếm)! Log chi tiết:" -ForegroundColor Red
        if (Test-Path $wireLog) { Get-Content $wireLog }
        throw "Lỗi khởi động WireDrop"
    }

    Write-Host "[*] Đang mở đường hầm Cloudflare Quick Tunnel (protocol: http2)..." -ForegroundColor Cyan
    $logFile = Join-Path $ScriptDir ".cf_quick.log"
    $logOutFile = Join-Path $ScriptDir ".cf_quick_out.log"
    if (Test-Path $logFile) { Remove-Item $logFile -Force -ErrorAction SilentlyContinue }
    if (Test-Path $logOutFile) { Remove-Item $logOutFile -Force -ErrorAction SilentlyContinue }

    # Dùng --protocol http2 để tránh bị nhà mạng chặn UDP/QUIC tại Việt Nam
    $cfProc = Start-Process -FilePath $CfBin -ArgumentList "tunnel --protocol http2 --url http://localhost:$Port" -PassThru -WindowStyle Hidden -RedirectStandardError $logFile -RedirectStandardOutput $logOutFile

    $tunnelUrl = ""
    Write-Host "[*] Đang kết nối tới Cloudflare Edge" -NoNewline
    for ($i = 0; $i -lt 50; $i++) {
        Start-Sleep -Milliseconds 500
        Write-Host "." -NoNewline

        if (Test-Path $logFile) {
            $content = Get-Content $logFile -Raw -ErrorAction SilentlyContinue
            if ($content -match '(https://[-a-zA-Z0-9.]*\.trycloudflare\.com)') {
                $tunnelUrl = $matches[1]
                break
            }
        }
        if (Test-Path $logOutFile) {
            $contentOut = Get-Content $logOutFile -Raw -ErrorAction SilentlyContinue
            if ($contentOut -match '(https://[-a-zA-Z0-9.]*\.trycloudflare\.com)') {
                $tunnelUrl = $matches[1]
                break
            }
        }
    }
    Write-Host ""

    if (-not $tunnelUrl) {
        Write-Host "[✕] Không bắt được URL Cloudflare. Log chi tiết:" -ForegroundColor Red
        if (Test-Path $logFile) { Get-Content $logFile }
        if (Test-Path $logOutFile) { Get-Content $logOutFile }
        Stop-Process -Id $wireProc.Id, $cfProc.Id -Force -ErrorAction SilentlyContinue
        throw "Không bắt được URL Cloudflare"
    }

    $chars = "abcdefghijklmnopqrstuvwxyz0123456789"
    $randSlug = -join ((1..4) | ForEach-Object { $chars[(Get-Random -Maximum $chars.Length)] })

    Write-Host ""
    Write-Host "================================================================" -ForegroundColor Green
    Write-Host "       ⚡ WIREDROP FASTHOST ONLINE (DOMAIN NGẪU NHIÊN)           " -ForegroundColor White
    Write-Host "================================================================" -ForegroundColor Green
    Write-Host " • Web Soạn Thảo (TX):  " -NoNewline; Write-Host "$tunnelUrl/$randSlug-s" -ForegroundColor Cyan
    Write-Host " • Lệnh Máy Đích (RX):"
    Write-Host "   Linux/macOS:        " -NoNewline; Write-Host "curl -sL $tunnelUrl/$randSlug-r | sh" -ForegroundColor Yellow
    Write-Host "   Windows PowerShell: " -NoNewline; Write-Host "irm $tunnelUrl/$randSlug-r | iex" -ForegroundColor Yellow
    Write-Host " • Tải File (RX):      " -NoNewline; Write-Host "curl -sLJO $tunnelUrl/$randSlug-r" -ForegroundColor Yellow
    Write-Host "----------------------------------------------------------------"
    Write-Host " [Nhấn Ctrl + C để dừng và đóng đường hầm bất cứ lúc nào]" -ForegroundColor White
    Write-Host "================================================================" -ForegroundColor Green
    Write-Host ""

    try {
        while (-not $cfProc.HasExited) {
            Start-Sleep -Seconds 1
        }
    } finally {
        Write-Host "`n[*] Đang đóng đường hầm và dừng máy chủ WireDrop..." -ForegroundColor Yellow
        Stop-Process -Id $wireProc.Id, $cfProc.Id -Force -ErrorAction SilentlyContinue
        if (Test-Path $logFile) { Remove-Item $logFile -Force -ErrorAction SilentlyContinue }
        if (Test-Path $logOutFile) { Remove-Item $logOutFile -Force -ErrorAction SilentlyContinue }
        if (Test-Path $wireLog) { Remove-Item $wireLog -Force -ErrorAction SilentlyContinue }
        Write-Host "[✓] Đã giải phóng cổng $Port. Thoát an toàn." -ForegroundColor Green
    }
}

# 6. Khởi chạy Custom Domain (Named Tunnel)
function Run-CustomTunnel {
    if (-not $CloudflareToken) {
        if (-not (Setup-CustomDomainEnv)) { return }
    }

    Stop-Process -Name "wiredrop" -Force -ErrorAction SilentlyContinue
    Stop-Process -Name "cloudflared" -Force -ErrorAction SilentlyContinue
    Start-Sleep -Milliseconds 300

    Write-Host "[*] Khởi động WireDrop trên cổng $Port..." -ForegroundColor Cyan
    $wireLog = Join-Path $ScriptDir ".wire_server.log"
    if (Test-Path $wireLog) { Remove-Item $wireLog -Force -ErrorAction SilentlyContinue }

    $wireProc = Start-Process -FilePath $WiredropBin -ArgumentList "-port $Port" -PassThru -WindowStyle Hidden -RedirectStandardError $wireLog -RedirectStandardOutput $wireLog

    Start-Sleep -Milliseconds 600

    if ($wireProc.HasExited) {
        Write-Host "[✕] Không thể khởi động WireDrop (cổng $Port bị chiếm)! Log chi tiết:" -ForegroundColor Red
        if (Test-Path $wireLog) { Get-Content $wireLog }
        throw "Lỗi khởi động WireDrop"
    }

    Write-Host "[*] Đang kết nối Cloudflare Named Tunnel theo Token (protocol: http2)..." -ForegroundColor Cyan
    $cfProc = Start-Process -FilePath $CfBin -ArgumentList "tunnel --protocol http2 run --token $CloudflareToken" -PassThru -WindowStyle Hidden

    $chars = "abcdefghijklmnopqrstuvwxyz0123456789"
    $randSlug = -join ((1..4) | ForEach-Object { $chars[(Get-Random -Maximum $chars.Length)] })
    $displayHost = if ($CustomDomain) { $CustomDomain } else { "<your-domain>" }

    Write-Host ""
    Write-Host "================================================================" -ForegroundColor Green
    Write-Host "       ⚡ WIREDROP FASTHOST ONLINE (DOMAIN CÁ NHÂN CỐ ĐỊNH)       " -ForegroundColor White
    Write-Host "================================================================" -ForegroundColor Green
    Write-Host " • Web Soạn Thảo (TX):  " -NoNewline; Write-Host "https://$displayHost/$randSlug-s" -ForegroundColor Cyan
    Write-Host " • Lệnh Máy Đích (RX):"
    Write-Host "   Linux/macOS:        " -NoNewline; Write-Host "curl -sL https://$displayHost/$randSlug-r | sh" -ForegroundColor Yellow
    Write-Host "   Windows PowerShell: " -NoNewline; Write-Host "irm https://$displayHost/$randSlug-r | iex" -ForegroundColor Yellow
    Write-Host " • Tải File (RX):      " -NoNewline; Write-Host "curl -sLJO https://$displayHost/$randSlug-r" -ForegroundColor Yellow
    Write-Host "----------------------------------------------------------------"
    Write-Host " [Nhấn Ctrl + C để dừng và ngắt kết nối domain cá nhân]" -ForegroundColor White
    Write-Host "================================================================" -ForegroundColor Green
    Write-Host ""

    try {
        while (-not $cfProc.HasExited) {
            Start-Sleep -Seconds 1
        }
    } finally {
        Write-Host "`n[*] Đang đóng đường hầm và dừng máy chủ WireDrop..." -ForegroundColor Yellow
        Stop-Process -Id $wireProc.Id, $cfProc.Id -Force -ErrorAction SilentlyContinue
        if (Test-Path $wireLog) { Remove-Item $wireLog -Force -ErrorAction SilentlyContinue }
        Write-Host "[✓] Đã giải phóng cổng $Port. Thoát an toàn." -ForegroundColor Green
    }
}

# --- MAIN CONTROLLER WITH TRY/CATCH ---
try {
    Load-Env
    Find-WiredropBin
    Ensure-Cloudflared

    if ($args[0] -eq "1" -or $args[0] -eq "--quick") {
        Run-QuickTunnel
        return
    }
    if ($args[0] -eq "2" -or $args[0] -eq "--custom") {
        Run-CustomTunnel
        return
    }

    $label2 = if ($CloudflareToken) {
        if ($CustomDomain) { "Domain cá nhân ($CustomDomain)" } else { "Domain cá nhân (Đã cấu hình Token)" }
    } else {
        "Domain cá nhân (Chưa cấu hình)"
    }

    Write-Host ""
    Write-Host "================================================================" -ForegroundColor White
    Write-Host "          ⚡ WIREDROP FASTHOSTING CLI - CLOUDFLARE TUNNEL       " -ForegroundColor White
    Write-Host "================================================================" -ForegroundColor White
    Write-Host " [1] Domain ngẫu nhiên (Mặc định)"
    if ($CloudflareToken) {
        Write-Host " [2] $label2" -ForegroundColor Green
    } else {
        Write-Host " [2] $label2" -ForegroundColor Yellow
    }
    Write-Host " [0] Thoát"
    Write-Host "----------------------------------------------------------------"
    $choice = Read-Host "Lựa chọn của bạn [1/2/0] (Mặc định: 1)"
    if (-not $choice) { $choice = "1" }

    switch ($choice) {
        "1" { Run-QuickTunnel }
        "2" { Run-CustomTunnel }
        "0" { Write-Host "Tạm biệt!"; return }
        default {
            Write-Host "[!] Lựa chọn không hợp lệ, chuyển về mặc định [1]." -ForegroundColor Yellow
            Run-QuickTunnel
        }
    }
} catch {
    Write-Host ""
    Write-Host "================================================================" -ForegroundColor Red
    Write-Host "[✕] ĐÃ XẢY RA LỖI TRONG QUÁ TRÌNH THỰC THI:" -ForegroundColor Red
    Write-Host $_.Exception.ToString() -ForegroundColor Red
    Write-Host "================================================================" -ForegroundColor Red
}
