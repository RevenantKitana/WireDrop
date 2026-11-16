// WireDrop v4.0.0 Client Logic - Pure Physical Connection Lock
(function () {
    let currentSlug = "1";
    let isSender = true;
    let isHome = false;
    let currentOS = "linux";
    let eventSource = null;
    let saveTimeout = null;
    let isLocalTyping = false;

    // Sinh mã phiên kết nối duy nhất cho cửa sổ/tab này trong RAM
    const connId = "c_" + Math.random().toString(36).substring(2, 10) + Date.now().toString(36);

    // DOM Elements
    const badgeRole = document.getElementById("badgeRole");
    const badgeSlug = document.getElementById("badgeSlug");
    const nodeSender = document.getElementById("nodeSender");
    const nodeReceiver = document.getElementById("nodeReceiver");
    const btnUnpair = document.getElementById("btnUnpair");
    const wireBadge = document.getElementById("wireBadge");
    const wireStatusCard = document.getElementById("wireStatusCard");

    // Views
    const homeView = document.getElementById("homeView");
    const senderView = document.getElementById("senderView");
    const receiverView = document.getElementById("receiverView");

    // Home Launcher Elements
    const roomInput = document.getElementById("roomInput");
    const btnOpenSender = document.getElementById("btnOpenSender");
    const btnOpenReceiver = document.getElementById("btnOpenReceiver");

    // Lockout Elements
    const lockoutModal = document.getElementById("lockoutModal");
    const lockoutTitle = document.getElementById("lockoutTitle");
    const lockoutMsg = document.getElementById("lockoutMsg");
    const btnRetryConn = document.getElementById("btnRetryConn");

    // Sender Elements
    const btnModeText = document.getElementById("btnModeText");
    const btnModeFile = document.getElementById("btnModeFile");
    const syncIndicator = document.getElementById("syncIndicator");
    const editorContainer = document.getElementById("editorContainer");
    const senderTextarea = document.getElementById("senderTextarea");

    const fileDropzone = document.getElementById("fileDropzone");
    const dropzoneBox = document.getElementById("dropzoneBox");
    const singleFileInput = document.getElementById("singleFileInput");
    const fileActiveCard = document.getElementById("fileActiveCard");
    const activeFileName = document.getElementById("activeFileName");
    const activeFileSize = document.getElementById("activeFileSize");
    const btnRemoveFile = document.getElementById("btnRemoveFile");

    const osTabs = document.querySelectorAll(".os-tab");
    const cheatCode = document.getElementById("cheatCode");
    const btnCopyCheat = document.getElementById("btnCopyCheat");

    // Receiver Elements
    const recvTextSection = document.getElementById("recvTextSection");
    const recvTextDisplay = document.getElementById("recvTextDisplay");
    const btnCopyRecvText = document.getElementById("btnCopyRecvText");

    const recvFileSection = document.getElementById("recvFileSection");
    const recvFileName = document.getElementById("recvFileName");
    const recvFileSize = document.getElementById("recvFileSize");
    const btnDownloadFile = document.getElementById("btnDownloadFile");

    const toast = document.getElementById("toast");

    // Phân giải URL để xác định Role (Home /#, TX *-s, hoặc RX *-r)
    function detectRoleAndSlug() {
        const path = window.location.pathname.replace(/^\/|\/$/g, "");
        const hash = window.location.hash;

        // Nếu đang ở trang chủ / hoặc /# hoặc /#guide
        if (path === "" || hash === "#" || hash === "#guide") {
            isHome = true;
            document.title = "WireDrop - Instant Web → Terminal Relay";
            if (homeView) homeView.style.display = "flex";
            if (senderView) senderView.style.display = "none";
            if (receiverView) receiverView.style.display = "none";
            if (wireBadge) wireBadge.style.display = "none";
            if (wireStatusCard) wireStatusCard.style.display = "none";

            if (eventSource) {
                eventSource.close();
                eventSource = null;
            }
            return;
        }

        isHome = false;
        if (homeView) homeView.style.display = "none";
        if (wireBadge) wireBadge.style.display = "inline-flex";
        if (wireStatusCard) wireStatusCard.style.display = "flex";

        if (path.endsWith("-r")) {
            isSender = false;
            currentSlug = path.slice(0, -2) || "1";
        } else if (path.endsWith("-s")) {
            isSender = true;
            currentSlug = path.slice(0, -2) || "1";
        } else {
            // Mặc định là TX phòng đó
            isSender = true;
            currentSlug = path || "1";
            window.history.replaceState(null, "", `/${currentSlug}-s`);
        }

        badgeRole.textContent = isSender ? "TX" : "RX";
        badgeSlug.textContent = `/${currentSlug}-${isSender ? "s" : "r"}`;

        if (isSender) {
            senderView.style.display = "flex";
            receiverView.style.display = "none";
            document.title = `TX: /${currentSlug}-s • WireDrop`;
        } else {
            senderView.style.display = "none";
            receiverView.style.display = "flex";
            document.title = `RX: /${currentSlug}-r • WireDrop`;
        }

        updateCheatCode();
        connectSSE();
    }

    function showToast(msg) {
        toast.textContent = msg;
        toast.classList.add("show");
        setTimeout(() => toast.classList.remove("show"), 1800);
    }

    function copyText(text, successMsg = "Đã copy ✓") {
        if (navigator.clipboard && navigator.clipboard.writeText) {
            navigator.clipboard.writeText(text).then(() => showToast(successMsg));
        } else {
            const ta = document.createElement("textarea");
            ta.value = text;
            document.body.appendChild(ta);
            ta.select();
            document.execCommand("copy");
            document.body.removeChild(ta);
            showToast(successMsg);
        }
    }

    // Cập nhật câu lệnh mẫu Cheat-sheet
    function updateCheatCode() {
        if (isHome) return;
        const origin = window.location.origin;
        const targetUrl = `${origin}/${currentSlug}-r`;
        const isFileMode = btnModeFile && btnModeFile.classList.contains("active");

        if (currentOS === "linux") {
            if (isFileMode) {
                cheatCode.textContent = `curl -sLJO ${targetUrl}`;
            } else {
                cheatCode.textContent = `curl -sL ${targetUrl} | sh`;
            }
        } else if (currentOS === "win") {
            if (isFileMode) {
                const fname = (activeFileName && activeFileName.textContent) || "file.bin";
                cheatCode.textContent = `iwr ${targetUrl} -OutFile ${fname}`;
            } else {
                cheatCode.textContent = `irm ${targetUrl} | iex`;
            }
        } else {
            cheatCode.textContent = targetUrl;
        }
    }

    // Hiển thị màn hình khóa khi bị 409 Line Busy
    function showLockout(msg) {
        lockoutTitle.textContent = isSender ? "TX ĐANG BẬN" : "RX ĐANG BẬN";
        lockoutMsg.textContent = msg || "Đầu dây này đang có một kết nối khác chiếm giữ. WireDrop chỉ cho phép đúng 1 kết nối tại một thời điểm.";
        lockoutModal.style.display = "flex";
    }

    function hideLockout() {
        lockoutModal.style.display = "none";
    }

    // Kết nối SSE để duy trì kết nối vật lý và theo dõi trạng thái
    function connectSSE() {
        if (isHome) return;

        if (eventSource) {
            eventSource.close();
            eventSource = null;
        }

        const role = isSender ? "sender" : "receiver";

        fetch(`/${currentSlug}/events?role=${role}&conn=${connId}&check=1`)
            .then(res => {
                if (res.status === 409) {
                    return res.json().then(data => {
                        showLockout(data.message);
                    });
                }
                if (res.ok) {
                    hideLockout();
                    startEventSource(role);
                }
            })
            .catch(() => {
                startEventSource(role);
            });
    }

    function startEventSource(role) {
        eventSource = new EventSource(`/${currentSlug}/events?role=${role}&conn=${connId}`);

        eventSource.addEventListener("init", (e) => {
            try {
                const data = JSON.parse(e.data);
                handleWireStatus(data);
                renderPayload(data);
            } catch (err) {}
        });

        eventSource.addEventListener("partner_paired", () => {
            if (nodeReceiver) {
                nodeReceiver.className = "status-node connected";
                nodeReceiver.textContent = "RX ●";
                nodeReceiver.title = "RX đã kết nối";
            }
            if (isSender && btnUnpair) btnUnpair.style.display = "inline-flex";
            showToast("RX đã kết nối ✓");
        });

        eventSource.addEventListener("partner_unpaired", () => {
            if (nodeReceiver) {
                nodeReceiver.className = "status-node waiting";
                nodeReceiver.textContent = "RX ○";
                nodeReceiver.title = "RX đang chờ";
            }
            if (btnUnpair) btnUnpair.style.display = "none";
        });

        eventSource.addEventListener("payload_updated", (e) => {
            try {
                const data = JSON.parse(e.data);
                if (isSender) {
                    if (data.type === "file") {
                        showActiveFile(data.name, data.size);
                    }
                } else {
                    if (data.type === "text") {
                        showRecvText(data.content);
                    } else if (data.type === "file") {
                        showRecvFile(data.name, data.size);
                    }
                }
            } catch (err) {}
        });

        eventSource.addEventListener("payload_cleared", () => {
            if (!isSender) {
                showRecvEmpty();
                showToast("Dữ liệu đã tự hủy ✓");
            }
        });

        eventSource.onerror = () => {
            fetch(`/${currentSlug}/events?role=${role}&conn=${connId}&check=1`)
                .then(res => {
                    if (res.status === 409) {
                        res.json().then(data => showLockout(data.message));
                        if (eventSource) eventSource.close();
                    }
                })
                .catch(() => {});
        };
    }

    // Cập nhật trạng thái kết nối TX ─── RX
    function handleWireStatus(data) {
        if (data.receiver_paired || data.paired) {
            if (nodeReceiver) {
                nodeReceiver.className = "status-node connected";
                nodeReceiver.textContent = "RX ●";
                nodeReceiver.title = "RX đã kết nối";
            }
            if (isSender && btnUnpair) btnUnpair.style.display = "inline-flex";
        } else {
            if (nodeReceiver) {
                nodeReceiver.className = "status-node waiting";
                nodeReceiver.textContent = "RX ○";
                nodeReceiver.title = "RX đang chờ";
            }
            if (btnUnpair) btnUnpair.style.display = "none";
        }
    }

    // Hiển thị payload khi vừa mở trang
    function renderPayload(data) {
        if (isSender) {
            if (data.slot_type === 2) {
                showActiveFile(data.slot_file_name, data.slot_file_size);
            } else {
                if (!isLocalTyping) {
                    senderTextarea.value = data.slot_text || "";
                }
                showTextEditor();
            }
        } else {
            if (data.slot_type === 2) {
                showRecvFile(data.slot_file_name, data.slot_file_size);
            } else if (data.slot_type === 1 && data.slot_text) {
                showRecvText(data.slot_text);
            } else {
                showRecvEmpty();
            }
        }
    }

    function showTextEditor() {
        btnModeText.classList.add("active");
        btnModeFile.classList.remove("active");
        editorContainer.style.display = "block";
        fileDropzone.style.display = "none";
        fileActiveCard.style.display = "none";
        updateCheatCode();
    }

    function showFileDropzone() {
        btnModeFile.classList.add("active");
        btnModeText.classList.remove("active");
        editorContainer.style.display = "none";
        fileDropzone.style.display = "block";
        updateCheatCode();
    }

    function showActiveFile(name, size) {
        showFileDropzone();
        dropzoneBox.style.display = "none";
        fileActiveCard.style.display = "flex";
        activeFileName.textContent = name;
        activeFileSize.textContent = formatSize(size);
        updateCheatCode();
    }

    function showRecvText(text) {
        recvTextSection.style.display = "block";
        recvFileSection.style.display = "none";
        recvTextDisplay.querySelector("code").textContent = text || "# (empty)";
    }

    function showRecvFile(name, size) {
        recvTextSection.style.display = "none";
        recvFileSection.style.display = "block";
        recvFileName.textContent = name;
        recvFileSize.textContent = formatSize(size);
        btnDownloadFile.href = `/${currentSlug}-r?conn=${connId}`;
        btnDownloadFile.download = name;
    }

    function showRecvEmpty() {
        recvTextSection.style.display = "block";
        recvFileSection.style.display = "none";
        recvTextDisplay.querySelector("code").textContent = "# Đang chờ dữ liệu...";
    }

    function formatSize(bytes) {
        if (!bytes) return "0 KB";
        if (bytes > 1024 * 1024) return (bytes / (1024 * 1024)).toFixed(1) + " MB";
        return Math.max(1, Math.round(bytes / 1024)) + " KB";
    }

    // Auto-save Text
    function autoSaveText() {
        if (!isSender || isHome) return;
        syncIndicator.textContent = "Lưu...";
        isLocalTyping = true;

        clearTimeout(saveTimeout);
        saveTimeout = setTimeout(() => {
            const content = senderTextarea.value;
            fetch(`/${currentSlug}-s?conn=${connId}`, {
                method: "POST",
                headers: { "Content-Type": "text/plain; charset=utf-8" },
                body: content
            })
            .then(res => {
                if (res.ok) {
                    syncIndicator.textContent = "Đã lưu ✓";
                } else if (res.status === 409) {
                    syncIndicator.textContent = "Bị khóa!";
                    showLockout("TX đang bị kết nối khác chiếm giữ.");
                } else {
                    syncIndicator.textContent = "Lỗi lưu!";
                }
            })
            .catch(() => {
                syncIndicator.textContent = "Mất mạng!";
            })
            .finally(() => {
                setTimeout(() => { isLocalTyping = false; }, 300);
            });
        }, 300);
    }

    // Upload 1 File
    function uploadSingleFile(file) {
        if (!file || !isSender || isHome) return;

        const formData = new FormData();
        formData.append("file", file);

        syncIndicator.textContent = "Đang nạp file...";
        fetch(`/${currentSlug}-s?conn=${connId}`, {
            method: "POST",
            body: formData
        })
        .then(res => {
            if (res.status === 409) {
                showLockout("TX đang bị chiếm giữ.");
                throw new Error("409");
            }
            return res.json();
        })
        .then(data => {
            showActiveFile(data.name, data.size);
            syncIndicator.textContent = "Đã nạp ✓";
            showToast(`Đã nạp ${data.name} ✓`);
        })
        .catch(err => {
            if (err.message !== "409") {
                syncIndicator.textContent = "Lỗi upload!";
            }
        });
    }

    // EVENT LISTENERS
    btnModeText.addEventListener("click", () => {
        showTextEditor();
        autoSaveText();
    });

    btnModeFile.addEventListener("click", () => {
        showFileDropzone();
        dropzoneBox.style.display = "block";
        fileActiveCard.style.display = "none";
    });

    btnRemoveFile.addEventListener("click", () => {
        showTextEditor();
        autoSaveText();
    });

    senderTextarea.addEventListener("input", autoSaveText);
    senderTextarea.addEventListener("keydown", (e) => {
        if (e.key === "Tab") {
            e.preventDefault();
            const start = senderTextarea.selectionStart;
            const end = senderTextarea.selectionEnd;
            senderTextarea.value = senderTextarea.value.substring(0, start) + "    " + senderTextarea.value.substring(end);
            senderTextarea.selectionStart = senderTextarea.selectionEnd = start + 4;
            autoSaveText();
        }
    });

    dropzoneBox.addEventListener("click", () => singleFileInput.click());
    singleFileInput.addEventListener("change", (e) => {
        if (e.target.files.length > 0) uploadSingleFile(e.target.files[0]);
    });

    dropzoneBox.addEventListener("dragover", (e) => { e.preventDefault(); dropzoneBox.classList.add("dragover"); });
    dropzoneBox.addEventListener("dragleave", () => dropzoneBox.classList.remove("dragover"));
    dropzoneBox.addEventListener("drop", (e) => {
        e.preventDefault();
        dropzoneBox.classList.remove("dragover");
        if (e.dataTransfer.files.length > 0) uploadSingleFile(e.dataTransfer.files[0]);
    });

    btnCopyCheat.addEventListener("click", () => copyText(cheatCode.textContent, "Đã copy lệnh ✓"));

    if (btnCopyRecvText) {
        btnCopyRecvText.addEventListener("click", () => {
            const code = recvTextDisplay.querySelector("code").textContent;
            copyText(code, "Đã copy nội dung ✓");
        });
    }

    // Reset RX
    btnUnpair.addEventListener("click", () => {
        if (confirm("Mở khóa RX để cho kết nối khác cắm vào?")) {
            fetch(`/${currentSlug}/reset-receiver`, { method: "POST" })
                .then(() => showToast("Đã mở khóa RX"));
        }
    });

    // Thử lại khi khóa
    btnRetryConn.addEventListener("click", () => {
        connectSSE();
    });

    // Chuyển OS Tabs
    osTabs.forEach(tab => {
        tab.addEventListener("click", () => {
            osTabs.forEach(t => t.classList.remove("active"));
            tab.classList.add("active");
            currentOS = tab.dataset.os;
            updateCheatCode();
        });
    });

    // Home Launcher Handlers
    if (btnOpenSender) {
        btnOpenSender.addEventListener("click", () => {
            const slug = (roomInput.value || "1").trim().replace(/[^a-zA-Z0-9_-]/g, "") || "1";
            window.location.href = `/${slug}-s`;
        });
    }

    if (btnOpenReceiver) {
        btnOpenReceiver.addEventListener("click", () => {
            const slug = (roomInput.value || "1").trim().replace(/[^a-zA-Z0-9_-]/g, "") || "1";
            window.location.href = `/${slug}-r`;
        });
    }

    if (roomInput) {
        roomInput.addEventListener("keydown", (e) => {
            if (e.key === "Enter") {
                const slug = (roomInput.value || "1").trim().replace(/[^a-zA-Z0-9_-]/g, "") || "1";
                window.location.href = `/${slug}-s`;
            }
        });
    }

    // Lắng nghe chuyển trang nội bộ
    window.addEventListener("hashchange", detectRoleAndSlug);
    window.addEventListener("popstate", detectRoleAndSlug);

    // KHỞI CHẠY
    detectRoleAndSlug();
})();
