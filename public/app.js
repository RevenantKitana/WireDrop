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
    let slug = defaultSlug, isSender = true, isHome = false, os = "linux", sse = null, saveTimer = null, typing = false;
    const connId = "c_" + Math.random().toString(36).slice(2, 9) + Date.now().toString(36);

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
        const url = `${location.origin}/${slug}-r`;
        const isFile = $("btnModeFile").classList.contains("active");
        if (os === "linux") {
            $("cheatCode").textContent = isFile ? `curl -sLJO ${url}` : `curl -sL ${url} | sh`;
        } else if (os === "win") {
            const fname = $("activeFileName").textContent !== "-" ? $("activeFileName").textContent : "file.bin";
            $("cheatCode").textContent = isFile ? `iwr ${url} -OutFile ${fname}` : `irm ${url} | iex`;
        } else {
            $("cheatCode").textContent = url;
        }
    }

    function showLockout(msg) {
        $("lockoutTitle").textContent = (isSender ? "TX" : "RX") + " ĐANG BẬN (409)";
        $("lockoutMsg").textContent = msg || "Đầu dây này đã có 1 kết nối khác cắm vào. WireDrop duy trì khóa 1-1.";
        $("lockoutModal").style.display = "flex";
    }

    function connectSSE() {
        if (isHome) return;
        if (sse) { sse.close(); sse = null; }
        const role = isSender ? "sender" : "receiver";
        fetch(`/${slug}/events?role=${role}&conn=${connId}&check=1`).then(res => {
            if (res.status === 409) return res.json().then(d => showLockout(d.message));
            if (res.ok) {
                $("lockoutModal").style.display = "none";
                startSSE(role);
            }
        }).catch(() => startSSE(role));
    }

    function startSSE(role) {
        sse = new EventSource(`/${slug}/events?role=${role}&conn=${connId}`);
        sse.addEventListener("init", e => {
            const d = JSON.parse(e.data);
            setPairStatus(d.receiver_paired || d.paired);
            renderPayload(d);
        });
        sse.addEventListener("partner_paired", () => { setPairStatus(true); showToast("[RX ĐÃ KẾT NỐI]"); });
        sse.addEventListener("partner_unpaired", () => setPairStatus(false));
        sse.addEventListener("payload_updated", e => {
            const d = JSON.parse(e.data);
            if (isSender && d.type === "file") showActiveFile(d.name, d.size);
            if (!isSender) d.type === "text" ? showRecvText(d.content) : showRecvFile(d.name, d.size);
        });
        sse.addEventListener("payload_cleared", () => {
            if (!isSender) { showRecvEmpty(); showToast("[ĐÃ TỰ HỦY]"); }
        });
        sse.onerror = () => {
            fetch(`/${slug}/events?role=${role}&conn=${connId}&check=1`).then(r => {
                if (r.status === 409) { r.json().then(d => showLockout(d.message)); sse.close(); }
            });
        };
    }

    function setPairStatus(paired) {
        const nr = $("nodeReceiver"), ub = $("btnUnpair");
        if (nr) nr.textContent = paired ? "RX ●" : "RX ○";
        if (ub) ub.style.display = (isSender && paired) ? "inline-block" : "none";
    }

    function renderPayload(d) {
        if (isSender) {
            if (d.slot_type === 2) {
                showActiveFile(d.slot_file_name, d.slot_file_size);
            } else if (location.search.includes("mode=file") || location.hash === "#file") {
                showFileDropzone();
            } else {
                if (!typing) $("senderTextarea").value = d.slot_text || "";
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
        updateCheatCode();
    }

    function showActiveFile(name, size) {
        showFileDropzone();
        $("dropzoneBox").style.display = "none";
        $("fileActiveCard").style.display = "flex";
        $("activeFileName").textContent = name;
        $("activeFileSize").textContent = fmtSize(size);
        updateCheatCode();
    }

    function showRecvText(txt) {
        $("recvTextSection").style.display = "flex";
        $("recvFileSection").style.display = "none";
        $("recvTextDisplay").querySelector("code").textContent = txt || "# (trống)";
    }

    function showRecvFile(name, size) {
        $("recvTextSection").style.display = "none";
        $("recvFileSection").style.display = "flex";
        $("recvFileName").textContent = name;
        $("recvFileSize").textContent = fmtSize(size);
        $("btnDownloadFile").href = `/${slug}-r?conn=${connId}`;
        $("btnDownloadFile").download = name;
    }

    function showRecvEmpty() {
        $("recvTextSection").style.display = "flex";
        $("recvFileSection").style.display = "none";
        $("recvTextDisplay").querySelector("code").textContent = "# Chờ dữ liệu từ TX...";
    }

    function fmtSize(b) {
        return b > 1048576 ? (b / 1048576).toFixed(1) + " MB" : Math.max(1, Math.round((b || 0) / 1024)) + " KB";
    }

    function autoSave() {
        if (!isSender || isHome) return;
        $("syncIndicator").textContent = "[LƯU...]";
        typing = true;
        clearTimeout(saveTimer);
        saveTimer = setTimeout(() => {
            fetch(`/${slug}-s?conn=${connId}`, {
                method: "POST",
                headers: { "Content-Type": "text/plain; charset=utf-8" },
                body: $("senderTextarea").value
            }).then(r => {
                $("syncIndicator").textContent = r.ok ? "[ĐÃ LƯU ✓]" : (r.status === 409 ? "[BỊ KHÓA]" : "[LỖI]");
                if (r.status === 409) showLockout("TX đang bị chiếm giữ.");
            }).catch(() => $("syncIndicator").textContent = "[MẤT MẠNG]")
              .finally(() => setTimeout(() => typing = false, 300));
        }, 300);
    }

    function uploadFile(file) {
        if (!file || !isSender || isHome) return;
        const fd = new FormData();
        fd.append("file", file);
        $("syncIndicator").textContent = "[NẠP...]";
        fetch(`/${slug}-s?conn=${connId}`, { method: "POST", body: fd })
            .then(r => {
                if (r.status === 409) { showLockout("TX bị khóa."); throw new Error(); }
                return r.json();
            }).then(d => {
                showActiveFile(d.name, d.size);
                $("syncIndicator").textContent = "[ĐÃ NẠP ✓]";
                showToast(`[ĐÃ NẠP ${d.name}]`);
            }).catch(() => $("syncIndicator").textContent = "[LỖI]");
    }

    function updateHomeQuickLinks(s) {
        const tx = $("quickTxLink"), rx = $("quickRxLink");
        if (tx) { tx.href = `/${s}-s`; tx.textContent = `/${s}-s (TX)`; }
        if (rx) { rx.href = `/${s}-r`; rx.textContent = `/${s}-r (RX)`; }
        const url = `${location.origin}/${s}-r`;
        const gl = $("homeGuideLinux"); if (gl) gl.textContent = `curl -sL ${url} | sh`;
        const gw = $("homeGuideWin"); if (gw) gw.textContent = `irm ${url} | iex`;
        const gf = $("homeGuideFile"); if (gf) gf.textContent = `curl -sLJO ${url}`;
    }

    function detectRole() {
        const path = location.pathname.replace(/^\/|\/$/g, "");
        if (path === "" || location.hash === "#") {
            isHome = true;
            document.title = "WireDrop - Web → Terminal Relay";
            $("homeView").style.display = "flex";
            $("senderView").style.display = $("receiverView").style.display = $("wireBadge").style.display = $("wireStatusCard").style.display = "none";
            if (!$("roomInput").value) $("roomInput").value = defaultSlug;
            updateHomeQuickLinks($("roomInput").value || defaultSlug);
            if (sse) { sse.close(); sse = null; }
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

        updateCheatCode();
        connectSSE();
    }

    // Event Listeners
    $("btnModeText").onclick = () => { showTextEditor(); autoSave(); };
    $("btnModeFile").onclick = () => {
        showFileDropzone();
        if ($("activeFileName").textContent && $("activeFileName").textContent !== "-") {
            $("dropzoneBox").style.display = "none";
            $("fileActiveCard").style.display = "flex";
        } else {
            $("dropzoneBox").style.display = "flex";
            $("fileActiveCard").style.display = "none";
        }
    };
    $("btnRemoveFile").onclick = () => {
        $("activeFileName").textContent = "-";
        $("dropzoneBox").style.display = "flex";
        $("fileActiveCard").style.display = "none";
        showTextEditor();
        autoSave();
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
    fi.onchange = e => { if (e.target.files[0]) uploadFile(e.target.files[0]); };
    dz.ondragover = e => { e.preventDefault(); dz.classList.add("dragover"); };
    dz.ondragleave = () => dz.classList.remove("dragover");
    dz.ondrop = e => { e.preventDefault(); dz.classList.remove("dragover"); if (e.dataTransfer.files[0]) uploadFile(e.dataTransfer.files[0]); };

    $("btnCopyCheat").onclick = () => copyText($("cheatCode").textContent);
    $("btnCopyRecvText").onclick = () => copyText($("recvTextDisplay").querySelector("code").textContent);
    $("btnRetryConn").onclick = connectSSE;
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
        const s = ($("roomInput").value || "").trim().replace(/[^a-zA-Z0-9_-]/g, "") || defaultSlug;
        location.href = `/${s}-${suf}`;
    };
    $("btnOpenSender").onclick = () => openRoom("s");
    $("btnOpenReceiver").onclick = () => openRoom("r");
    $("roomInput").oninput = () => {
        const s = ($("roomInput").value || "").trim().replace(/[^a-zA-Z0-9_-]/g, "") || defaultSlug;
        updateHomeQuickLinks(s);
    };
    $("roomInput").onkeydown = e => { if (e.key === "Enter") openRoom("s"); };

    window.onhashchange = window.onpopstate = detectRole;
    detectRole();
})();
