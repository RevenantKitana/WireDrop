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
    const valSender = document.getElementById("valSender");
    const dotReceiver = document.getElementById("dotReceiver");
    const valReceiver = document.getElementById("valReceiver");
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
    const cheatHint = document.getElementById("cheatHint");
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

    // Phân giải URL để xác định Role (Home /#, Sender *-s, hoặc Receiver *-r)
    function detectRoleAndSlug() {
        const path = window.location.pathname.replace(/^\/|\/$/g, "");
        const hash = window.location.hash;

        // Nếu đang ở trang chủ / hoặc /# hoặc /#guide
        if (path === "" || hash === "#" || hash === "#guide") {
            isHome = true;
            document.title = "WireDrop - Sợi dây kết nối Web sang Terminal";
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
            // Nếu gõ link không có hậu tố (ví dụ /1)
            isSender = true;
            currentSlug = path || "1";
            window.history.replaceState(null, "", `/${currentSlug}-s`);
        }

        badgeRole.textContent = isSender ? "SENDER (PHÁT)" : "RECEIVER (NHẬN)";
        badgeSlug.textContent = `/${currentSlug}-${isSender ? "s" : "r"}`;

        if (isSender) {
            senderView.style.display = "flex";
            receiverView.style.display = "none";
            document.title = `WireDrop [Phát: /${currentSlug}-s]`;
        } else {
            senderView.style.display = "none";
            receiverView.style.display = "flex";
            document.title = `WireDrop [Nhận: /${currentSlug}-r]`;
        }

        updateCheatCode();
        connectSSE();
    }

    function showToast(msg) {
        toast.textContent = msg;
        toast.classList.add("show");
        setTimeout(() => toast.classList.remove("show"), 2000);
    }

    function copyText(text, successMsg = "Đã sao chép vào clipboard!") {
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
                cheatHint.textContent = "Tải file nguyên bản về thư mục hiện tại với đúng tên file gốc.";
            } else {
                cheatCode.textContent = `curl -sL ${targetUrl} | sh`;
                cheatHint.textContent = "Kéo script về và thực thi trực tiếp qua bash shell.";
            }
        } else if (currentOS === "win") {
            if (isFileMode) {
                const fname = activeFileName.textContent || "file.bin";
                cheatCode.textContent = `iwr ${targetUrl} -OutFile ${fname}`;
                cheatHint.textContent = "Tải file về máy qua PowerShell.";
            } else {
                cheatCode.textContent = `irm ${targetUrl} | iex`;
                cheatHint.textContent = "Tải lệnh về và thực thi trực tiếp trong PowerShell.";
            }
        } else {
            cheatCode.textContent = targetUrl;
            cheatHint.textContent = "Mở link trên trình duyệt máy B để xem và tải trực tiếp.";
        }
    }

    // Hiển thị màn hình khóa khi bị 409 Line Busy
    function showLockout(msg) {
        lockoutTitle.textContent = isSender ? "ĐẦU PHÁT ĐANG BỊ CHIẾM GIỮ" : "ĐẦU NHẬN ĐANG BỊ CHIẾM GIỮ";
        lockoutMsg.textContent = msg || "Đầu dây này đang có một cửa sổ hoặc tiến trình khác kết nối. Sợi dây tuân thủ nghiêm ngặt nguyên lý 1-1 vật lý: chỉ 1 kết nối được phép cắm tại một thời điểm.";
        lockoutModal.style.display = "flex";
    }

    function hideLockout() {
        lockoutModal.style.display = "none";
    }

    // Kết nối SSE để theo dõi trạng thái sợi dây và duy trì kết nối vật lý
    function connectSSE() {
        if (isHome) return;

        if (eventSource) {
            eventSource.close();
            eventSource = null;
        }

        const role = isSender ? "sender" : "receiver";

        // Bước 1: Pre-check xem đầu dây có bị cửa sổ khác chiếm giữ không
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
            } catch (err) {
                console.error("Lỗi parse SSE init:", err);
            }
        });

        eventSource.addEventListener("partner_paired", () => {
            dotReceiver.className = "status-dot connected";
            valReceiver.textContent = "Đã cắm kết nối (Khóa 1-1 ✓)";
            if (isSender) btnUnpair.style.display = "inline-flex";
            showToast("Máy B đã cắm vào đầu nhận!");
        });

        eventSource.addEventListener("partner_unpaired", () => {
            dotReceiver.className = "status-dot waiting";
            valReceiver.textContent = "Đang chờ cắm dây...";
            btnUnpair.style.display = "none";
        });

        eventSource.addEventListener("payload_updated", (e) => {
            try {
                const data = JSON.parse(e.data);
                if (isSender) {
                    // Trên Sender: Không ghi đè nếu đang tự tay gõ
                    if (data.type === "file") {
                        showActiveFile(data.name, data.size);
                    }
                } else {
                    // Trên Receiver: Cập nhật ngay lập tức giao diện Chỉ Đọc
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
                showToast("Dữ liệu đã được tiêu thụ. Đường ống tự hủy!");
            }
        });

        eventSource.onerror = () => {
            // Kiểm tra xem có bị ngắt do kết nối khác chiếm hay không
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

    // Cập nhật đèn trạng thái dây 1-1
    function handleWireStatus(data) {
        if (data.receiver_paired || data.paired) {
            dotReceiver.className = "status-dot connected";
            valReceiver.textContent = "Đã cắm kết nối (Khóa 1-1 ✓)";
            if (isSender) btnUnpair.style.display = "inline-flex";
        } else {
            dotReceiver.className = "status-dot waiting";
            valReceiver.textContent = "Đang chờ cắm dây...";
            btnUnpair.style.display = "none";
        }
    }

    // Hiển thị vật phẩm trong khe cắm khi vừa mở trang
    function renderPayload(data) {
        if (isSender) {
            if (data.slot_type === 2) { // File
                showActiveFile(data.slot_file_name, data.slot_file_size);
            } else {
                if (!isLocalTyping) {
                    senderTextarea.value = data.slot_text || "";
                }
                showTextEditor();
            }
        } else {
            // Receiver
            if (data.slot_type === 2) {
                showRecvFile(data.slot_file_name, data.slot_file_size);
            } else if (data.slot_type === 1 && data.slot_text) {
                showRecvText(data.slot_text);
            } else {
                showRecvEmpty();
            }
        }
    }

    // Chuyển sang chế độ soạn Text trên Sender
    function showTextEditor() {
        btnModeText.classList.add("active");
        btnModeFile.classList.remove("active");
        editorContainer.style.display = "block";
        fileDropzone.style.display = "none";
        fileActiveCard.style.display = "none";
        updateCheatCode();
    }

    // Chuyển sang chế độ File trên Sender
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
        recvFileSize.textContent = `Dung lượng: ${formatSize(size)}`;
        btnDownloadFile.href = `/${currentSlug}-r?conn=${connId}`;
        btnDownloadFile.download = name;
    }

    function showRecvEmpty() {
        recvTextSection.style.display = "block";
        recvFileSection.style.display = "none";
        recvTextDisplay.querySelector("code").textContent = "# Đang chờ Máy A phát dữ liệu...";
    }

    function formatSize(bytes) {
        if (!bytes) return "0 KB";
        if (bytes > 1024 * 1024) return (bytes / (1024 * 1024)).toFixed(1) + " MB";
        return Math.max(1, Math.round(bytes / 1024)) + " KB";
    }

    // Auto-save Text trên Sender kèm theo connId
    function autoSaveText() {
        if (!isSender || isHome) return;
        syncIndicator.textContent = "Đang lưu...";
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
                    syncIndicator.textContent = "Đã nạp vào ống ✓";
                } else if (res.status === 409) {
                    syncIndicator.textContent = "Bị khóa (Line Busy)!";
                    showLockout("Cửa sổ này đã mất quyền phát vì có kết nối khác!");
                } else {
                    syncIndicator.textContent = "Lỗi nạp!";
                }
            })
            .catch(() => {
                syncIndicator.textContent = "Mất kết nối!";
            })
            .finally(() => {
                setTimeout(() => { isLocalTyping = false; }, 300);
            });
        }, 300);
    }

    // Upload duy nhất 1 File kèm theo connId
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
                showLockout("Không thể upload: Đầu phát đang bị chiếm!");
                throw new Error("409");
            }
            return res.json();
        })
        .then(data => {
            showActiveFile(data.name, data.size);
            syncIndicator.textContent = "File đã nạp vào ống ✓";
            showToast(`Đã nạp file ${data.name} vào đường ống!`);
        })
        .catch(err => {
            if (err.message !== "409") {
                syncIndicator.textContent = "Lỗi upload file!";
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

    btnCopyCheat.addEventListener("click", () => copyText(cheatCode.textContent, "Đã copy câu lệnh cho máy B!"));

    if (btnCopyRecvText) {
        btnCopyRecvText.addEventListener("click", () => {
            const code = recvTextDisplay.querySelector("code").textContent;
            copyText(code, "Đã copy toàn bộ nội dung lệnh!");
        });
    }

    // Mở khóa đầu nhận (Reset receiver)
    btnUnpair.addEventListener("click", () => {
        if (confirm("Mở khóa đầu nhận để cho phép máy khác cắm vào sợi dây?")) {
            fetch(`/${currentSlug}/reset-receiver`, { method: "POST" })
                .then(() => showToast("Đã mở khóa đầu nhận!"));
        }
    });

    // Thử kết nối lại trên Lockout Modal
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
