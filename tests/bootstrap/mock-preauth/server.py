"""Minimal mock preauth server for integration tests.

Stores preauth entries in memory. The test creates entries via POST /entry,
and the API service fetches them via GET /<id>.

Endpoints:
  POST /entry  - Create preauth entry (called by test setup)
    Body: {"id": "...", "token_hash": "...", "is_active": true, "active_until": "...", "tier": "..."}
    Returns: 201

  GET /<id>    - Get preauth entry (called by API during registration)
    Returns: 200 with preauth JSON, or 404
"""

import json
from http.server import HTTPServer, BaseHTTPRequestHandler

entries: dict[str, dict] = {}


class Handler(BaseHTTPRequestHandler):
    def do_POST(self):
        if self.path == "/entry":
            length = int(self.headers.get("Content-Length", 0))
            body = json.loads(self.rfile.read(length))
            entries[body["id"]] = body
            self.send_response(201)
            self.end_headers()
            return
        self.send_response(404)
        self.end_headers()

    def do_GET(self):
        if self.path == "/health":
            self.send_response(200)
            self.end_headers()
            return
        entry_id = self.path.lstrip("/")
        if entry_id in entries:
            self.send_response(200)
            self.send_header("Content-Type", "application/json")
            self.end_headers()
            self.wfile.write(json.dumps(entries[entry_id]).encode())
            return
        self.send_response(404)
        self.end_headers()

    def log_message(self, format, *args):
        pass  # suppress logs


if __name__ == "__main__":
    server = HTTPServer(("0.0.0.0", 8080), Handler)
    print("Mock preauth server listening on :8080")
    server.serve_forever()
