const ws = new WebSocket("ws://" + window.location.hostname + ":9998");

ws.onopen = function() {
    document.getElementById("status-dot").className = "dot green";
    document.getElementById("status-text").innerText = "Daemon Connected";
};

ws.onclose = function() {
    document.getElementById("status-dot").className = "dot red";
    document.getElementById("status-text").innerText = "Disconnected";
};

ws.onmessage = function(event) {
    const msg = JSON.parse(event.data);
    if (msg.type === "status") {
        document.getElementById("active-display-name").innerText = msg.data;
        if (msg.data === "Active") {
            document.querySelector('.metric-circle').style.background = "conic-gradient(#00e676 100%, rgba(255,255,255,0.05) 0%)";
        } else {
            document.querySelector('.metric-circle').style.background = "conic-gradient(var(--accent-color) 0%, rgba(255,255,255,0.05) 0%)";
        }
    }
};

function sendCommand(cmd) {
    if (ws.readyState === WebSocket.OPEN) {
        ws.send(JSON.stringify({ action: "command", cmd: cmd }));
    }
}

document.getElementById('scale-slider').addEventListener('input', function(e) {
    const val = e.target.value;
    document.getElementById('scale-val').innerText = Math.round(val * 100) + "%";
});

document.getElementById('scale-slider').addEventListener('change', function(e) {
    const val = e.target.value;
    if (ws.readyState === WebSocket.OPEN) {
        ws.send(JSON.stringify({ action: "scale", val: val }));
    }
});
