# ==============================================================================
# WireDrop FastHosting CLI (Windows PowerShell)
# ==============================================================================

# Xac dinh thu muc script tuyet doi
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

# 1. Doc tep .env
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

# 2. Quet tim file thuc thi WireDrop da co
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

    Write-Host "[X] Khong tim thay file thuc thi WireDrop (wiredrop.exe)!" -ForegroundColor Red
    Write-Host ">> Vui long dat file thuc thi wiredrop.exe vao thu muc: $ScriptDir" -ForegroundColor Yellow
    throw "Thieu file thuc thi wiredrop.exe"
}

# 3. Tu dong tai Cloudflare Portable neu chua co
function Ensure-Cloudflared {
    if (Test-Path $CfBin) { return }

    $sysCf = Get-Command "cloudflared" -ErrorAction SilentlyContinue
    if ($sysCf) {
        $global:CfBin = $sysCf.Source
        return
    }

    Write-Host "[*] Dang tu dong tai Cloudflare Portable (windows-amd64)..." -ForegroundColor Cyan
    $url = "https://github.com/cloudflare/cloudflared/releases/latest/download/cloudflared-windows-amd64.exe"

    try {
        $curlCmd = Get-Command "curl.exe" -ErrorAction SilentlyContinue
        if ($curlCmd) {
            & curl.exe -fsSL -o $CfBin $url
        } else {
            [Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12
            Invoke-WebRequest -Uri $url -OutFile $CfBin -UseBasicParsing
        }
        Write-Host "[OK] Da tai Cloudflare Portable ve: $CfBin" -ForegroundColor Green
    } catch {
        Write-Host "[X] Tai Cloudflare Portable that bai: $_" -ForegroundColor Red
        throw "Khong the tai cloudflared.exe"
    }
}

# 4. Thiet lap .env cho Domain ca nhan
function Setup-CustomDomainEnv {
    Write-Host ""
    Write-Host "================================================================" -ForegroundColor White
    Write-Host "        THIET LAP DOMAIN CA NHAN VOI CLOUDFLARE TUNNEL          " -ForegroundColor White
    Write-Host "================================================================" -ForegroundColor White
    Write-Host "De dung Domain rieng co dinh, ban can Token tu Cloudflare Dashboard:"
    Write-Host " 1. Mo: " -NoNewline; Write-Host "https://dash.cloudflare.com/" -ForegroundColor Cyan -NoNewline; Write-Host " > Zero Trust > Networks > Tunnels"
    Write-Host " 2. Bam " -NoNewline; Write-Host "Add a tunnel" -ForegroundColor Yellow -NoNewline; Write-Host " > Chon Cloudflared > Copy chuoi TOKEN."
    Write-Host " 3. Tai tab Public Hostname, tro subdomain ve: " -NoNewline; Write-Host "http://localhost:$Port" -ForegroundColor Green
    Write-Host "----------------------------------------------------------------"
    Write-Host ""

    $token = (Read-Host ">> Dan Cloudflare Tunnel Token cua ban (eyJh...)").Trim()
    if (-not $token) {
        Write-Host "[!] Token khong duoc de trong. Huy cau hinh." -ForegroundColor Red
        return $false
    }

    $domain = (Read-Host ">> Nhap ten domain ca nhan cua ban (vi du: w.yourdomain.com)").Trim()

    $lines = @(
        "# Cau hinh WireDrop FastHosting (Tu dong tao boi fasthost CLI)",
        "PORT=$Port",
        "CLOUDFLARE_TUNNEL_TOKEN=$token",
        "CUSTOM_DOMAIN=$domain"
    )
    $lines | Set-Content -Path $EnvFile -Encoding UTF8
    Write-Host "[OK] Da luu cau hinh vao .env thanh cong!" -ForegroundColor Green
    Load-Env
    return $true
}

# 5. Khoi chay Quick Tunnel (Domain ngau nhien)
function Run-QuickTunnel {
    Stop-Process -Name "wiredrop" -Force -ErrorAction SilentlyContinue
    Stop-Process -Name "cloudflared" -Force -ErrorAction SilentlyContinue
    Start-Sleep -Milliseconds 300

    Write-Host "[*] Khoi dong WireDrop tren cong $Port..." -ForegroundColor Cyan
    $wireErrLog = Join-Path $ScriptDir ".wire_err.log"
    $wireOutLog = Join-Path $ScriptDir ".wire_out.log"
    if (Test-Path $wireErrLog) { Remove-Item $wireErrLog -Force -ErrorAction SilentlyContinue }
    if (Test-Path $wireOutLog) { Remove-Item $wireOutLog -Force -ErrorAction SilentlyContinue }

    $wireProc = Start-Process -FilePath $WiredropBin -ArgumentList "-port $Port" -PassThru -WindowStyle Hidden -RedirectStandardError $wireErrLog -RedirectStandardOutput $wireOutLog

    Start-Sleep -Milliseconds 600

    if ($wireProc.HasExited) {
        Write-Host "[X] Khong the khoi dong WireDrop (cong $Port bi chiem)! Log:" -ForegroundColor Red
        if (Test-Path $wireErrLog) { Get-Content $wireErrLog }
        throw "Loi khoi dong WireDrop"
    }

    Write-Host "[*] Dang mo duong ham Cloudflare Quick Tunnel (protocol: http2)..." -ForegroundColor Cyan
    $logFile = Join-Path $ScriptDir ".cf_quick.log"
    $logOutFile = Join-Path $ScriptDir ".cf_quick_out.log"
    if (Test-Path $logFile) { Remove-Item $logFile -Force -ErrorAction SilentlyContinue }
    if (Test-Path $logOutFile) { Remove-Item $logOutFile -Force -ErrorAction SilentlyContinue }

    $cfProc = Start-Process -FilePath $CfBin -ArgumentList "tunnel --protocol http2 --url http://localhost:$Port" -PassThru -WindowStyle Hidden -RedirectStandardError $logFile -RedirectStandardOutput $logOutFile

    $tunnelUrl = ""
    Write-Host "[*] Dang ket noi toi Cloudflare Edge" -NoNewline
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
        Write-Host "[X] Khong bat duoc URL Cloudflare. Log chi tiet:" -ForegroundColor Red
        if (Test-Path $logFile) { Get-Content $logFile }
        if (Test-Path $logOutFile) { Get-Content $logOutFile }
        Stop-Process -Id $wireProc.Id, $cfProc.Id -Force -ErrorAction SilentlyContinue
        throw "Khong bat duoc URL Cloudflare"
    }

    $chars = "abcdefghijklmnopqrstuvwxyz0123456789"
    $randSlug = -join ((1..4) | ForEach-Object { $chars[(Get-Random -Maximum $chars.Length)] })

    Write-Host ""
    Write-Host "================================================================" -ForegroundColor Green
    Write-Host "       WIREDROP FASTHOST ONLINE (DOMAIN NGAU NHIEN)             " -ForegroundColor White
    Write-Host "================================================================" -ForegroundColor Green
    Write-Host " • Web Hub (Trang chu #):   " -NoNewline; Write-Host "$tunnelUrl/#" -ForegroundColor Cyan
    Write-Host "   (Mo link tren de xem Cheatsheet hoac tu tao phong tuy y)" -ForegroundColor Gray
    Write-Host ""
    Write-Host " • Vi du truyen nhanh (Phong mau ngau nhien: $randSlug):" -ForegroundColor White
    Write-Host "   - Web phat (TX):         " -NoNewline; Write-Host "$tunnelUrl/$randSlug-s" -ForegroundColor Cyan
    Write-Host "   - Lenh nhan (RX):"
    Write-Host "     Linux/macOS:           " -NoNewline; Write-Host "curl -sL $tunnelUrl/$randSlug-r | sh" -ForegroundColor Yellow
    Write-Host "     Windows PowerShell:    " -NoNewline; Write-Host "irm $tunnelUrl/$randSlug-r | iex" -ForegroundColor Yellow
    Write-Host "     Tai File:              " -NoNewline; Write-Host "curl -sLJO $tunnelUrl/$randSlug-r" -ForegroundColor Yellow
    Write-Host "----------------------------------------------------------------"
    Write-Host " [Nhan Ctrl + C de dung va dong duong ham bat cu luc nao]" -ForegroundColor White
    Write-Host "================================================================" -ForegroundColor Green
    Write-Host ""

    try {
        while (-not $cfProc.HasExited) {
            Start-Sleep -Seconds 1
        }
    } finally {
        Write-Host "`n[*] Dang dong duong ham va dung may chu WireDrop..." -ForegroundColor Yellow
        Stop-Process -Id $wireProc.Id, $cfProc.Id -Force -ErrorAction SilentlyContinue
        if (Test-Path $logFile) { Remove-Item $logFile -Force -ErrorAction SilentlyContinue }
        if (Test-Path $logOutFile) { Remove-Item $logOutFile -Force -ErrorAction SilentlyContinue }
        if (Test-Path $wireErrLog) { Remove-Item $wireErrLog -Force -ErrorAction SilentlyContinue }
        if (Test-Path $wireOutLog) { Remove-Item $wireOutLog -Force -ErrorAction SilentlyContinue }
        Write-Host "[OK] Da giai phong cong $Port. Thoat an toan." -ForegroundColor Green
    }
}

# 6. Khoi chay Custom Domain (Named Tunnel)
function Run-CustomTunnel {
    if (-not $CloudflareToken) {
        if (-not (Setup-CustomDomainEnv)) { return }
    }

    Stop-Process -Name "wiredrop" -Force -ErrorAction SilentlyContinue
    Stop-Process -Name "cloudflared" -Force -ErrorAction SilentlyContinue
    Start-Sleep -Milliseconds 300

    Write-Host "[*] Khoi dong WireDrop tren cong $Port..." -ForegroundColor Cyan
    $wireErrLog = Join-Path $ScriptDir ".wire_err.log"
    $wireOutLog = Join-Path $ScriptDir ".wire_out.log"
    if (Test-Path $wireErrLog) { Remove-Item $wireErrLog -Force -ErrorAction SilentlyContinue }
    if (Test-Path $wireOutLog) { Remove-Item $wireOutLog -Force -ErrorAction SilentlyContinue }

    $wireProc = Start-Process -FilePath $WiredropBin -ArgumentList "-port $Port" -PassThru -WindowStyle Hidden -RedirectStandardError $wireErrLog -RedirectStandardOutput $wireOutLog

    Start-Sleep -Milliseconds 600

    if ($wireProc.HasExited) {
        Write-Host "[X] Khong the khoi dong WireDrop (cong $Port bi chiem)! Log:" -ForegroundColor Red
        if (Test-Path $wireErrLog) { Get-Content $wireErrLog }
        throw "Loi khoi dong WireDrop"
    }

    Write-Host "[*] Dang ket noi Cloudflare Named Tunnel theo Token (protocol: http2)..." -ForegroundColor Cyan
    $cfProc = Start-Process -FilePath $CfBin -ArgumentList "tunnel --protocol http2 run --token $CloudflareToken" -PassThru -WindowStyle Hidden

    $chars = "abcdefghijklmnopqrstuvwxyz0123456789"
    $randSlug = -join ((1..4) | ForEach-Object { $chars[(Get-Random -Maximum $chars.Length)] })
    $displayHost = if ($CustomDomain) { $CustomDomain } else { "<your-domain>" }

    Write-Host ""
    Write-Host "================================================================" -ForegroundColor Green
    Write-Host "       WIREDROP FASTHOST ONLINE (DOMAIN CA NHAN CO DINH)        " -ForegroundColor White
    Write-Host "================================================================" -ForegroundColor Green
    Write-Host " • Web Hub (Trang chu #):   " -NoNewline; Write-Host "https://$displayHost/#" -ForegroundColor Cyan
    Write-Host "   (Mo link tren de xem Cheatsheet hoac tu tao phong tuy y)" -ForegroundColor Gray
    Write-Host ""
    Write-Host " • Vi du truyen nhanh (Phong mau ngau nhien: $randSlug):" -ForegroundColor White
    Write-Host "   - Web phat (TX):         " -NoNewline; Write-Host "https://$displayHost/$randSlug-s" -ForegroundColor Cyan
    Write-Host "   - Lenh nhan (RX):"
    Write-Host "     Linux/macOS:           " -NoNewline; Write-Host "curl -sL https://$displayHost/$randSlug-r | sh" -ForegroundColor Yellow
    Write-Host "     Windows PowerShell:    " -NoNewline; Write-Host "irm https://$displayHost/$randSlug-r | iex" -ForegroundColor Yellow
    Write-Host "     Tai File:              " -NoNewline; Write-Host "curl -sLJO https://$displayHost/$randSlug-r" -ForegroundColor Yellow
    Write-Host "----------------------------------------------------------------"
    Write-Host " [Nhan Ctrl + C de dung va ngat ket noi domain ca nhan]" -ForegroundColor White
    Write-Host "================================================================" -ForegroundColor Green
    Write-Host ""

    try {
        while (-not $cfProc.HasExited) {
            Start-Sleep -Seconds 1
        }
    } finally {
        Write-Host "`n[*] Dang dong duong ham va dung may chu WireDrop..." -ForegroundColor Yellow
        Stop-Process -Id $wireProc.Id, $cfProc.Id -Force -ErrorAction SilentlyContinue
        if (Test-Path $wireErrLog) { Remove-Item $wireErrLog -Force -ErrorAction SilentlyContinue }
        if (Test-Path $wireOutLog) { Remove-Item $wireOutLog -Force -ErrorAction SilentlyContinue }
        Write-Host "[OK] Da giai phong cong $Port. Thoat an toan." -ForegroundColor Green
    }
}

# --- MAIN CONTROLLER WITH TRY/CATCH ---
try {
    Load-Env
    Find-WiredropBin
    Ensure-Cloudflared

    if ($args.Count -gt 0) {
        if ($args[0] -eq "1" -or $args[0] -eq "--quick") {
            Run-QuickTunnel
            return
        }
        if ($args[0] -eq "2" -or $args[0] -eq "--custom") {
            Run-CustomTunnel
            return
        }
    }

    $label2 = if ($CloudflareToken) {
        if ($CustomDomain) { "Domain ca nhan ($CustomDomain)" } else { "Domain ca nhan (Da cau hinh Token)" }
    } else {
        "Domain ca nhan (Chua cau hinh)"
    }

    Write-Host ""
    Write-Host "================================================================" -ForegroundColor White
    Write-Host "          WIREDROP FASTHOSTING CLI - CLOUDFLARE TUNNEL          " -ForegroundColor White
    Write-Host "================================================================" -ForegroundColor White
    Write-Host " [1] Domain ngau nhien (Mac dinh)"
    if ($CloudflareToken) {
        Write-Host " [2] $label2" -ForegroundColor Green
    } else {
        Write-Host " [2] $label2" -ForegroundColor Yellow
    }
    Write-Host " [0] Thoat"
    Write-Host "----------------------------------------------------------------"
    $choice = Read-Host "Lua chon cua ban [1/2/0] (Mac dinh: 1)"
    if (-not $choice) { $choice = "1" }

    switch ($choice) {
        "1" { Run-QuickTunnel }
        "2" { Run-CustomTunnel }
        "0" { Write-Host "Tam biet!"; return }
        default {
            Write-Host "[!] Lua chon khong hop le, chuyen ve mac dinh [1]." -ForegroundColor Yellow
            Run-QuickTunnel
        }
    }
} catch {
    Write-Host ""
    Write-Host "================================================================" -ForegroundColor Red
    Write-Host "[X] DA XAY RA LOI TRONG QUA TRINH THUC THI:" -ForegroundColor Red
    Write-Host $_.Exception.ToString() -ForegroundColor Red
    Write-Host "================================================================" -ForegroundColor Red
}
