#!/usr/bin/env python3
"""
Mock A2A Server for Testing

A simple HTTP server that implements the A2A protocol for testing purposes.
Exposes an Agent Card and responds to A2A JSON-RPC requests.

Usage:
    python3 examples/mock-a2a-server.py [--port PORT]

Default port: 9999
"""

import json
import uuid
import argparse
from http.server import HTTPServer, BaseHTTPRequestHandler
from datetime import datetime

# Agent Card for this mock server
AGENT_CARD = {
    "name": "Mock Test Agent",
    "description": "A mock A2A agent for testing AgentLab integration",
    "url": "http://localhost:9999",
    "version": "1.0.0",
    "provider": {
        "organization": "AgentLab Test"
    },
    "capabilities": {
        "streaming": False,
        "pushNotifications": False
    },
    "skills": [
        {
            "id": "echo",
            "name": "Echo",
            "description": "Echoes back any message sent to it"
        },
        {
            "id": "greeting",
            "name": "Greeting",
            "description": "Responds with a friendly greeting"
        }
    ],
    "defaultInputModes": ["text"],
    "defaultOutputModes": ["text"]
}

# In-memory task storage
tasks = {}


class A2AHandler(BaseHTTPRequestHandler):
    """HTTP request handler implementing A2A protocol."""

    def log_message(self, format, *args):
        """Custom log format."""
        print(f"[{datetime.now().isoformat()}] {args[0]}")

    def send_json(self, data, status=200):
        """Send JSON response."""
        body = json.dumps(data, indent=2).encode('utf-8')
        self.send_response(status)
        self.send_header('Content-Type', 'application/json')
        self.send_header('Content-Length', len(body))
        self.send_header('Access-Control-Allow-Origin', '*')
        self.end_headers()
        self.wfile.write(body)

    def do_OPTIONS(self):
        """Handle CORS preflight."""
        self.send_response(200)
        self.send_header('Access-Control-Allow-Origin', '*')
        self.send_header('Access-Control-Allow-Methods', 'GET, POST, OPTIONS')
        self.send_header('Access-Control-Allow-Headers', 'Content-Type, Authorization')
        self.end_headers()

    def do_GET(self):
        """Handle GET requests - Agent Card discovery."""
        if self.path == '/.well-known/agent.json':
            print(f"  -> Serving Agent Card")
            self.send_json(AGENT_CARD)
        else:
            self.send_json({"error": "Not found"}, 404)

    def do_POST(self):
        """Handle POST requests - A2A JSON-RPC."""
        content_length = int(self.headers.get('Content-Length', 0))
        body = self.rfile.read(content_length)

        try:
            request = json.loads(body)
        except json.JSONDecodeError:
            self.send_json({
                "jsonrpc": "2.0",
                "error": {"code": -32700, "message": "Parse error"},
                "id": None
            }, 400)
            return

        jsonrpc = request.get('jsonrpc')
        method = request.get('method')
        params = request.get('params', {})
        req_id = request.get('id')

        print(f"  -> Method: {method}, ID: {req_id}")

        if jsonrpc != '2.0':
            self.send_json({
                "jsonrpc": "2.0",
                "error": {"code": -32600, "message": "Invalid Request"},
                "id": req_id
            }, 400)
            return

        # Route to method handlers
        if method == 'message/send':
            self.handle_message_send(params, req_id)
        elif method == 'tasks/get':
            self.handle_tasks_get(params, req_id)
        elif method == 'tasks/list':
            self.handle_tasks_list(params, req_id)
        elif method == 'tasks/cancel':
            self.handle_tasks_cancel(params, req_id)
        else:
            self.send_json({
                "jsonrpc": "2.0",
                "error": {"code": -32601, "message": f"Method not found: {method}"},
                "id": req_id
            }, 400)

    def handle_message_send(self, params, req_id):
        """Handle message/send - create a task and respond."""
        message = params.get('message', {})
        parts = message.get('parts', [])

        # Extract text from message
        text = ""
        for part in parts:
            if 'text' in part:
                text += part['text']

        # Create task
        task_id = str(uuid.uuid4())

        # Generate response based on content
        if 'hello' in text.lower() or 'hi' in text.lower():
            response_text = "Hello! I'm the Mock Test Agent. How can I help you today?"
        elif 'help' in text.lower():
            response_text = "I'm a mock A2A agent for testing. I can echo messages and respond to greetings. Try saying 'hello' or ask me to 'echo' something!"
        elif 'echo' in text.lower():
            response_text = f"Echo: {text}"
        else:
            response_text = f"I received your message: '{text}'. I'm a mock agent for testing A2A protocol integration."

        task = {
            "id": task_id,
            "state": "completed",
            "messages": [
                message,
                {
                    "role": "agent",
                    "parts": [{"text": response_text}]
                }
            ]
        }
        tasks[task_id] = task

        self.send_json({
            "jsonrpc": "2.0",
            "result": task,
            "id": req_id
        })

    def handle_tasks_get(self, params, req_id):
        """Handle tasks/get - retrieve a task by ID."""
        task_id = params.get('taskId')

        if task_id in tasks:
            self.send_json({
                "jsonrpc": "2.0",
                "result": tasks[task_id],
                "id": req_id
            })
        else:
            self.send_json({
                "jsonrpc": "2.0",
                "error": {"code": -32000, "message": f"Task not found: {task_id}"},
                "id": req_id
            }, 404)

    def handle_tasks_list(self, params, req_id):
        """Handle tasks/list - list all tasks."""
        self.send_json({
            "jsonrpc": "2.0",
            "result": {"tasks": list(tasks.values())},
            "id": req_id
        })

    def handle_tasks_cancel(self, params, req_id):
        """Handle tasks/cancel - cancel a task."""
        task_id = params.get('taskId')

        if task_id in tasks:
            tasks[task_id]['state'] = 'cancelled'
            self.send_json({
                "jsonrpc": "2.0",
                "result": tasks[task_id],
                "id": req_id
            })
        else:
            self.send_json({
                "jsonrpc": "2.0",
                "error": {"code": -32000, "message": f"Task not found: {task_id}"},
                "id": req_id
            }, 404)


def main():
    parser = argparse.ArgumentParser(description='Mock A2A Server for Testing')
    parser.add_argument('--port', type=int, default=9999, help='Port to listen on (default: 9999)')
    args = parser.parse_args()

    server = HTTPServer(('0.0.0.0', args.port), A2AHandler)
    print(f"""
╔══════════════════════════════════════════════════════════════╗
║                    Mock A2A Server                           ║
╠══════════════════════════════════════════════════════════════╣
║  Listening on: http://localhost:{args.port:<5}                       ║
║  Agent Card:   http://localhost:{args.port}/.well-known/agent.json  ║
╠══════════════════════════════════════════════════════════════╣
║  Skills:                                                     ║
║    - echo: Echoes back messages                              ║
║    - greeting: Responds with greetings                       ║
╠══════════════════════════════════════════════════════════════╣
║  Press Ctrl+C to stop                                        ║
╚══════════════════════════════════════════════════════════════╝
""")

    try:
        server.serve_forever()
    except KeyboardInterrupt:
        print("\nShutting down...")
        server.shutdown()


if __name__ == '__main__':
    main()
