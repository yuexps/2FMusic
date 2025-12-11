import http.server
import socketserver
import urllib.request
import urllib.error
import sys
import os

# 配置
PORT = 23235
TARGET_BASE = "http://127.0.0.1:23237"

class NoRedirectHandler(urllib.request.HTTPRedirectHandler):
    def http_error_302(self, req, fp, code, msg, headers):
        return fp
    http_error_301 = http_error_303 = http_error_307 = http_error_302

class ProxyHandler(http.server.BaseHTTPRequestHandler):
    def do_GET(self): self.handle_request()
    def do_HEAD(self): self.handle_request()
    def do_POST(self): self.handle_request()
    def do_PUT(self): self.handle_request()
    def do_DELETE(self): self.handle_request()
    def do_OPTIONS(self): self.handle_request()

    def handle_request(self):
        url = f"{TARGET_BASE}{self.path}"
        try:
            # 读取请求体 (如果有)
            data = None
            content_length = int(self.headers.get('Content-Length', 0))
            if content_length > 0:
                data = self.rfile.read(content_length)

            # 使用原始 Header 创建请求
            req = urllib.request.Request(url, method=self.command, data=data)
            for header, value in self.headers.items():
                if header.lower() == 'host':
                     # 如果存在，保留原始 Host Header (适配 Lucky)
                     req.add_header('Host', value)
                elif header.lower() == 'content-length':
                     # urllib 会自动计算 content-length，避免重复或冲突
                     continue 
                else:
                     req.add_header(header, value)

            # 向目标发起请求，禁用自动重定向
            opener = urllib.request.build_opener(NoRedirectHandler)
            with opener.open(req) as response:
                self.send_response(response.status)
                
                # 转发 Header
                for header, value in response.getheaders():
                    # 跳过逐跳 (hop-by-hop) Header
                    if header.lower() not in ['transfer-encoding', 'connection', 'keep-alive', 'proxy-authenticate', 'proxy-authorization', 'te', 'trailers', 'upgrade']: 
                        self.send_header(header, value)
                self.end_headers()

                # 流式传输响应体 (非 HEAD 请求)
                if self.command != 'HEAD':
                     while True:
                        chunk = response.read(8192)
                        if not chunk:
                            break
                        self.wfile.write(chunk)

        except urllib.error.HTTPError as e:
            self.send_response(e.code)
            # 同时转发错误响应的 Header (例如 302 中的 Location)
            for header, value in e.headers.items():
                 if header.lower() not in ['transfer-encoding', 'connection']:
                      self.send_header(header, value)
            self.end_headers()
            if e.fp:
                self.wfile.write(e.fp.read())
        except (ConnectionRefusedError, urllib.error.URLError):
            self.send_response(502)
            self.send_header('Content-Type', 'text/html; charset=utf-8')
            self.end_headers()
            html = """
            <!DOCTYPE html>
            <html lang="zh-CN">
            <head>
                <meta charset="UTF-8">
                <meta name="viewport" content="width=device-width, initial-scale=1.0">
                <title>服务未启动</title>
                <style>
                    body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif; display: flex; align-items: center; justify-content: center; height: 100vh; margin: 0; background-color: #f5f5f7; color: #333; }
                    .container { text-align: center; padding: 2rem; background: white; border-radius: 12px; box-shadow: 0 4px 6px rgba(0,0,0,0.1); max-width: 400px; width: 90%; }
                    h1 { font-size: 1.5rem; margin-bottom: 1rem; color: #d32f2f; }
                    p { margin-bottom: 1.5rem; line-height: 1.6; color: #666; }
                    .btn { display: inline-block; padding: 0.6rem 1.2rem; background-color: #007aff; color: white; text-decoration: none; border-radius: 6px; font-weight: 500; transition: background-color 0.2s; }
                    .btn:hover { background-color: #0056b3; }
                </style>
            </head>
            <body>
                <div class="container">
                    <h1>主应用未启动</h1>
                    <p>无法连接到 2FMusic 主程序。<br>请确保您已安装并启动了 <a href="https://github.com/yuexps/2FMusic" target="_blank" style="color: #007aff; text-decoration: none; font-weight: bold;">2FMusic</a> 主应用。</p>
                    <a href="javascript:location.reload()" class="btn">重 试</a>
                </div>
            </body>
            </html>
            """
            self.wfile.write(html.encode('utf-8'))
        except Exception as e:
            self.send_error(500, "Internal Server Error", str(e))

    def log_message(self, format, *args):
        # 禁用控制台日志以避免干扰，除非必要
        return

if __name__ == "__main__":
    # 确保端口处理正确
    try:
        if os.environ.get('PORT'):
            PORT = int(os.environ.get('PORT'))
    except:
        pass

    with socketserver.ThreadingTCPServer(("", PORT), ProxyHandler) as httpd:
        print(f"Proxy serving at port {PORT} -> {TARGET_BASE}")
        httpd.serve_forever()
