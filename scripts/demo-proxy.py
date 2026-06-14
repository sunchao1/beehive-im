#!/usr/bin/env python3
"""静态托管 demo/web，并将 /im/* 代理到 usrsvr（解决 8088→8000 跨域）。"""
import http.server
import os
import socket
import sys
import urllib.error
import urllib.request

USRSVR = os.environ.get("BEEHIVE_USRSVR_URL", "http://127.0.0.1:8000")
PORT = int(os.environ.get("DEMO_WEB_PORT", "8088"))
WEB_ROOT = os.path.join(os.path.dirname(__file__), "..", "demo", "web")


class ReuseHTTPServer(http.server.ThreadingHTTPServer):
    allow_reuse_address = True


class DemoHandler(http.server.SimpleHTTPRequestHandler):
    def __init__(self, *args, **kwargs):
        super().__init__(*args, directory=WEB_ROOT, **kwargs)

    def end_headers(self):
        self.send_header("Access-Control-Allow-Origin", "*")
        self.send_header("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
        self.send_header("Access-Control-Allow-Headers", "*")
        super().end_headers()

    def do_OPTIONS(self):
        self.send_response(204)
        self.end_headers()

    def do_GET(self):
        if self.path.startswith("/im/"):
            self._proxy_api()
            return
        super().do_GET()

    def _proxy_api(self):
        url = USRSVR + self.path
        try:
            with urllib.request.urlopen(url, timeout=10) as resp:
                body = resp.read()
                self.send_response(resp.status)
                ctype = resp.headers.get("Content-Type", "application/json")
                self.send_header("Content-Type", ctype)
                self.end_headers()
                self.wfile.write(body)
        except urllib.error.HTTPError as e:
            body = e.read()
            self.send_response(e.code)
            self.send_header("Content-Type", e.headers.get("Content-Type", "text/plain"))
            self.end_headers()
            self.wfile.write(body)
        except urllib.error.URLError as e:
            msg = f'{{"code":1,"errmsg":"usrsvr unreachable: {e.reason}"}}'.encode()
            self.send_response(502)
            self.send_header("Content-Type", "application/json")
            self.end_headers()
            self.wfile.write(msg)


if __name__ == "__main__":
    os.chdir(WEB_ROOT)
    try:
        httpd = ReuseHTTPServer(("127.0.0.1", PORT), DemoHandler)
    except OSError as e:
        if e.errno in (48, 98):  # macOS EADDRINUSE / Linux EADDRINUSE
            print(f"[serve-demo] ERROR: port {PORT} already in use.", file=sys.stderr)
            print(f"  Stop old server: kill $(lsof -ti :{PORT})", file=sys.stderr)
            print(f"  Or use another port: DEMO_WEB_PORT=8089 ./scripts/serve-demo.sh", file=sys.stderr)
            sys.exit(1)
        raise
    with httpd:
        print(f"Serving demo/web at http://127.0.0.1:{PORT}/")
        print(f"API proxy: /im/* -> {USRSVR}")
        try:
            httpd.serve_forever()
        except KeyboardInterrupt:
            print("\nStopped.")
            sys.exit(0)
