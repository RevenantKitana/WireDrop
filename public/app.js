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

    function updateCheatCode() {
        if (isHome) return;
        const urlRx = `${location.origin}/${slug}-r`;
        const urlTx = `${location.origin}/${slug}-s`;
        const isFile = $("btnModeFile").classList.contains("active");
        const titleEl = $("rxBarTitle");

        if (os === "tx") {
            if (titleEl) titleEl.textContent = "LỆNH ĐẨY DỮ LIỆU TỪ TERMINAL (TX):";
            if (isFile) {
                const fname = ($("activeFileName").textContent && $("activeFileName").textContent !== "-") ? $("activeFileName").textContent : "file.ext";
                $("cheatCode").textContent = `curl -s -F "file=@${fname}" ${urlTx}`;
            } else {
                $("cheatCode").textContent = `curl -s -X POST --data-binary @script.sh ${urlTx}`;
            }
        } else {
            if (titleEl) titleEl.textContent = "LỆNH CHẠY TRÊN MÁY ĐÍCH (RX):";
            if (os === "linux") {
                $("cheatCode").textContent = isFile ? `curl -sLJO ${urlRx}` : `curl -sL ${urlRx} | sh`;
            } else if (os === "win") {
                const fname = ($("activeFileName").textContent && $("activeFileName").textContent !== "-") ? $("activeFileName").textContent : "file.bin";
                $("cheatCode").textContent = isFile ? `iwr ${urlRx} -OutFile ${fname}` : `irm ${urlRx} | iex`;
            } else {
                $("cheatCode").textContent = urlRx;
            }
        }
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
                            chunkUploadState.uploadNext();
                        }
                        if (!isSender) {
                            renderPayload(d);
                        } else {
                            if (d.slot_type === 2 && $("activeFileName").textContent !== d.slot_file_name) {
                                showActiveFile(d.slot_file_name, d.slot_file_size);
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
                    chunkUploadState.uploadNext();
                }
            }
        });
        sse.addEventListener("partner_unpaired", () => {
            const ns = $("nodeSender");
            setPairStatus(false, ns ? ns.textContent.includes("●") : true);
            if (isSender) showToast("[RX ĐÃ NGẮT]");
        });
        sse.addEventListener("rx_connected", () => {
            const nr = $("nodeReceiver");
            setPairStatus(true, $("nodeSender") ? $("nodeSender").textContent.includes("●") : true);
            if (isSender) {
                showToast("[RX ĐÃ BẮT ĐẦU KÉO FILE]");
                if (chunkUploadState && chunkUploadState.waitingRx) {
                    chunkUploadState.waitingRx = false;
                    $("uploadEta").textContent = "Đang truyền...";
                    chunkUploadState.uploadNext();
                }
            }
        });
        sse.addEventListener("chunk_ack", () => {});
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
                if (d.type === "file") {
                    showActiveFile(d.name, d.size);
                } else if (d.type === "text" && !typing) {
                    $("senderTextarea").value = d.content || "";
                }
            }
            if (!isSender) {
                d.type === "text" ? showRecvText(d.content) : showRecvFile(d.name, d.size);
            }
        });
        sse.addEventListener("payload_cleared", () => {
            if (!isSender) { showRecvEmpty(); showToast("[DỮ LIỆU ĐÃ RÚT KHỎI DÂY]"); }
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
            if (d.slot_type === 2) {
                showActiveFile(d.slot_file_name, d.slot_file_size);
            } else if ($("btnModeFile").classList.contains("active") || location.search.includes("mode=file") || location.hash === "#file") {
                showFileDropzone();
            } else {
                if (!typing && d.slot_text !== undefined) $("senderTextarea").value = d.slot_text || "";
                showTextEditor();
            }
        } else {
            if (d.slot_type === 2) showRecvFile(d.slot_file_name, d.slot_file_size);
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

    function showActiveFile(name, size) {
        showFileDropzone();
        $("dropzoneBox").style.display = "none";
        $("fileUploadProgressCard").style.display = "none";
        $("fileActiveCard").style.display = "flex";
        $("activeFileName").textContent = name;
        $("activeFileSize").textContent = fmtSize(size);
        updateCheatCode();
    }

    function showRecvText(txt) {
        $("recvTextSection").style.display = "flex";
        $("recvFileSection").style.display = "none";
        $("recvTextDisplay").querySelector("code").textContent = txt || "# (trống)";
        $("btnCopyRecvText").style.display = "inline-block";
    }

    function showRecvFile(name, size) {
        $("recvTextSection").style.display = "none";
        $("recvFileSection").style.display = "flex";
        $("recvFileName").textContent = name;
        $("recvFileSize").textContent = fmtSize(size);
        $("btnDownloadFile").href = `/${slug}-r?conn=${getConnId()}`;
        $("btnDownloadFile").download = name;
        $("btnCopyRecvText").style.display = "none";
    }

    function showRecvEmpty() {
        $("recvTextSection").style.display = "flex";
        $("recvFileSection").style.display = "none";
        $("recvTextDisplay").querySelector("code").textContent = "# Chờ dữ liệu từ TX...";
        $("btnCopyRecvText").style.display = "inline-block";
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
                if (r.status === 409) showLockout("TX đang bị chiếm giữ. Bấm [THỬ KẾT NỐI LẠI] để cắm lại dây!");
            }).catch(() => $("syncIndicator").textContent = "[MẤT MẠNG]")
              .finally(() => setTimeout(() => typing = false, 300));
        }, 300);
    }

    function uploadFile(file) {
        if (!file || !isSender || isHome) return;
        if (file.size <= CHUNK_SIZE) {
            uploadSingleFile(file);
        } else {
            uploadChunkedFile(file);
        }
    }

    function uploadSingleFile(file) {
        if (currentUploadXhr) {
            currentUploadXhr.abort();
            currentUploadXhr = null;
        }

        showUploadProgress(file.name, file.size);
        $("syncIndicator").textContent = "[NẠP: 0%]";

        const fd = new FormData();
        fd.append("file", file);
        const cid = getConnId();

        const xhr = new XMLHttpRequest();
        currentUploadXhr = xhr;
        const startTime = Date.now();

        xhr.upload.onprogress = (e) => {
            if (e.lengthComputable) {
                const percent = Math.min(100, Math.round((e.loaded / e.total) * 100));
                const elapsedSec = (Date.now() - startTime) / 1000;
                let speedBytes = 0;
                if (elapsedSec > 0.2) speedBytes = e.loaded / elapsedSec;

                let etaStr = "--";
                if (speedBytes > 0 && e.loaded < e.total) {
                    const remainingSec = Math.round((e.total - e.loaded) / speedBytes);
                    if (remainingSec < 60) etaStr = `~${remainingSec}s`;
                    else etaStr = `~${Math.floor(remainingSec / 60)}m ${remainingSec % 60}s`;
                } else if (percent === 100) {
                    etaStr = "Xử lý...";
                }

                updateUploadProgressUI(percent, e.loaded, e.total, speedBytes, etaStr, 1, 1);
            }
        };

        xhr.onload = () => {
            currentUploadXhr = null;
            if (xhr.status === 200) {
                try {
                    const d = JSON.parse(xhr.responseText);
                    showActiveFile(d.name, d.size);
                    $("syncIndicator").textContent = "[ĐÃ NẠP ✓]";
                    showToast(`[ĐÃ NẠP ${d.name}]`);
                } catch (_) {
                    showActiveFile(file.name, file.size);
                    $("syncIndicator").textContent = "[ĐÃ NẠP ✓]";
                }
            } else if (xhr.status === 409) {
                hideUploadProgress();
                $("syncIndicator").textContent = "[BỊ KHÓA]";
                showLockout("TX đang bị chiếm giữ. Bấm [THỬ KẾT NỐI LẠI] để cắm lại dây!");
            } else {
                hideUploadProgress();
                $("syncIndicator").textContent = "[LỖI " + xhr.status + "]";
                showToast("[LỖI NẠP FILE]");
            }
        };

        xhr.onerror = () => {
            currentUploadXhr = null;
            hideUploadProgress();
            $("syncIndicator").textContent = "[MẤT MẠNG]";
            showToast("[LỖI KẾT NỐI MẠNG]");
        };

        xhr.onabort = () => {
            currentUploadXhr = null;
            hideUploadProgress();
            $("syncIndicator").textContent = "[ĐÃ HỦY]";
            showToast("[ĐÃ HỦY TẢI LÊN]");
        };

        xhr.open("POST", `/${slug}-s?conn=${cid}`);
        xhr.send(fd);
    }

    function uploadChunkedFile(file) {
        if (currentUploadXhr) {
            currentUploadXhr.abort();
            currentUploadXhr = null;
        }
        if (chunkUploadState) {
            chunkUploadState.cancelled = true;
            chunkUploadState = null;
        }

        const totalChunks = Math.ceil(file.size / CHUNK_SIZE);
        const sessionId = "sess_" + slug + "_" + Date.now().toString(36) + "_" + Math.random().toString(36).slice(2, 7);
        const cid = getConnId();

        showUploadProgress(file.name, file.size);
        $("syncIndicator").textContent = "[NẠP: 0%]";

        const state = {
            file,
            sessionId,
            cid,
            totalChunks,
            currentChunk: 0,
            cancelled: false,
            waitingRx: false,
            startTime: Date.now(),
            retryCount: 0,
            uploadNext: null
        };
        chunkUploadState = state;

        state.uploadNext = function () {
            if (state.cancelled) return;
            if (state.currentChunk >= state.totalChunks) {
                showActiveFile(file.name, file.size);
                $("syncIndicator").textContent = "[ĐÃ NẠP ✓]";
                showToast(`[ĐÃ NẠP ${file.name}]`);
                chunkUploadState = null;
                return;
            }

            const idx = state.currentChunk;
            const start = idx * CHUNK_SIZE;
            const end = Math.min(file.size, start + CHUNK_SIZE);
            const chunkBlob = file.slice(start, end);

            const xhr = new XMLHttpRequest();
            currentUploadXhr = xhr;

            xhr.upload.onprogress = (e) => {
                if (state.cancelled) return;
                if (e.lengthComputable) {
                    const currentTotalTransferred = start + e.loaded;
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

                    updateUploadProgressUI(percent, currentTotalTransferred, file.size, speedBytes, etaStr, idx + 1, state.totalChunks);
                }
            };

            xhr.onload = () => {
                currentUploadXhr = null;
                if (state.cancelled) return;

                if (xhr.status === 200) {
                    state.retryCount = 0;
                    let resData = {};
                    try { resData = JSON.parse(xhr.responseText); } catch (_) {}

                    if (resData.status === "waiting_rx") {
                        state.waitingRx = true;
                        state.currentChunk = 1;
                        $("uploadEta").textContent = "Chờ RX...";
                        $("syncIndicator").textContent = "[⏸ CHỜ RX]";
                        showToast("[CHUNK 0 SẴN SÀNG • CHỜ RX KẾT NỐI]");
                        return;
                    }

                    state.currentChunk++;
                    state.uploadNext();
                } else if (xhr.status === 409) {
                    hideUploadProgress();
                    $("syncIndicator").textContent = "[BỊ KHÓA]";
                    showLockout("TX đang bị chiếm giữ.");
                } else if (xhr.status === 410) {
                    hideUploadProgress();
                    $("syncIndicator").textContent = "[RX ĐÃ NGẮT]";
                    showToast("[BÊN NHẬN ĐÃ NGẮT KẾT NỐI]");
                } else {
                    handleChunkError();
                }
            };

            xhr.onerror = () => {
                currentUploadXhr = null;
                if (!state.cancelled) handleChunkError();
            };

            xhr.onabort = () => {
                currentUploadXhr = null;
                hideUploadProgress();
                $("syncIndicator").textContent = "[ĐÃ HỦY]";
                showToast("[ĐÃ HỦY TẢI LÊN]");
            };

            xhr.open("POST", `/${slug}-s/chunk?conn=${cid}`);
            xhr.setRequestHeader("X-Session-ID", state.sessionId);
            xhr.setRequestHeader("X-File-Name", encodeURIComponent(file.name));
            xhr.setRequestHeader("X-File-Size", file.size.toString());
            xhr.setRequestHeader("X-Chunk-Index", idx.toString());
            xhr.setRequestHeader("X-Total-Chunks", state.totalChunks.toString());
            xhr.setRequestHeader("X-Chunk-Size", chunkBlob.size.toString());
            xhr.send(chunkBlob);
        };

        function handleChunkError() {
            if (state.cancelled) return;
            if (state.retryCount < 3) {
                state.retryCount++;
                const delay = Math.pow(2, state.retryCount - 1) * 1000;
                $("uploadEta").textContent = `Thử lại sau ${delay / 1000}s...`;
                setTimeout(() => {
                    if (!state.cancelled) state.uploadNext();
                }, delay);
            } else {
                hideUploadProgress();
                $("syncIndicator").textContent = "[LỖI MẠNG]";
                showToast("[LỖI NẠP CHUNK - ĐÃ THỬ LẠI 3 LẦN]");
            }
        }

        state.uploadNext();
    }

    function updateHomeQuickLinks(s) {
        const tx = $("quickTxLink"), rx = $("quickRxLink");
        if (tx) { tx.href = `/${s}-s`; tx.textContent = `/${s}-s (TX)`; }
        if (rx) { rx.href = `/${s}-r`; rx.textContent = `/${s}-r (RX)`; }
        const urlRx = `${location.origin}/${s}-r`;
        const urlTx = `${location.origin}/${s}-s`;
        const gtx = $("homeGuideTx"); if (gtx) gtx.textContent = `curl -s -X POST --data-binary @script.sh ${urlTx}`;
        const gtxp = $("homeGuideTxPipe"); if (gtxp) gtxp.textContent = `echo "cmd" | curl -s -X POST --data-binary @- ${urlTx}`;
        const gl = $("homeGuideLinux"); if (gl) gl.textContent = `curl -sL ${urlRx} | sh`;
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

        if (isSender && (location.search.includes("mode=file") || location.hash === "#file")) {
            showFileDropzone();
        }

        const tabParam = new URLSearchParams(location.search).get("tab");
        if (tabParam && ["linux", "win", "tx", "browser"].includes(tabParam)) {
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
            chunkUploadState = null;
        }
        fetch(`/${slug}-s/abort?conn=${getConnId()}`, { method: "POST" }).catch(() => {});
        $("activeFileName").textContent = "-";
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
    $("btnCopyRecvText").onclick = () => copyText($("recvTextDisplay").querySelector("code").textContent);
    $("btnDownloadFile").onclick = async (e) => {
        if (window.showSaveFilePicker) {
            e.preventDefault();
            const btn = $("btnDownloadFile");
            const originalText = btn.textContent;
            try {
                const name = $("recvFileName").textContent || "download.bin";
                const handle = await window.showSaveFilePicker({
                    suggestedName: name
                });
                const writable = await handle.createWritable();
                btn.textContent = "ĐANG TẢI VỀ MÁY...";
                btn.style.pointerEvents = "none";
                btn.style.opacity = "0.7";

                const resp = await fetch(`/${slug}-r?conn=${getConnId()}`);
                if (!resp.ok) {
                    throw new Error("HTTP error " + resp.status);
                }

                if (resp.body && resp.body.pipeTo) {
                    await resp.body.pipeTo(writable);
                } else if (resp.body) {
                    const reader = resp.body.getReader();
                    while (true) {
                        const { done, value } = await reader.read();
                        if (done) break;
                        await writable.write(value);
                    }
                    await writable.close();
                } else {
                    const blob = await resp.blob();
                    await writable.write(blob);
                    await writable.close();
                }
                showToast("[TẢI FILE THÀNH CÔNG ✓]");
            } catch (err) {
                if (err.name !== "AbortError") {
                    console.warn("Direct-to-disk error, fallback to native download:", err);
                    location.href = `/${slug}-r?conn=${getConnId()}`;
                }
            } finally {
                btn.textContent = originalText;
                btn.style.pointerEvents = "auto";
                btn.style.opacity = "1";
            }
        }
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
