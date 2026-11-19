// WireDrop Minimalist Unified Form Client Logic
(function () {
    const $ = id => document.getElementById(id);
    function generateSlug(len = 4) {
        const chars = "abcdefghijklmnopqrstuvwxyz0123456789";
        let res = "";
        for (let i = 0; i < len; i++) {
            res += chars.charAt(Math.floor(Math.random() * chars.length));
        }
        return res;
    }
    const defaultSlug = generateSlug(4);
    let slug = defaultSlug, isSender = true, isHome = false, os = "linux", sse = null, saveTimer = null, typing = false, pollTimer = null, currentUploadXhr = null, chunkUploadState = null;

    // Quản lý mã kết nối vật lý (Physical Connection ID) lưu theo phiên phòng/vai trò để F5 không bị khóa 409
    function getConnId() {
        const key = `wd_conn_${slug}_${isSender ? "s" : "r"}`;
        let cid = sessionStorage.getItem(key);
        if (!cid) {
            cid = "c_" + Math.random().toString(36).slice(2, 9) + Date.now().toString(36);
            sessionStorage.setItem(key, cid);
        }
        return cid;
    }

    // Nhận diện thiết bị di động (Mobile User-Agent hoặc màn hình cảm ứng hẹp)
    function isMobileDevice() {
        return /Android|webOS|iPhone|iPad|iPod|BlackBerry|IEMobile|Opera Mini/i.test(navigator.userAgent) ||
               (navigator.maxTouchPoints > 1 && window.innerWidth <= 850);
    }

    // Quản lý Screen Wake Lock API (Giữ màn hình sáng không bị tự động sleep khi đang truyền tải)
    let screenWakeLock = null;
    async function acquireWakeLock() {
        if ("wakeLock" in navigator) {
            try {
                screenWakeLock = await navigator.wakeLock.request("screen");
                screenWakeLock.addEventListener("release", () => { screenWakeLock = null; });
            } catch (_) {}
        }
    }
    function releaseWakeLock() {
        if (screenWakeLock) {
            screenWakeLock.release().catch(() => {});
            screenWakeLock = null;
        }
    }
    document.addEventListener("visibilitychange", async () => {
        if (document.visibilityState === "visible" && isDownloadingRx) {
            await acquireWakeLock();
        }
    });

    function showToast(msg) {
        const t = $("toast");
        t.textContent = msg;
        t.classList.add("show");
        setTimeout(() => t.classList.remove("show"), 1500);
    }

    function copyText(text, msg = "[ĐÃ COPY ✓]") {
        if (navigator.clipboard) {
            navigator.clipboard.writeText(text).then(() => showToast(msg));
        } else {
            const ta = document.createElement("textarea");
            ta.value = text;
            document.body.appendChild(ta);
            ta.select();
            document.execCommand("copy");
            document.body.removeChild(ta);
            showToast(msg);
        }
    }

    let activeShortURL = null;

    function updateCheatCode() {
        if (isHome) return;
        let urlRx = `${location.origin}/${slug}-r`;
        if (activeShortURL) {
            urlRx = activeShortURL;
        }
        const isFile = $("btnModeFile").classList.contains("active");
        const titleEl = $("rxBarTitle");
        if (titleEl) titleEl.textContent = "LỆNH CHẠY TRÊN MÁY ĐÍCH (RX):";

        if (os === "linux") {
            $("cheatCode").textContent = isFile ? `curl -sLJO ${urlRx}` : `curl -sL ${urlRx} | sh`;
        } else if (os === "alpine") {
            const fname = ($("activeFileName").textContent && $("activeFileName").textContent !== "-") ? $("activeFileName").textContent : "file.bin";
            $("cheatCode").textContent = isFile ? `wget -O ${fname} ${urlRx}` : `wget -qO- ${urlRx} | sh`;
        } else if (os === "win") {
            const fname = ($("activeFileName").textContent && $("activeFileName").textContent !== "-") ? $("activeFileName").textContent : "file.bin";
            $("cheatCode").textContent = isFile ? `iwr ${urlRx} -OutFile ${fname}` : `irm ${urlRx} | iex`;
        } else {
            $("cheatCode").textContent = urlRx;
        }

        const tipEl = $("shortenTooltip");
        if (tipEl) {
            const isLongUrl = location.origin.includes("trycloudflare.com") || location.origin.length > 28;
            if (isSender && isLongUrl && !activeShortURL) {
                tipEl.style.display = "block";
            } else {
                tipEl.style.display = "none";
            }
        }
    }

    function saveTxHistory(txt) {
        txt = (txt || "").trim();
        if (!txt) return;
        try {
            let hist = JSON.parse(localStorage.getItem("wd_tx_history") || "[]");
            hist = hist.filter(item => item !== txt);
            hist.unshift(txt);
            if (hist.length > 3) hist = hist.slice(0, 3);
            localStorage.setItem("wd_tx_history", JSON.stringify(hist));
            renderTxHistory();
        } catch (_) {}
    }

    function renderTxHistory() {
        const container = $("txHistoryContainer");
        const list = $("txHistoryList");
        if (!container || !list) return;
        try {
            const hist = JSON.parse(localStorage.getItem("wd_tx_history") || "[]");
            if (!hist || hist.length === 0) {
                container.style.display = "none";
                return;
            }
            container.style.display = "flex";
            list.innerHTML = "";
            hist.forEach((item, idx) => {
                const chip = document.createElement("button");
                chip.className = "tx-history-chip";
                chip.type = "button";
                const displayTitle = item.replace(/[\r\n]+/g, " ");
                chip.textContent = `#${idx + 1}: ${displayTitle.length > 25 ? displayTitle.slice(0, 25) + '…' : displayTitle}`;
                chip.title = item;
                chip.onclick = () => {
                    $("senderTextarea").value = item;
                    autoSave();
                };
                list.appendChild(chip);
            });
        } catch (_) {}
    }

    function showLockout(msg) {
        $("lockoutTitle").textContent = (isSender ? "TX" : "RX") + " ĐANG BẬN (409)";
        $("lockoutMsg").textContent = msg || "Đầu dây này đã có 1 kết nối khác cắm vào. Bấm [THỬ KẾT NỐI LẠI] để chuyển quyền cắm dây!";
        $("lockoutModal").style.display = "flex";
    }

    function connectSSE(force = false) {
        if (isHome) return;
        if (sse) { sse.close(); sse = null; }
        const role = isSender ? "sender" : "receiver";
        const cid = getConnId();
        const forceParam = force ? "&force=1" : "";
        fetch(`/${slug}/events?role=${role}&conn=${cid}&check=1${forceParam}`).then(res => {
            if (res.status === 409) return res.json().then(d => showLockout(d.message));
            if (res.ok) {
                $("lockoutModal").style.display = "none";
                res.json().then(d => {
                    setPairStatus(!!d.receiver_paired, !!d.sender_paired);
                    renderPayload(d);
                });
                startSSE(role, cid, force);
                startPolling(role, cid);
            }
        }).catch(() => {
            startSSE(role, cid, force);
            startPolling(role, cid);
        });
    }

    let recvCurrentFile = null;
    let recvDownloadController = null;
    let isDownloadingRx = false;
    const isUploadingTx = () => !!(chunkUploadState || currentUploadXhr);

    function startPolling(role, cid) {
        if (pollTimer) clearInterval(pollTimer);
        pollTimer = setInterval(() => {
            if (isHome) return;
            fetch(`/${slug}/events?role=${role}&conn=${cid}&check=1`).then(res => {
                if (res.ok) {
                    res.json().then(d => {
                        setPairStatus(!!d.receiver_paired, !!d.sender_paired);
                        if (isSender && chunkUploadState && chunkUploadState.waitingRx && d.receiver_paired) {
                            chunkUploadState.waitingRx = false;
                            $("uploadEta").textContent = "Đang truyền...";
                            showToast("[RX ĐÃ KẾT NỐI • TIẾP TỤC ĐẨY FILE]");
                            (chunkUploadState.dispatch || chunkUploadState.uploadNext)();
                        }
                        if (!isSender) {
                            if (!isDownloadingRx) renderPayload(d);
                        } else if (!isUploadingTx()) {
                            if (d.slot_type === 2 && $("activeFileName").textContent !== d.slot_file_name) {
                                showActiveFile(d.slot_file_name, d.slot_file_size, d.total_chunks);
                            } else if (d.slot_type === 0 && $("activeFileName").textContent !== "-") {
                                $("activeFileName").textContent = "-";
                                if ($("activeFileBatchRow")) $("activeFileBatchRow").style.display = "none";
                                showFileDropzone();
                                $("syncIndicator").textContent = "[ĐÃ CHUYỂN ✓]";
                            } else if (d.slot_type === 1 && !$("btnModeFile").classList.contains("active") && !typing) {
                                if ($("senderTextarea").value !== (d.slot_text || "")) {
                                    $("senderTextarea").value = d.slot_text || "";
                                }
                            }
                        }
                    });
                } else if (res.status === 409) {
                    res.json().then(d => showLockout(d.message));
                }
            }).catch(() => {});
        }, 3000);
    }

    function startSSE(role, cid, force = false) {
        if (sse) { sse.close(); sse = null; }
        const forceParam = force ? "&force=1" : "";
        sse = new EventSource(`/${slug}/events?role=${role}&conn=${cid}${forceParam}`);
        sse.addEventListener("init", e => {
            const d = JSON.parse(e.data);
            setPairStatus(!!d.receiver_paired, !!d.sender_paired);
            renderPayload(d);
        });
        sse.addEventListener("partner_paired", () => {
            const ns = $("nodeSender");
            setPairStatus(true, ns ? ns.textContent.includes("●") : true);
            if (isSender) {
                showToast("[RX ĐÃ KẾT NỐI]");
                if (chunkUploadState && chunkUploadState.waitingRx) {
                    chunkUploadState.waitingRx = false;
                    $("uploadEta").textContent = "Đang truyền...";
                    (chunkUploadState.dispatch || chunkUploadState.uploadNext)();
                }
            }
        });
        sse.addEventListener("partner_unpaired", () => {
            const ns = $("nodeSender");
            setPairStatus(false, ns ? ns.textContent.includes("●") : true);
            if (isSender) showToast("[RX ĐÃ NGẮT]");
        });
        sse.addEventListener("rx_connected", () => {
            setPairStatus(true, $("nodeSender") ? $("nodeSender").textContent.includes("●") : true);
            if (isSender) {
                showToast("[RX ĐÃ BẮT ĐẦU KÉO FILE]");
                if (chunkUploadState && chunkUploadState.waitingRx) {
                    chunkUploadState.waitingRx = false;
                    $("uploadEta").textContent = "Đang truyền...";
                    (chunkUploadState.dispatch || chunkUploadState.uploadNext)();
                }
            }
        });
        sse.addEventListener("chunk_ack", e => {
            if (!isSender) {
                if (isDownloadingRx) {
                    try {
                        const d = JSON.parse(e.data);
                        const done = (d.index || 0) + 1;
                        const total = d.total || 1;
                        const pct = Math.min(100, Math.round((done / total) * 100));
                        $("recvProgressBar").style.width = `${pct}%`;
                        $("recvPercent").textContent = `${pct}%`;
                        $("recvTransferred").textContent = `Mảnh ${done}/${total} (Đã xả vào máy)`;
                        $("recvEta").textContent = done >= total ? "Đang ghi đĩa..." : "Tải ngầm trong nền...";
                        renderBatchGrid("rxBatchContainer", total, done, "downloading");
                    } catch (_) {}
                }
                return;
            }
            try {
                const d = JSON.parse(e.data);
                const done = (d.index || 0) + 1;
                const total = d.total || 1;
                const batchInfo = $("activeFileBatchInfo");
                if (batchInfo && $("fileActiveCard").style.display === "flex") {
                    batchInfo.textContent = done < total
                        ? `RX đang kéo mảnh ${done}/${total}...`
                        : `RX đã nhận đủ ${total}/${total} mảnh ✓`;
                }
            } catch (_) {}
        });
        sse.addEventListener("sender_paired", () => {
            const nr = $("nodeReceiver");
            setPairStatus(nr ? nr.textContent.includes("●") : false, true);
            if (!isSender) showToast("[TX ĐÃ KẾT NỐI]");
        });
        sse.addEventListener("sender_unpaired", () => {
            const nr = $("nodeReceiver");
            setPairStatus(nr ? nr.textContent.includes("●") : false, false);
            if (!isSender) showToast("[TX ĐÃ RỜI DÂY]");
        });
        sse.addEventListener("payload_updated", e => {
            const d = JSON.parse(e.data);
            if (isSender) {
                if (d.type === "file" && !isUploadingTx()) {
                    showActiveFile(d.name, d.size, d.total_chunks);
                } else if (d.type === "text" && !typing && !isUploadingTx()) {
                    $("senderTextarea").value = d.content || "";
                }
            }
            if (!isSender && !isDownloadingRx) {
                d.type === "text" ? showRecvText(d.content) : showRecvFile(d.name, d.size, d.total_chunks);
            }
        });
        sse.addEventListener("payload_cleared", () => {
            if (isSender && !isUploadingTx() && $("activeFileName").textContent !== "-") {
                $("activeFileName").textContent = "-";
                if ($("activeFileBatchRow")) $("activeFileBatchRow").style.display = "none";
                showFileDropzone();
                $("syncIndicator").textContent = "[ĐÃ CHUYỂN ✓]";
                showToast("[RX ĐÃ NHẬN XONG FILE ✓]");
            }
            if (!isSender) {
                if (isDownloadingRx) {
                    $("recvProgressBar").style.width = "100%";
                    $("recvPercent").textContent = "100%";
                    $("recvEta").textContent = "Hoàn tất ✓";
                    renderBatchGrid("rxBatchContainer", recvCurrentFile ? recvCurrentFile.totalChunks : 1, recvCurrentFile ? recvCurrentFile.totalChunks : 1, "downloading");
                    showToast("[TẢI VỀ HOÀN TẤT ✓]");
                    releaseWakeLock();
                    setTimeout(() => {
                        isDownloadingRx = false;
                        $("recvFileProgressCard").style.display = "none";
                        showRecvEmpty();
                    }, 2000);
                } else {
                    showRecvEmpty();
                    showToast("[DỮ LIỆU ĐÃ RÚT KHỎI DÂY]");
                }
            }
        });
        sse.addEventListener("payload_consumed", e => {
            try {
                const d = JSON.parse(e.data);
                if (isSender) {
                    const badge = $("deliveryBadge");
                    if (badge) {
                        const byStr = d.by === "cli" ? "RX CLI (curl/wget)" : `RX Web (${d.action})`;
                        badge.textContent = `✓ ĐÃ ĐẾN MÁY ĐÍCH [${byStr} lúc ${d.time}]`;
                        badge.style.display = "inline-block";
                        badge.className = "delivery-badge success";
                        showToast(`[✓ DỮ LIỆU ĐÃ ĐẾN MÁY ĐÍCH - ${byStr}]`);
                    }
                }
            } catch (_) {}
        });
        sse.onerror = () => {
            fetch(`/${slug}/events?role=${role}&conn=${cid}&check=1`).then(r => {
                if (r.status === 409) { r.json().then(d => showLockout(d.message)); if (sse) sse.close(); }
            });
        };
    }

    function setPairStatus(rxPaired, txPaired) {
        const nr = $("nodeReceiver"), ns = $("nodeSender"), ub = $("btnUnpair");
        if (nr) nr.textContent = rxPaired ? "RX ●" : "RX ○";
        if (ns && typeof txPaired === "boolean") ns.textContent = txPaired ? "TX ●" : "TX ○";
        if (ub) ub.style.display = (isSender && rxPaired) ? "inline-block" : "none";
    }

    function renderPayload(d) {
        if (isSender) {
            if (isUploadingTx()) return;
            if (d.slot_type === 2) {
                showActiveFile(d.slot_file_name, d.slot_file_size, d.total_chunks);
            } else if ($("btnModeFile").classList.contains("active") || location.search.includes("mode=file") || location.hash === "#file") {
                showFileDropzone();
            } else {
                if (!typing && d.slot_text !== undefined) $("senderTextarea").value = d.slot_text || "";
                showTextEditor();
            }
        } else {
            if (isDownloadingRx) return;
            if (d.slot_type === 2) showRecvFile(d.slot_file_name, d.slot_file_size, d.total_chunks);
            else if (d.slot_type === 1 && d.slot_text) showRecvText(d.slot_text);
            else showRecvEmpty();
        }
    }

    function showTextEditor() {
        $("btnModeText").classList.add("active");
        $("btnModeFile").classList.remove("active");
        $("editorContainer").style.display = "flex";
        $("fileDropzone").style.display = "none";
        updateCheatCode();
    }

    function showFileDropzone() {
        $("btnModeFile").classList.add("active");
        $("btnModeText").classList.remove("active");
        $("editorContainer").style.display = "none";
        $("fileDropzone").style.display = "flex";
        if (currentUploadXhr) {
            $("dropzoneBox").style.display = "none";
            $("fileActiveCard").style.display = "none";
            $("fileUploadProgressCard").style.display = "flex";
        } else if ($("activeFileName").textContent && $("activeFileName").textContent !== "-") {
            $("dropzoneBox").style.display = "none";
            $("fileUploadProgressCard").style.display = "none";
            $("fileActiveCard").style.display = "flex";
        } else {
            $("dropzoneBox").style.display = "flex";
            $("fileUploadProgressCard").style.display = "none";
            $("fileActiveCard").style.display = "none";
        }
        updateCheatCode();
    }

    function getOptimalChunkSize(fileSize) {
        if (!fileSize || fileSize <= 64 * 1024 * 1024) return 8 * 1024 * 1024;   // <= 64MB: 8MB/mảnh
        if (fileSize <= 512 * 1024 * 1024) return 16 * 1024 * 1024;              // <= 512MB: 16MB/mảnh
        return 32 * 1024 * 1024;                                                  // > 512MB: 32MB/mảnh
    }

    function showActiveFile(name, size, totalChunks) {
        showFileDropzone();
        $("dropzoneBox").style.display = "none";
        $("fileUploadProgressCard").style.display = "none";
        $("fileActiveCard").style.display = "flex";
        $("activeFileName").textContent = name;
        $("activeFileSize").textContent = fmtSize(size);
        const batchRow = $("activeFileBatchRow");
        const batchInfo = $("activeFileBatchInfo");
        if (batchRow && batchInfo) {
            const cSize = getOptimalChunkSize(size || 0);
            const tChunks = totalChunks || Math.max(1, Math.ceil((size || 0) / cSize));
            batchRow.style.display = "flex";
            batchInfo.textContent = tChunks > 1 ? `${tChunks} Mảnh (${fmtSize(cSize)}/mảnh) • Sẵn sàng` : `1 Mảnh duy nhất (${fmtSize(size)}) • Sẵn sàng`;
        }
        updateCheatCode();
    }

    function showRecvText(txt) {
        $("recvTextSection").style.display = "flex";
        $("recvFileSection").style.display = "none";
        $("recvTextDisplay").querySelector("code").textContent = txt || "# (trống)";
        $("btnCopyRecvText").style.display = "inline-block";
    }

    function showRecvFile(name, size, totalChunks) {
        const cSize = getOptimalChunkSize(size || 0);
        const tChunks = totalChunks || Math.max(1, Math.ceil((size || 0) / cSize));
        recvCurrentFile = { name, size, totalChunks: tChunks, chunkSize: cSize };

        $("recvTextSection").style.display = "none";
        $("recvFileSection").style.display = "flex";
        $("recvFileReadyCard").style.display = "flex";
        $("recvFileProgressCard").style.display = "none";

        $("recvFileName").textContent = name;
        $("recvFileSize").textContent = fmtSize(size);
        const batchNote = $("recvFileBatchNote");
        if (batchNote) {
            batchNote.textContent = tChunks > 1 ? `Đa Mảnh: ${tChunks} mảnh × ${fmtSize(cSize)}` : `Đơn Mảnh: 1 mảnh nguyên vẹn (${fmtSize(size)})`;
        }
        const mobileNotice = $("recvMobileNotice");
        const linkNative = $("linkNativeDownload");
        if (isMobileDevice()) {
            if (mobileNotice) mobileNotice.style.display = "block";
            if (linkNative) linkNative.style.display = "none";
            $("btnDownloadFile").textContent = "TẢI VỀ MÁY (CHẠY NGẦM)";
        } else {
            if (mobileNotice) mobileNotice.style.display = "none";
            if (linkNative) linkNative.style.display = window.showSaveFilePicker ? "inline-block" : "none";
            $("btnDownloadFile").textContent = "TẢI FILE VỀ MÁY";
        }
        $("btnCopyRecvText").style.display = "none";
    }

    function showRecvEmpty() {
        recvCurrentFile = null;
        $("recvTextSection").style.display = "flex";
        $("recvFileSection").style.display = "none";
        if ($("recvFileReadyCard")) $("recvFileReadyCard").style.display = "flex";
        if ($("recvFileProgressCard")) $("recvFileProgressCard").style.display = "none";
        $("recvTextDisplay").querySelector("code").textContent = "# Chờ dữ liệu từ TX...";
        $("btnCopyRecvText").style.display = "inline-block";
    }

    function renderBatchGrid(containerId, totalChunks, activeIndices, mode) {
        const container = $(containerId);
        if (!container) return;
        if (!totalChunks || totalChunks <= 0) {
            container.style.display = "none";
            container.innerHTML = "";
            return;
        }
        container.style.display = "flex";
        container.innerHTML = "";

        const maxVisible = 32;
        const count = Math.min(totalChunks, maxVisible);
        const activeList = Array.isArray(activeIndices) ? activeIndices : (typeof activeIndices === "number" ? [activeIndices] : []);
        const activeSet = new Set(activeList);
        const minActive = activeList.length > 0 ? Math.min(...activeList) : (totalChunks + 1);

        for (let i = 0; i < count; i++) {
            const chip = document.createElement("span");
            chip.className = "batch-chip";
            const num = i + 1;
            if (activeSet.has(i)) {
                chip.classList.add("active");
                chip.textContent = mode === "uploading" ? `⟳ Nạp ${num}` : `⬇ Nhận ${num}`;
            } else if (i < minActive) {
                chip.classList.add("done");
                chip.textContent = `✓ Mảnh ${num}`;
            } else {
                chip.classList.add("waiting");
                chip.textContent = `Mảnh ${num}`;
            }
            container.appendChild(chip);
        }

        if (totalChunks > maxVisible) {
            const extra = document.createElement("span");
            extra.className = "batch-chip waiting";
            extra.textContent = `+${totalChunks - maxVisible} mảnh khác...`;
            container.appendChild(extra);
        }
    }

    function fmtSize(b) {
        return b > 1048576 ? (b / 1048576).toFixed(1) + " MB" : Math.max(1, Math.round((b || 0) / 1024)) + " KB";
    }

    function fmtSpeed(bytesPerSec) {
        if (!bytesPerSec || bytesPerSec <= 0) return "0 KB/s";
        if (bytesPerSec >= 1048576) {
            return (bytesPerSec / 1048576).toFixed(1) + " MB/s";
        }
        return Math.round(bytesPerSec / 1024) + " KB/s";
    }

    function showUploadProgress(name, totalSize) {
        $("dropzoneBox").style.display = "none";
        $("fileActiveCard").style.display = "none";
        $("fileUploadProgressCard").style.display = "flex";

        $("uploadFileName").textContent = name;
        $("uploadProgressBar").style.width = "0%";
        $("uploadPercent").textContent = "0%";
        $("uploadTransferred").textContent = `0 KB / ${fmtSize(totalSize)}`;
        $("uploadSpeed").textContent = "0 KB/s";
        $("uploadEta").textContent = "Tính toán...";
    }

    const CHUNK_SIZE = 16 * 1024 * 1024; // 16MB chuẩn nhị phân

    function updateUploadProgressUI(percent, loaded, total, speedBytes, etaStr, currentChunk, totalChunks) {
        $("uploadProgressBar").style.width = `${percent}%`;
        $("uploadPercent").textContent = `${percent}%`;
        $("syncIndicator").textContent = `[NẠP: ${percent}%]`;
        const chunkInfo = (currentChunk && totalChunks) ? ` (Mảnh ${currentChunk}/${totalChunks})` : "";
        $("uploadTransferred").textContent = `${fmtSize(loaded)} / ${fmtSize(total)}${chunkInfo}`;
        $("uploadSpeed").textContent = fmtSpeed(speedBytes);
        $("uploadEta").textContent = etaStr;
    }

    function hideUploadProgress() {
        $("fileUploadProgressCard").style.display = "none";
        if ($("txBatchContainer")) $("txBatchContainer").style.display = "none";
        if ($("activeFileName").textContent && $("activeFileName").textContent !== "-") {
            $("fileActiveCard").style.display = "flex";
        } else {
            $("dropzoneBox").style.display = "flex";
        }
    }

    function autoSave() {
        if (!isSender || isHome) return;
        $("syncIndicator").textContent = "[LƯU...]";
        typing = true;
        clearTimeout(saveTimer);
        saveTimer = setTimeout(() => {
            const cid = getConnId();
            fetch(`/${slug}-s?conn=${cid}`, {
                method: "POST",
                headers: { "Content-Type": "text/plain; charset=utf-8" },
                body: $("senderTextarea").value
            }).then(r => {
                $("syncIndicator").textContent = r.ok ? "[ĐÃ LƯU ✓]" : (r.status === 409 ? "[BỊ KHÓA]" : "[LỖI]");
                if (r.ok) {
                    const badge = $("deliveryBadge");
                    if (badge && $("senderTextarea").value.trim()) {
                        badge.textContent = "[ĐANG CHỜ RX...]";
                        badge.className = "delivery-badge";
                        badge.style.display = "inline-block";
                    }
                    saveTxHistory($("senderTextarea").value);
                }
                if (r.status === 409) showLockout("TX đang bị chiếm giữ. Bấm [THỬ KẾT NỐI LẠI] để cắm lại dây!");
            }).catch(() => $("syncIndicator").textContent = "[MẤT MẠNG]")
              .finally(() => setTimeout(() => typing = false, 300));
        }, 300);
    }

    function uploadFile(file) {
        if (!file || !isSender || isHome) return;
        // Tất cả file (từ 1 batch <16MB đến nhiều batch >16MB) đều đi qua pipeline cắt mảnh đồng bộ
        uploadChunkedFile(file);
    }

    function uploadChunkedFile(file) {
        if (currentUploadXhr) {
            currentUploadXhr.abort();
            currentUploadXhr = null;
        }
        if (chunkUploadState) {
            chunkUploadState.cancelled = true;
            if (chunkUploadState.xhrMap) {
                chunkUploadState.xhrMap.forEach(x => { try { x.abort(); } catch (_) {} });
            }
            chunkUploadState = null;
        }

        const chunkSize = getOptimalChunkSize(file.size);
        const totalChunks = Math.max(1, Math.ceil(file.size / chunkSize));
        const sessionId = "sess_" + slug + "_" + Date.now().toString(36) + "_" + Math.random().toString(36).slice(2, 7);
        const cid = getConnId();

        showUploadProgress(file.name, file.size);
        $("syncIndicator").textContent = "[NẠP: 0%]";
        renderBatchGrid("txBatchContainer", totalChunks, [0], "uploading");

        const state = {
            file,
            chunkSize,
            sessionId,
            cid,
            totalChunks,
            nextChunkToDispatch: 0,
            activeWorkers: 0,
            activeIndices: new Set(),
            xhrMap: new Map(),
            chunkTransferred: new Array(totalChunks).fill(0),
            chunkCompleted: new Array(totalChunks).fill(false),
            retryCounts: new Array(totalChunks).fill(0),
            cancelled: false,
            waitingRx: false,
            startTime: Date.now(),
            dispatch: null
        };
        chunkUploadState = state;

        function updateProgress() {
            if (state.cancelled) return;
            const currentTotalTransferred = Math.min(file.size, state.chunkTransferred.reduce((a, b) => a + b, 0));
            const percent = Math.min(100, Math.round((currentTotalTransferred / file.size) * 100));
            const elapsedSec = (Date.now() - state.startTime) / 1000;
            let speedBytes = 0;
            if (elapsedSec > 0.2) speedBytes = currentTotalTransferred / elapsedSec;

            let etaStr = "--";
            if (speedBytes > 0 && currentTotalTransferred < file.size) {
                const remainingSec = Math.round((file.size - currentTotalTransferred) / speedBytes);
                if (remainingSec < 60) etaStr = `~${remainingSec}s`;
                else etaStr = `~${Math.floor(remainingSec / 60)}m ${remainingSec % 60}s`;
            } else if (percent === 100) {
                etaStr = "Xử lý...";
            }

            const activeList = Array.from(state.activeIndices).sort((a, b) => a - b);
            const activeDisplay = activeList.length > 0 ? activeList.map(n => n + 1).join("&") : state.totalChunks;
            updateUploadProgressUI(percent, currentTotalTransferred, file.size, speedBytes, etaStr, activeDisplay, state.totalChunks);
            renderBatchGrid("txBatchContainer", state.totalChunks, activeList, "uploading");
        }

        state.dispatch = function () {
            if (state.cancelled || state.waitingRx) return;

            const completedCount = state.chunkCompleted.filter(Boolean).length;
            if (completedCount >= state.totalChunks) {
                showActiveFile(file.name, file.size, state.totalChunks);
                $("syncIndicator").textContent = "[ĐÃ NẠP ✓]";
                showToast(`[ĐÃ NẠP ${file.name}]`);
                renderBatchGrid("txBatchContainer", state.totalChunks, [], "uploading");
                chunkUploadState = null;
                return;
            }

            // Chunk 0 nạp đơn lẻ để kiểm tra RX; khi RX online thì mở tối đa 2 luồng gối đầu song song
            const maxConcurrency = (state.nextChunkToDispatch === 0 && completedCount === 0) ? 1 : 2;

            while (state.activeWorkers < maxConcurrency && state.nextChunkToDispatch < state.totalChunks && !state.waitingRx && !state.cancelled) {
                const idx = state.nextChunkToDispatch++;
                state.activeWorkers++;
                state.activeIndices.add(idx);
                uploadSingleChunk(idx);
            }
            updateProgress();
        };

        function uploadSingleChunk(idx) {
            if (state.cancelled) return;

            const start = idx * state.chunkSize;
            const end = Math.min(file.size, start + state.chunkSize);
            const chunkBlob = file.slice(start, end);

            const xhr = new XMLHttpRequest();
            state.xhrMap.set(idx, xhr);
            currentUploadXhr = xhr;

            xhr.upload.onprogress = (e) => {
                if (state.cancelled) return;
                if (e.lengthComputable) {
                    state.chunkTransferred[idx] = Math.min(chunkBlob.size, e.loaded);
                    updateProgress();
                }
            };

            xhr.onload = () => {
                state.xhrMap.delete(idx);
                if (state.cancelled) return;

                if (xhr.status === 200) {
                    state.activeWorkers--;
                    state.activeIndices.delete(idx);
                    state.chunkTransferred[idx] = chunkBlob.size;
                    state.chunkCompleted[idx] = true;
                    state.retryCounts[idx] = 0;

                    let resData = {};
                    try { resData = JSON.parse(xhr.responseText); } catch (_) {}

                    if (idx === 0 && resData.status === "waiting_rx") {
                        state.waitingRx = true;
                        $("uploadEta").textContent = "Chờ RX...";
                        $("syncIndicator").textContent = "[⏸ CHỜ RX]";
                        showToast("[CHUNK 0 SẴN SÀNG • CHỜ RX KẾT NỐI]");
                        const firstChip = $("txBatchContainer") ? $("txBatchContainer").querySelector(".batch-chip") : null;
                        if (firstChip) {
                            firstChip.className = "batch-chip buffered";
                            firstChip.textContent = "⏸ Mảnh 1 (Chờ RX)";
                        }
                        return;
                    }

                    state.dispatch();
                } else if (xhr.status === 409) {
                    state.cancelled = true;
                    state.xhrMap.forEach(x => { try { x.abort(); } catch (_) {} });
                    hideUploadProgress();
                    $("syncIndicator").textContent = "[BỊ KHÓA]";
                    showLockout("TX đang bị chiếm giữ.");
                } else if (xhr.status === 410) {
                    state.cancelled = true;
                    state.xhrMap.forEach(x => { try { x.abort(); } catch (_) {} });
                    hideUploadProgress();
                    $("syncIndicator").textContent = "[RX ĐÃ NGẮT]";
                    showToast("[BÊN NHẬN ĐÃ NGẮT KẾT NỐI]");
                } else {
                    handleWorkerError(idx);
                }
            };

            xhr.onerror = () => {
                state.xhrMap.delete(idx);
                if (!state.cancelled) handleWorkerError(idx);
            };

            xhr.onabort = () => {
                state.xhrMap.delete(idx);
            };

            xhr.open("POST", `/${slug}-s/chunk?conn=${cid}`);
            xhr.setRequestHeader("X-Session-ID", state.sessionId);
            xhr.setRequestHeader("X-File-Name", encodeURIComponent(file.name));
            xhr.setRequestHeader("X-File-Size", file.size.toString());
            xhr.setRequestHeader("X-Chunk-Index", idx.toString());
            xhr.setRequestHeader("X-Total-Chunks", state.totalChunks.toString());
            xhr.setRequestHeader("X-Chunk-Size", chunkBlob.size.toString());
            xhr.setRequestHeader("X-Byte-Offset", start.toString());
            xhr.send(chunkBlob);
        }

        function handleWorkerError(idx) {
            if (state.cancelled) return;
            state.activeWorkers--;
            state.activeIndices.delete(idx);

            if (state.retryCounts[idx] < 3) {
                state.retryCounts[idx]++;
                const delay = Math.pow(2, state.retryCounts[idx] - 1) * 1000;
                $("uploadEta").textContent = `Mảnh ${idx + 1} thử lại sau ${delay / 1000}s...`;
                setTimeout(() => {
                    if (!state.cancelled) {
                        state.activeWorkers++;
                        state.activeIndices.add(idx);
                        uploadSingleChunk(idx);
                    }
                }, delay);
            } else {
                state.cancelled = true;
                state.xhrMap.forEach(x => { try { x.abort(); } catch (_) {} });
                hideUploadProgress();
                $("syncIndicator").textContent = "[LỖI MẠNG]";
                showToast(`[LỖI NẠP MẢNH ${idx + 1} - ĐÃ THỬ LẠI 3 LẦN]`);
            }
        }

        state.dispatch();
    }

    function updateHomeQuickLinks(s) {
        const tx = $("quickTxLink"), rx = $("quickRxLink");
        if (tx) { tx.href = `/${s}-s`; tx.textContent = `/${s}-s (TX)`; }
        if (rx) { rx.href = `/${s}-r`; rx.textContent = `/${s}-r (RX)`; }
        const urlRx = `${location.origin}/${s}-r`;
        const gtx = $("homeGuideTx"); if (gtx) gtx.textContent = `cat script.sh | wiredrop ${s}`;
        const gl = $("homeGuideLinux"); if (gl) gl.textContent = `curl -sL ${urlRx} | sh`;
        const ga = $("homeGuideAlpine"); if (ga) ga.textContent = `wget -qO- ${urlRx} | sh`;
        const gw = $("homeGuideWin"); if (gw) gw.textContent = `irm ${urlRx} | iex`;
        const gf = $("homeGuideFile"); if (gf) gf.textContent = `curl -sLJO ${urlRx}`;
    }

    function detectRole() {
        const path = location.pathname.replace(/^\/|\/$/g, "").toLowerCase();
        if (path === "") {
            isHome = true;
            document.title = "WireDrop - Web → Terminal Relay";
            $("homeView").style.display = "flex";
            $("senderView").style.display = $("receiverView").style.display = $("wireBadge").style.display = $("wireStatusCard").style.display = "none";
            if (!$("roomInput").value) $("roomInput").value = defaultSlug;
            updateHomeQuickLinks($("roomInput").value || defaultSlug);
            if (sse) { sse.close(); sse = null; }
            if (pollTimer) { clearInterval(pollTimer); pollTimer = null; }
            return;
        }
        isHome = false;
        $("homeView").style.display = "none";
        $("wireBadge").style.display = "inline-block";
        $("wireStatusCard").style.display = "flex";

        isSender = !path.endsWith("-r");
        slug = path.replace(/-[sr]$/, "") || defaultSlug;
        if (!path.endsWith("-s") && !path.endsWith("-r")) history.replaceState(null, "", `/${slug}-s`);

        $("badgeRole").textContent = isSender ? "TX" : "RX";
        $("badgeSlug").textContent = `/${slug}-${isSender ? "s" : "r"}`;
        document.title = `${isSender ? "TX" : "RX"}: /${slug}-${isSender ? "s" : "r"} • WireDrop`;

        $("senderView").style.display = isSender ? "flex" : "none";
        $("receiverView").style.display = isSender ? "none" : "flex";

        if (isSender) {
            renderTxHistory();
        }

        if (isSender && (location.search.includes("mode=file") || location.hash === "#file")) {
            showFileDropzone();
        }

        const tabParam = new URLSearchParams(location.search).get("tab");
        if (tabParam && ["linux", "alpine", "win", "browser"].includes(tabParam)) {
            os = tabParam;
            document.querySelectorAll(".os-tab").forEach(t => t.classList.toggle("active", t.dataset.os === os));
        }

        updateCheatCode();
        connectSSE();
    }

    // Event Listeners
    $("btnModeText").onclick = () => {
        showTextEditor();
        if (location.hash === "#file") history.replaceState(null, "", location.pathname + location.search);
        autoSave();
    };
    $("btnModeFile").onclick = () => {
        showFileDropzone();
        history.replaceState(null, "", location.pathname + location.search + "#file");
        if (currentUploadXhr) {
            $("dropzoneBox").style.display = "none";
            $("fileActiveCard").style.display = "none";
            $("fileUploadProgressCard").style.display = "flex";
        } else if ($("activeFileName").textContent && $("activeFileName").textContent !== "-") {
            $("dropzoneBox").style.display = "none";
            $("fileUploadProgressCard").style.display = "none";
            $("fileActiveCard").style.display = "flex";
        } else {
            $("dropzoneBox").style.display = "flex";
            $("fileUploadProgressCard").style.display = "none";
            $("fileActiveCard").style.display = "none";
        }
    };
    $("btnRemoveFile").onclick = () => {
        if (currentUploadXhr) {
            currentUploadXhr.abort();
            currentUploadXhr = null;
        }
        if (chunkUploadState) {
            chunkUploadState.cancelled = true;
            if (chunkUploadState.xhrMap) {
                chunkUploadState.xhrMap.forEach(x => { try { x.abort(); } catch (_) {} });
            }
            chunkUploadState = null;
        }
        fetch(`/${slug}-s/abort?conn=${getConnId()}`, { method: "POST" }).catch(() => {});
        $("activeFileName").textContent = "-";
        const batchRow = $("activeFileBatchRow");
        if (batchRow) batchRow.style.display = "none";
        $("dropzoneBox").style.display = "flex";
        $("fileActiveCard").style.display = "none";
        $("fileUploadProgressCard").style.display = "none";
        if (location.hash === "#file") history.replaceState(null, "", location.pathname + location.search);
        showTextEditor();
        autoSave();
    };
    $("btnCancelUpload").onclick = () => {
        if (currentUploadXhr) {
            currentUploadXhr.abort();
            currentUploadXhr = null;
        }
        if (chunkUploadState) {
            chunkUploadState.cancelled = true;
            if (chunkUploadState.xhrMap) {
                chunkUploadState.xhrMap.forEach(x => { try { x.abort(); } catch (_) {} });
            }
            chunkUploadState = null;
        }
        fetch(`/${slug}-s/abort?conn=${getConnId()}`, { method: "POST" }).catch(() => {});
        hideUploadProgress();
        $("syncIndicator").textContent = "[ĐÃ HỦY]";
        showToast("[ĐÃ HỦY TẢI LÊN]");
    };

    $("senderTextarea").oninput = autoSave;
    $("senderTextarea").onkeydown = e => {
        if (e.key === "Tab") {
            e.preventDefault();
            const s = $("senderTextarea").selectionStart;
            $("senderTextarea").value = $("senderTextarea").value.slice(0, s) + "    " + $("senderTextarea").value.slice($("senderTextarea").selectionEnd);
            $("senderTextarea").selectionStart = $("senderTextarea").selectionEnd = s + 4;
            autoSave();
        }
    };

    const dz = $("dropzoneBox"), fi = $("singleFileInput");
    dz.onclick = () => fi.click();
    fi.onchange = e => { if (e.target.files[0]) { uploadFile(e.target.files[0]); fi.value = ""; } };
    dz.ondragover = e => { e.preventDefault(); dz.classList.add("dragover"); };
    dz.ondragleave = () => dz.classList.remove("dragover");
    dz.ondrop = e => { e.preventDefault(); dz.classList.remove("dragover"); if (e.dataTransfer.files[0]) uploadFile(e.dataTransfer.files[0]); };

    $("btnCopyCheat").onclick = () => copyText($("cheatCode").textContent);
    $("btnCopyRecvText").onclick = () => {
        copyText($("recvTextDisplay").querySelector("code").textContent);
        fetch(`/${slug}/ack?action=copy`, { method: "POST" }).catch(() => {});
    };

    const btnShorten = $("btnShorten");
    const shortenTooltip = $("shortenTooltip");
    if (shortenTooltip) {
        shortenTooltip.onclick = () => {
            if (btnShorten && !btnShorten.disabled) {
                btnShorten.click();
            }
        };
    }
    if (btnShorten) {
        btnShorten.onclick = async () => {
            if (shortenTooltip) shortenTooltip.style.display = "none";
            btnShorten.textContent = "ĐANG TẠO...";
            btnShorten.disabled = true;
            try {
                const res = await fetch("/api/shorten", {
                    method: "POST",
                    headers: { "Content-Type": "application/json" },
                    body: JSON.stringify({ slug: slug })
                });
                const data = await res.json();
                if (data && data.short_r) {
                    activeShortURL = data.short_r;
                    updateCheatCode();
                    btnShorten.textContent = "✓ ĐÃ RÚT GỌN";
                    showToast("[ĐÃ TẠO SHORTLINK s.wd.io.vn]");
                } else {
                    btnShorten.textContent = "⚡ RÚT GỌN";
                    btnShorten.disabled = false;
                    showToast("[KHÔNG THỂ TẠO SHORTLINK]");
                }
            } catch (e) {
                btnShorten.textContent = "⚡ RÚT GỌN";
                btnShorten.disabled = false;
                showToast("[LỖI KẾT NỐI SHORTLINK GATEWAY]");
            }
        };
    }

    // Chế độ 1: Tải trực tiếp bằng Trình quản lý của Hệ điều hành (Native OS Download)
    // Dành cho Mobile (iOS Safari / Android Chrome) hoặc khi không có File System Access API
    // -> Ghi thẳng xuống ổ đĩa, 0 Byte RAM JavaScript, cho phép tắt màn hình, khóa máy hoặc chuyển app
    async function startNativeDownload(fileName, totalSize, totalChunks, chunkSize) {
        await acquireWakeLock();
        isDownloadingRx = true;
        $("recvFileReadyCard").style.display = "none";
        $("recvFileProgressCard").style.display = "flex";
        $("recvProgressFileName").textContent = fileName;
        $("recvProgressBar").style.width = "0%";
        $("recvPercent").textContent = "0%";
        $("recvTransferred").textContent = `0 KB / ${fmtSize(totalSize)}`;
        $("recvSpeed").textContent = "Native OS";
        $("recvEta").textContent = "Tải ngầm trong nền...";
        $("rxBatchContainer").style.display = totalChunks > 1 ? "flex" : "none";
        renderBatchGrid("rxBatchContainer", totalChunks, 0, "downloading");

        const a = document.createElement("a");
        a.href = `/${slug}-r?conn=${getConnId()}`;
        a.download = fileName;
        document.body.appendChild(a);
        a.click();
        setTimeout(() => a.remove(), 1000);
        showToast("[BẮT ĐẦU TẢI NGẦM - CHO PHÉP TẮT MÀN HÌNH]");
    }

    // Chế độ 2: Stream trực tiếp xuống file đã chọn trên ổ cứng (Desktop Chromium)
    async function startDiskStreamDownload(fileName, totalSize, totalChunks, chunkSize, writable) {
        await acquireWakeLock();
        isDownloadingRx = true;
        $("recvFileReadyCard").style.display = "none";
        $("recvFileProgressCard").style.display = "flex";
        $("recvProgressFileName").textContent = fileName;
        $("recvProgressBar").style.width = "0%";
        $("recvPercent").textContent = "0%";
        $("recvTransferred").textContent = `0 KB / ${fmtSize(totalSize)}`;
        $("recvSpeed").textContent = "0 KB/s";
        $("recvEta").textContent = "Đang kết nối...";

        $("rxBatchContainer").style.display = totalChunks > 1 ? "flex" : "none";
        renderBatchGrid("rxBatchContainer", totalChunks, 0, "downloading");

        const abortController = new AbortController();
        recvDownloadController = abortController;
        const startTime = Date.now();
        let receivedBytes = 0;

        try {
            const resp = await fetch(`/${slug}-r?conn=${getConnId()}`, { signal: abortController.signal });
            if (!resp.ok) throw new Error("HTTP error " + resp.status);

            const hdrChunks = parseInt(resp.headers.get("X-Total-Chunks"));
            if (hdrChunks > 0) totalChunks = hdrChunks;
            const hdrChunkSize = parseInt(resp.headers.get("X-Chunk-Size"));
            if (hdrChunkSize > 0) chunkSize = hdrChunkSize;

            const reader = resp.body.getReader();

            while (true) {
                const { done, value } = await reader.read();
                if (done) break;

                await writable.write(value);
                receivedBytes += value.length;

                const percent = totalSize > 0 ? Math.min(100, Math.round((receivedBytes / totalSize) * 100)) : 100;
                const currentBatch = Math.min(totalChunks, Math.floor(receivedBytes / chunkSize) + 1);
                const elapsedSec = (Date.now() - startTime) / 1000;
                let speedBytes = elapsedSec > 0.2 ? (receivedBytes / elapsedSec) : 0;
                let etaStr = "--";
                if (speedBytes > 0 && receivedBytes < totalSize) {
                    const remSec = Math.round((totalSize - receivedBytes) / speedBytes);
                    etaStr = remSec < 60 ? `~${remSec}s` : `~${Math.floor(remSec / 60)}m ${remSec % 60}s`;
                }

                $("recvProgressBar").style.width = `${percent}%`;
                $("recvPercent").textContent = `${percent}%`;
                $("recvTransferred").textContent = `${fmtSize(receivedBytes)} / ${fmtSize(totalSize)} (Mảnh ${currentBatch}/${totalChunks})`;
                $("recvSpeed").textContent = fmtSpeed(speedBytes);
                $("recvEta").textContent = etaStr;

                renderBatchGrid("rxBatchContainer", totalChunks, currentBatch - 1, "downloading");
            }

            await writable.close();
            writable = null;

            showToast(`[TẢI VỀ HOÀN TẤT ✓ - ${fmtSize(receivedBytes)}]`);
            $("recvProgressBar").style.width = "100%";
            $("recvPercent").textContent = "100%";
            $("recvEta").textContent = "Hoàn tất ✓";
            renderBatchGrid("rxBatchContainer", totalChunks, totalChunks, "downloading");

            setTimeout(() => {
                isDownloadingRx = false;
                releaseWakeLock();
                $("recvFileProgressCard").style.display = "none";
                showRecvEmpty();
            }, 1800);

        } catch (err) {
            isDownloadingRx = false;
            releaseWakeLock();
            if (writable) {
                try { await writable.abort(); } catch (_) {}
            }
            if (err.name !== "AbortError") {
                console.error("Lỗi tải file RX:", err);
                showToast("[LỖI TẢI VỀ MÁY]");
            }
            $("recvFileProgressCard").style.display = "none";
            $("recvFileReadyCard").style.display = "flex";
        } finally {
            recvDownloadController = null;
        }
    }

    $("btnDownloadFile").onclick = async () => {
        fetch(`/${slug}/ack?action=download`, { method: "POST" }).catch(() => {});
        const fileInfo = recvCurrentFile || { name: $("recvFileName").textContent, size: 0, totalChunks: 1 };
        const fileName = fileInfo.name && fileInfo.name !== "-" ? fileInfo.name : "download.bin";
        const totalSize = fileInfo.size || 0;
        let chunkSize = fileInfo.chunkSize || getOptimalChunkSize(totalSize);
        let totalChunks = fileInfo.totalChunks || Math.max(1, Math.ceil(totalSize / chunkSize));

        // 1. Trên Mobile HOẶC trình duyệt không hỗ trợ File System Access API:
        // Luôn ưu tiên Native OS Download để tắt màn hình, khóa máy vẫn tải ngầm 100%
        if (isMobileDevice() || !window.showSaveFilePicker) {
            startNativeDownload(fileName, totalSize, totalChunks, chunkSize);
            return;
        }

        // 2. Trên Desktop Chromium (có showSaveFilePicker):
        try {
            const fileHandle = await window.showSaveFilePicker({ suggestedName: fileName });
            const writable = await fileHandle.createWritable();
            startDiskStreamDownload(fileName, totalSize, totalChunks, chunkSize, writable);
        } catch (err) {
            if (err.name === "AbortError") return; // Người dùng bấm Hủy
            console.warn("showSaveFilePicker failed, fallback to native download:", err);
            startNativeDownload(fileName, totalSize, totalChunks, chunkSize);
        }
    };

    if ($("linkNativeDownload")) {
        $("linkNativeDownload").onclick = e => {
            e.preventDefault();
            const fileInfo = recvCurrentFile || { name: $("recvFileName").textContent, size: 0, totalChunks: 1 };
            const fileName = fileInfo.name && fileInfo.name !== "-" ? fileInfo.name : "download.bin";
            const totalSize = fileInfo.size || 0;
            let chunkSize = fileInfo.chunkSize || getOptimalChunkSize(totalSize);
            let totalChunks = fileInfo.totalChunks || Math.max(1, Math.ceil(totalSize / chunkSize));
            startNativeDownload(fileName, totalSize, totalChunks, chunkSize);
        };
    }

    $("btnCancelDownload").onclick = () => {
        isDownloadingRx = false;
        releaseWakeLock();
        if (recvDownloadController) {
            recvDownloadController.abort();
            recvDownloadController = null;
        }
        $("recvFileProgressCard").style.display = "none";
        $("recvFileReadyCard").style.display = "flex";
        showToast("[ĐÃ HỦY TẢI VỀ]");
    };
    $("btnRetryConn").onclick = () => connectSSE(true);
    $("btnUnpair").onclick = () => {
        if (confirm("Mở khóa RX?")) fetch(`/${slug}/reset-receiver`, { method: "POST" }).then(() => showToast("[ĐÃ MỞ KHÓA RX]"));
    };

    document.querySelectorAll(".os-tab").forEach(tab => {
        tab.onclick = () => {
            document.querySelectorAll(".os-tab").forEach(t => t.classList.remove("active"));
            tab.classList.add("active");
            os = tab.dataset.os;
            updateCheatCode();
        };
    });

    const openRoom = suf => {
        const s = ($("roomInput").value || "").trim().toLowerCase().replace(/[^a-z0-9_-]/g, "") || defaultSlug;
        location.href = `/${s}-${suf}`;
    };
    $("btnOpenSender").onclick = () => openRoom("s");
    $("btnOpenReceiver").onclick = () => openRoom("r");
    $("roomInput").oninput = () => {
        const s = ($("roomInput").value || "").trim().toLowerCase().replace(/[^a-z0-9_-]/g, "") || defaultSlug;
        updateHomeQuickLinks(s);
    };
    $("roomInput").onkeydown = e => { if (e.key === "Enter") openRoom("s"); };

    window.onhashchange = window.onpopstate = detectRole;
    detectRole();
})();
