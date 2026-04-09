import json
import urllib.request
import os
from http.server import HTTPServer, BaseHTTPRequestHandler

HOST = "0.0.0.0"
PORT = 3004

class StreamHandler(BaseHTTPRequestHandler):
    def log_message(self, format, *args):
        pass

    def do_POST(self):
        if self.path != "/stream":
            self.send_response(404)
            self.end_headers()
            return
            
        length = int(self.headers.get("Content-Length", 0))
        body = self.rfile.read(length)
        
        try:
            data = json.loads(body)
        except:
            self.send_response(400)
            self.end_headers()
            return

        backend = data.get("backend", "ollama")
        prompt = data.get("prompt", "")

        self.send_response(200)
        self.send_header("Content-Type", "text/event-stream")
        self.send_header("Cache-Control", "no-cache")
        self.send_header("Connection", "keep-alive")
        self.end_headers()
        
        try:
            if backend == "ollama":
                self._stream_ollama(prompt)
            elif backend == "gemini":
                self._stream_gemini(prompt)
            elif backend == "openai":
                self._stream_openai(prompt)
        except Exception as e:
            self.wfile.write(f"data: {json.dumps({'error': str(e)})}\n\n".encode())
            
    def _stream_ollama(self, prompt):
        url = os.environ.get("OLLAMA_URL", "http://localhost:11434") + "/api/generate"
        payload = {"model": os.environ.get("OLLAMA_MODEL", "qwen2.5:0.5b"), "prompt": prompt, "stream": True}
        req = urllib.request.Request(url, data=json.dumps(payload).encode(), method="POST")
        req.add_header('Content-Type', 'application/json')
        
        with urllib.request.urlopen(req) as response:
            for line in response:
                if line.strip():
                    try:
                        chunk = json.loads(line)
                        if "response" in chunk:
                            self.wfile.write(f"data: {json.dumps({'chunk': chunk['response']})}\n\n".encode())
                            self.wfile.flush()
                    except:
                        pass
                        
    def _stream_gemini(self, prompt):
        key = os.environ.get("GEMINI_API_KEY", "")
        if not key: raise Exception("GEMINI_API_KEY no set")
        url = f"https://generativelanguage.googleapis.com/v1beta/models/gemini-2.0-flash:streamGenerateContent?alt=sse&key={key}"
        payload = {"contents": [{"parts": [{"text": prompt}]}]}
        req = urllib.request.Request(url, data=json.dumps(payload).encode(), method="POST")
        req.add_header('Content-Type', 'application/json')
        
        with urllib.request.urlopen(req) as response:
            for line in response:
                line_str = line.decode('utf-8').strip()
                if line_str.startswith('data: '):
                    data_str = line_str[6:]
                    try:
                        chunk = json.loads(data_str)
                        text = chunk.get("candidates", [{}])[0].get("content", {}).get("parts", [{}])[0].get("text", "")
                        if text:
                            self.wfile.write(f"data: {json.dumps({'chunk': text})}\n\n".encode())
                            self.wfile.flush()
                    except:
                        pass

    def _stream_openai(self, prompt):
        key = os.environ.get("OPENAI_API_KEY", "")
        if not key: raise Exception("OPENAI_API_KEY not set")
        url = "https://api.openai.com/v1/chat/completions"
        payload = {"model": "gpt-4o-mini", "messages": [{"role": "user", "content": prompt}], "stream": True}
        req = urllib.request.Request(url, data=json.dumps(payload).encode(), method="POST")
        req.add_header('Content-Type', 'application/json')
        req.add_header('Authorization', f'Bearer {key}')
        
        with urllib.request.urlopen(req) as response:
            for line in response:
                line_str = line.decode('utf-8').strip()
                if line_str.startswith('data: ') and not '[DONE]' in line_str:
                    data_str = line_str[6:]
                    try:
                        chunk = json.loads(data_str)
                        text = chunk.get("choices", [{}])[0].get("delta", {}).get("content", "")
                        if text:
                            self.wfile.write(f"data: {json.dumps({'chunk': text})}\n\n".encode())
                            self.wfile.flush()
                    except:
                        pass

if __name__ == "__main__":
    print(f"Starting Python LLM Streaming Service on port {PORT}")
    server = HTTPServer((HOST, PORT), StreamHandler)
    server.serve_forever()
