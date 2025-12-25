package webterm

const indexHTMLTemplate = `<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>ac2 Web Terminal</title>
    <link rel="stylesheet" href="/static/xterm.css" />
    <style>
        :root {
            --page-bg: #f5f2ec;
            --page-accent: #e9e4db;
            --panel-bg: #fffdf9;
            --panel-border: #e0d8cc;
            --panel-shadow: 0 16px 40px rgba(36, 32, 25, 0.12);
            --ink-strong: #1f1d1a;
            --ink-muted: #6a635b;
            --terminal-bg: #1e1e1e;
            --terminal-panel: #262424;
            --terminal-focus-border: #3a7bd5;
            --terminal-focus-glow: rgba(58, 123, 213, 0.4);
            --status-ok: #2e7d32;
            --status-warn: #b26a00;
            --status-bad: #b3261e;
            --button-bg: #f3eee7;
            --button-border: #d8cfc2;
            --button-ink: #2b2520;
            --button-active: #efe6db;
        }
        * {
            margin: 0;
            padding: 0;
            box-sizing: border-box;
        }
        body {
            background: radial-gradient(circle at 10% 10%, #fbf7f1, var(--page-bg));
            font-family: "IBM Plex Sans", "Trebuchet MS", "Segoe UI", sans-serif;
            overflow: hidden;
            display: flex;
            flex-direction: column;
            color: var(--ink-strong);
            height: 100vh;
            width: 100vw;
        }
        body::before {
            content: "";
            position: fixed;
            inset: 0;
            background:
                linear-gradient(120deg, rgba(255, 255, 255, 0.6), rgba(233, 228, 219, 0.3)),
                repeating-linear-gradient(
                    120deg,
                    transparent 0,
                    transparent 24px,
                    rgba(31, 29, 26, 0.035) 24px,
                    rgba(31, 29, 26, 0.035) 25px
                );
            pointer-events: none;
            z-index: 0;
        }
        #info-bar {
            position: relative;
            background: var(--panel-bg);
            color: var(--ink-strong);
            padding: 10px 20px;
            font-size: 13px;
            border-bottom: 1px solid var(--panel-border);
            display: flex;
            justify-content: space-between;
            align-items: center;
            flex-shrink: 0;
            box-shadow: 0 4px 12px rgba(33, 28, 23, 0.06);
            z-index: 2;
        }
        #info-bar .title {
            font-weight: 700;
            letter-spacing: 0.02em;
        }
        #info-bar .agent {
            font-weight: 600;
            color: #7a5230;
        }
        #info-bar .info {
            color: var(--ink-muted);
        }
        #terminal-shell {
            flex: 1;
            display: flex;
            flex-direction: column;
            padding: 0;
            margin: 0;
            min-height: 0;
        }
        #terminal-panel {
            flex: 1;
            background: var(--terminal-panel);
            border-radius: 0;
            box-shadow: none;
            border: none;
            overflow: hidden;
            display: flex;
            flex-direction: column;
            min-height: 0;
        }
        #terminal-container {
            flex: 1;
            padding: 4px;
            overflow: auto;
            background: var(--terminal-bg);
            min-height: 0;
            -webkit-overflow-scrolling: touch;
            position: relative;
            border: 1px solid transparent;
            transition: border-color 0.2s ease, box-shadow 0.2s ease;
        }
        #terminal-container:focus-within {
            border-color: var(--terminal-focus-border);
            box-shadow: 0 0 0 1px var(--terminal-focus-border), 0 0 18px var(--terminal-focus-glow);
        }
        #terminal-container .xterm {
            height: 100%;
            min-width: max-content;
        }
        #terminal-container .xterm-viewport {
            overflow-y: scroll !important;
            overflow-x: hidden !important;
            -webkit-overflow-scrolling: touch;
            overscroll-behavior: contain;
            touch-action: pan-y pan-x !important;
            position: relative;
            z-index: 1;
        }
        #terminal-container .xterm-screen {
            touch-action: pan-y pan-x !important;
        }
        #status {
            padding: 4px 8px;
            background: rgba(233, 228, 219, 0.6);
            color: var(--status-ok);
            border-radius: 3px;
            font-size: 12px;
            border: 1px solid rgba(46, 125, 50, 0.5);
        }
        #status.disconnected {
            color: var(--status-bad);
            border-color: rgba(179, 38, 30, 0.5);
        }
        #status.connecting {
            color: var(--status-warn);
            border-color: rgba(178, 106, 0, 0.5);
        }
        #toolbar {
            display: flex;
            gap: 8px;
            align-items: center;
            justify-content: flex-start;
            padding: 8px 12px;
            background: var(--panel-bg);
            border-top: 1px solid var(--panel-border);
            flex-wrap: nowrap;
            overflow-x: auto;
            -webkit-overflow-scrolling: touch;
            flex-shrink: 0;
        }
        .toolbar-button {
            border: 1px solid var(--button-border);
            background: var(--button-bg);
            color: var(--button-ink);
            padding: 4px 10px;
            font-size: 12px;
            border-radius: 4px;
            cursor: pointer;
            transition: transform 0.15s ease, background 0.15s ease;
            white-space: nowrap;
            flex-shrink: 0;
        }
        .toolbar-button:active {
            transform: translateY(1px);
            background: var(--button-active);
        }
        .toolbar-button.primary {
            background: #f1ddc6;
            border-color: #e1c7a5;
        }
        .toolbar-button.active {
            background: #e2d4c2;
            border-color: #c6b39c;
        }
        #mobile-keys {
            display: flex;
            gap: 8px;
            align-items: center;
            flex-shrink: 0;
        }
        @keyframes rise {
            from {
                transform: translateY(8px);
                opacity: 0.7;
            }
            to {
                transform: translateY(0);
                opacity: 1;
            }
        }
        @media (max-width: 720px) {
            #info-bar {
                flex-direction: row;
                align-items: center;
                gap: 8px;
                padding: 8px 12px;
            }
        }
        @media (orientation: portrait) {
            #terminal-shell {
                padding-bottom: 20vh;
            }
        }
        @media (orientation: landscape) {
            #terminal-shell {
                padding-bottom: 20vh;
            }
        }
        @media (max-width: 1024px) {
            .shortcuts {
                display: none;
            }
            #terminal-container {
                overflow: auto;
                -webkit-overflow-scrolling: touch;
            }
            #terminal-container .xterm-viewport {
                border: 1px solid rgba(31, 29, 26, 0.15);
                overflow-y: scroll !important;
                overflow-x: hidden !important;
                -webkit-overflow-scrolling: touch;
            }
            #terminal-container .xterm-screen {
                touch-action: pan-y pan-x !important;
            }
            #terminal-container .xterm-rows {
                touch-action: pan-y pan-x !important;
            }
        }
        /* Fullscreen mode styles */
        body.fullscreen-mode #terminal-shell {
            display: flex;
            flex-direction: column;
            width: 100vw;
            height: 100vh;
            padding: 0;
            margin: 0;
            background: var(--terminal-bg);
        }
        body.fullscreen-mode #terminal-panel {
            flex: 1;
            height: 100% !important;
            max-height: 100% !important;
            min-height: 0;
            border-radius: 0;
            box-shadow: none;
            border: none;
            display: flex;
            flex-direction: column;
        }
        body.fullscreen-mode #terminal-container {
            flex: 1;
            min-height: 0;
            padding: 2px 4px;
            overflow: hidden;
        }
        body.fullscreen-mode #toolbar {
            flex-shrink: 0;
            height: auto;
            background: var(--terminal-panel);
            border-top: 1px solid rgba(255, 255, 255, 0.1);
            padding: 4px 6px;
            display: flex;
            flex-direction: row;
            flex-wrap: nowrap;
            gap: 6px;
            overflow-x: auto;
            overflow-y: hidden;
            -webkit-overflow-scrolling: touch;
            align-items: center;
        }
        body.fullscreen-mode #toolbar .toolbar-button {
            flex-shrink: 0;
            white-space: nowrap;
            font-size: 14px;
            padding: 8px 16px;
            border-radius: 6px;
            line-height: 1.2;
        }
        body.fullscreen-mode #mobile-keys {
            display: flex;
            flex-direction: row;
            flex-shrink: 0;
            gap: 4px;
        }
    </style>
</head>
<body>
    <div id="info-bar">
        <div>
            <span class="title">ac2 Web Terminal</span>
            <span class="info">—</span>
            <span class="agent" id="agent-name">{{AGENT_NAME}}</span>
            <span class="info shortcuts">│ Ctrl+C to interrupt │ Paste with Ctrl+Shift+V</span>
        </div>
        <div id="status" class="connecting">Connecting...</div>
    </div>
    <div id="terminal-shell">
        <div id="terminal-panel">
            <div id="terminal-container"></div>
            <div id="toolbar">
                <button class="toolbar-button primary" id="btn-reconnect">Reconnect</button>
                <button class="toolbar-button" id="btn-clear">Clear</button>
                <button class="toolbar-button" id="btn-page-up">Page Up</button>
                <button class="toolbar-button" id="btn-page-down">Page Down</button>
                <button class="toolbar-button" id="btn-to-bottom">To Bottom</button>
                <button class="toolbar-button" id="btn-fullscreen">Fullscreen</button>
                <button class="toolbar-button" id="btn-up">↑</button>
                <button class="toolbar-button" id="btn-down">↓</button>
                <button class="toolbar-button" id="btn-left">←</button>
                <button class="toolbar-button" id="btn-right">→</button>
                <div id="mobile-keys">
                    <button class="toolbar-button" id="btn-esc">Esc</button>
                    <button class="toolbar-button" id="btn-tab">Tab</button>
                    <button class="toolbar-button" id="btn-enter">Enter</button>
                    <button class="toolbar-button" id="btn-ctrl">Ctrl</button>
                    <button class="toolbar-button" id="btn-shift">Shift</button>
                </div>
            </div>
        </div>
    </div>

    <script src="/static/xterm.js"></script>
    <script>
        const AGENT_NAME = "{{AGENT_NAME_JS}}";

        // Workaround for UMD module loading issue
        // Temporarily remove exports if it exists to force global export
        const _exports = typeof exports !== 'undefined' ? exports : undefined;
        if (typeof exports !== 'undefined') {
            exports = undefined;
        }
    </script>
    <script src="/static/addon-fit.js"></script>
    <script>
        // Restore exports if it was removed
        if (typeof _exports !== 'undefined') {
            exports = _exports;
        }

        // The UMD wrapper exports an object with FitAddon property, not the class directly
        const FitAddonConstructor = (window.FitAddon && window.FitAddon.FitAddon) || window.FitAddon;

        const term = new window.Terminal({
            cursorBlink: true,
            fontSize: 14,
            fontFamily: 'Courier New, monospace',
            allowProposedApi: true,
            scrollback: 10000,
            scrollSensitivity: 2,
            smoothScrollDuration: 0,
            fastScrollModifier: 'shift',
            fastScrollSensitivity: 5,
            scrollOnUserInput: true,
            theme: {
                background: '#1e1e1e',
                foreground: '#d4d4d4',
                cursor: '#d4d4d4',
                cursorAccent: '#1e1e1e',
                selection: 'rgba(255, 255, 255, 0.3)',
                black: '#000000',
                red: '#cd3131',
                green: '#0dbc79',
                yellow: '#e5e510',
                blue: '#2472c8',
                magenta: '#bc3fbc',
                cyan: '#11a8cd',
                white: '#e5e5e5',
                brightBlack: '#666666',
                brightRed: '#f14c4c',
                brightGreen: '#23d18b',
                brightYellow: '#f5f543',
                brightBlue: '#3b8eea',
                brightMagenta: '#d670d6',
                brightCyan: '#29b8db',
                brightWhite: '#e5e5e5'
            }
        });

        const fitAddon = new FitAddonConstructor();
        term.loadAddon(fitAddon);

        term.open(document.getElementById('terminal-container'));

        // Enable touch scrolling on mobile devices
        const terminalContainer = document.getElementById('terminal-container');
        if (terminalContainer) {
            // Ensure touch events can scroll the viewport
            const viewport = terminalContainer.querySelector('.xterm-viewport');
            if (viewport) {
                // Prevent xterm from interfering with touch scroll
                viewport.addEventListener('touchstart', function(e) {
                    // Allow native scrolling behavior
                }, { passive: true });

                viewport.addEventListener('touchmove', function(e) {
                    // Allow native scrolling behavior
                }, { passive: true });
            }
        }

        // Smart fit: use fixed columns on portrait mobile to enable horizontal scroll
        function smartFit() {
            const isPortrait = window.matchMedia('(orientation: portrait)').matches;
            const isMobile = window.matchMedia('(max-width: 768px)').matches;

            if (isPortrait && isMobile) {
                // Use fixed columns on portrait mobile to preserve layout
                const container = document.getElementById('terminal-container');
                const rows = Math.floor((container.clientHeight - 8) / 17); // Approximate row height
                term.resize(120, rows > 0 ? rows : 24); // Fixed 120 columns
            } else {
                // Use auto-fit on desktop and landscape mobile
                fitAddon.fit();
            }
        }

        smartFit();

        const status = document.getElementById('status');
        const agentName = document.getElementById('agent-name');
        if (agentName && AGENT_NAME) {
            agentName.textContent = AGENT_NAME;
        }
        let ws = null;
        let reconnectAttempts = 0;
        const maxReconnectAttempts = 5;
        let allowReconnect = true;
        let ctrlActive = false;
        let shiftActive = false;

        function connect() {
            allowReconnect = true;
            const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
            ws = new WebSocket(protocol + '//' + window.location.host + '/ws');

            ws.onopen = () => {
                status.textContent = 'Connected';
                status.className = '';
                reconnectAttempts = 0;

                // Send initial terminal size
                ws.send(JSON.stringify({
                    type: 'resize',
                    rows: term.rows,
                    cols: term.cols
                }));
            };

            ws.onclose = (event) => {
                if (event && event.code === 4001) {
                    allowReconnect = false;
                    status.textContent = event.reason || 'Disconnected by server';
                    status.className = 'disconnected';
                    return;
                }

                status.textContent = 'Disconnected';
                status.className = 'disconnected';

                // Attempt to reconnect
                if (allowReconnect && reconnectAttempts < maxReconnectAttempts) {
                    reconnectAttempts++;
                    status.textContent = 'Reconnecting... (' + reconnectAttempts + '/' + maxReconnectAttempts + ')';
                    status.className = 'connecting';
                    setTimeout(connect, Math.min(1000 * reconnectAttempts, 5000));
                }
            };

            ws.onerror = (err) => {
                console.error('WebSocket error:', err);
                status.textContent = 'Error';
                status.className = 'disconnected';
            };

            ws.onmessage = (event) => {
                try {
                    const msg = JSON.parse(event.data);

                    if (msg.type === 'data') {
                        // Decode base64 to Uint8Array for proper binary handling
                        const binaryString = atob(msg.data);
                        const bytes = new Uint8Array(binaryString.length);
                        for (let i = 0; i < binaryString.length; i++) {
                            bytes[i] = binaryString.charCodeAt(i);
                        }
                        term.write(bytes);
                    } else if (msg.type === 'reset') {
                        term.reset();
                    } else if (msg.type === 'agent') {
                        if (agentName) {
                            agentName.textContent = msg.data || 'Unknown';
                        }
                    } else if (msg.type === 'disconnect') {
                        allowReconnect = false;
                        status.textContent = msg.data || 'Disconnected by server';
                        status.className = 'disconnected';
                        if (ws) {
                            ws.close();
                        }
                    } else if (msg.type === 'ping') {
                        ws.send(JSON.stringify({type: 'pong'}));
                    }
                } catch (e) {
                    console.error('Message parse error:', e);
                }
            };
        }

        function manualReconnect() {
            reconnectAttempts = 0;
            if (ws && ws.readyState !== WebSocket.CLOSED) {
                ws.close();
            }
            connect();
        }

        function manualDisconnect() {
            allowReconnect = false;
            if (ws && ws.readyState !== WebSocket.CLOSED) {
                ws.close();
            }
            status.textContent = 'Disconnected';
            status.className = 'disconnected';
        }

        async function toggleFullscreen() {
            const terminalShell = document.getElementById('terminal-shell');
            const fullscreenBtn = document.getElementById('btn-fullscreen');

            try {
                if (!document.fullscreenElement) {
                    // Enter fullscreen
                    await terminalShell.requestFullscreen();
                    if (screen.orientation && screen.orientation.lock) {
                        try {
                            await screen.orientation.lock('landscape');
                        } catch (e) {
                            console.log('Orientation lock failed:', e);
                        }
                    }
                    fullscreenBtn.textContent = 'Exit Fullscreen';
                } else {
                    // Exit fullscreen
                    await document.exitFullscreen();
                    if (screen.orientation && screen.orientation.unlock) {
                        screen.orientation.unlock();
                    }
                    fullscreenBtn.textContent = 'Fullscreen';
                }
            } catch (err) {
                console.error('Fullscreen error:', err);
            }
        }

        // Listen for fullscreen changes
        document.addEventListener('fullscreenchange', () => {
            const fullscreenBtn = document.getElementById('btn-fullscreen');
            if (document.fullscreenElement) {
                document.body.classList.add('fullscreen-mode');
                fullscreenBtn.textContent = 'Exit Fullscreen';
            } else {
                document.body.classList.remove('fullscreen-mode');
                fullscreenBtn.textContent = 'Fullscreen';
            }
            setTimeout(() => smartFit(), 100);
        });

        function applyModifiers(data) {
            let result = data;
            if (shiftActive) {
                result = result.replace(/[a-z]/g, (match) => match.toUpperCase());
            }
            if (ctrlActive && result.length === 1) {
                const ch = result;
                let code = null;
                if (ch >= 'A' && ch <= 'Z') {
                    code = ch.charCodeAt(0) - 64;
                } else if (ch >= 'a' && ch <= 'z') {
                    code = ch.toUpperCase().charCodeAt(0) - 64;
                } else if (ch === '@') {
                    code = 0;
                } else if (ch === '[') {
                    code = 27;
                } else if (ch === '\\') {
                    code = 28;
                } else if (ch === ']') {
                    code = 29;
                } else if (ch === '^') {
                    code = 30;
                } else if (ch === '_') {
                    code = 31;
                } else if (ch === '?') {
                    code = 127;
                }
                if (code !== null) {
                    result = String.fromCharCode(code);
                }
            }

            if (ctrlActive || shiftActive) {
                ctrlActive = false;
                shiftActive = false;
                updateModifierButtons();
            }
            return result;
        }

        function updateModifierButtons() {
            const ctrlButton = document.getElementById('btn-ctrl');
            const shiftButton = document.getElementById('btn-shift');
            if (ctrlButton) {
                ctrlButton.classList.toggle('active', ctrlActive);
            }
            if (shiftButton) {
                shiftButton.classList.toggle('active', shiftActive);
            }
        }

        function sendData(data) {
            if (ws && ws.readyState === WebSocket.OPEN) {
                const encoder = new TextEncoder();
                const bytes = encoder.encode(data);
                let binaryString = '';
                for (let i = 0; i < bytes.length; i++) {
                    binaryString += String.fromCharCode(bytes[i]);
                }
                ws.send(JSON.stringify({
                    type: 'data',
                    data: btoa(binaryString)
                }));
            }
        }

        term.onData((data) => {
            const modified = applyModifiers(data);
            sendData(modified);
        });

        term.onResize(({rows, cols}) => {
            if (ws && ws.readyState === WebSocket.OPEN) {
                ws.send(JSON.stringify({
                    type: 'resize',
                    rows: rows,
                    cols: cols
                }));
            }
        });

        window.addEventListener('resize', () => {
            smartFit();
        });

        // Listen for orientation changes on mobile
        window.addEventListener('orientationchange', () => {
            setTimeout(() => smartFit(), 200);
        });

        document.getElementById('btn-reconnect').addEventListener('click', manualReconnect);
        document.getElementById('btn-clear').addEventListener('click', () => term.clear());
        document.getElementById('btn-page-up').addEventListener('click', () => term.scrollPages(-1));
        document.getElementById('btn-page-down').addEventListener('click', () => term.scrollPages(1));
        document.getElementById('btn-to-bottom').addEventListener('click', () => term.scrollToBottom());
        document.getElementById('btn-fullscreen').addEventListener('click', toggleFullscreen);
        document.getElementById('btn-up').addEventListener('click', () => sendData('\x1b[A'));
        document.getElementById('btn-down').addEventListener('click', () => sendData('\x1b[B'));
        document.getElementById('btn-left').addEventListener('click', () => sendData('\x1b[D'));
        document.getElementById('btn-right').addEventListener('click', () => sendData('\x1b[C'));
        document.getElementById('btn-esc').addEventListener('click', () => sendData('\x1b'));
        document.getElementById('btn-tab').addEventListener('click', () => sendData('\t'));
        document.getElementById('btn-enter').addEventListener('click', () => sendData('\r'));
        document.getElementById('btn-ctrl').addEventListener('click', () => {
            ctrlActive = !ctrlActive;
            updateModifierButtons();
        });
        document.getElementById('btn-shift').addEventListener('click', () => {
            shiftActive = !shiftActive;
            updateModifierButtons();
        });

        // Start connection
        connect();

        // Focus terminal on load
        term.focus();
    </script>
</body>
</html>
`
