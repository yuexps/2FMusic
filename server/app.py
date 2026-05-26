#!/usr/bin/env python3
# -*- coding: utf-8 -*-
import os
import sys
import argparse
import threading

# 1. 确保【源码模式】下优先加载本地 lib 依赖目录中的模块 (必须最先运行)
if getattr(sys, 'frozen', False):
    BASE_DIR = os.path.dirname(sys.executable)
else:
    BASE_DIR = os.path.dirname(os.path.abspath(__file__))
    sys.path.insert(0, os.path.join(BASE_DIR, 'lib'))

try:
    from core.config import app_config
    from core import create_app
    from core.models.db import init_db
    from core.services.scanner import scan_library_incremental, init_watchdog, clean_temp_part_files
except ImportError as e:
    print(f"错误：初始化核心模块失败。\n详情: {e}")
    try:
        with open(os.path.join(BASE_DIR, 'error_import.log'), 'w', encoding='utf-8') as f:
            f.write(f"错误：初始化核心模块失败。\n详情: {e}\n")
    except Exception:
        pass
    sys.exit(1)

# 2. 命令行参数解析
parser = argparse.ArgumentParser(description='2FMusic Server')
parser.add_argument('--music-library-path', type=str, default=os.environ.get('MUSIC_LIBRARY_PATH'), help='Path to music library')
parser.add_argument('--log-path', type=str, default=os.environ.get('LOG_PATH'), help='Path to log file')
parser.add_argument('--port', type=int, default=os.environ.get('PORT') and int(os.environ.get('PORT')) or None, help='Server port')
parser.add_argument('--unix-socket', type=str, default=os.environ.get('UNIX_SOCKET'), help='Unix Domain Socket path')
parser.add_argument('--base-url', type=str, default=os.environ.get('BASE_URL'), help='Application base URL prefix (e.g. /2fmusic)')
parser.add_argument('--password', type=str, default=os.environ.get('APP_AUTH_PASSWORD') or os.environ.get('APP_PASSWORD'),
                    help='Optional password for web access; leave empty to disable auth')
args = parser.parse_args()

# 3. 初始化全局配置
app_config.init_from_args(args)

# 4. 使用统一日志管理器配置日志输出
from core.utils.logger import LogManager
logger = LogManager.setup(app_config.LOG_FILE)


# 5. 创建 Flask 应用实例
app = create_app()

# 自定义基准子路径 (Base URL)
class PrefixMiddleware:
    def __init__(self, app, prefix=''):
        self.app = app
        self.prefix = '/' + prefix.strip('/') if prefix and prefix.strip('/') else ''

    def __call__(self, environ, start_response):
        import urllib.parse
        # 1. 统一提取纯路径
        path = urllib.parse.urlparse(environ.get('PATH_INFO', '')).path
        if path.startswith('//'):
            path = '/' + path.lstrip('/')
            
        # 2. 合并前缀剥离与 SCRIPT_NAME 赋值逻辑
        if self.prefix:
            if 'HTTP_X_FORWARDED_PREFIX' in environ:
                environ['HTTP_X_FORWARDED_PREFIX'] = self.prefix
            if path.startswith(self.prefix):
                path = path[len(self.prefix):]
                if not path.startswith('/'):
                    path = '/' + path
                environ['SCRIPT_NAME'] = self.prefix
                environ['2FMUSIC_BASE_URL_ACTIVE'] = '1'
            
        environ['PATH_INFO'] = path
        return self.app(environ, start_response)

if app_config.BASE_URL and app_config.BASE_URL != '/':
    try:
        app.wsgi_app = PrefixMiddleware(app.wsgi_app, prefix=app_config.BASE_URL)
    except Exception as e:
        print(f"Error applying Base URL middleware: {e}")


import socket
from werkzeug.serving import BaseWSGIServer
from socketserver import ThreadingMixIn

class UnixWSGIServer(ThreadingMixIn, BaseWSGIServer):
    """自定义 UNIX Domain Socket 监听服务器（支持多线程高并发）"""
    def __init__(self, socket_path, *args, **kwargs):
        self.socket_path = os.path.abspath(socket_path)
        # 清除已有的 socket 文件
        if os.path.exists(self.socket_path):
            try:
                os.remove(self.socket_path)
            except Exception:
                pass
        
        try:
            self.socket_fd = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
            self.socket_fd.bind(self.socket_path)
            # 分配读写权限给 socket 文件，以便 Nginx/Caddy 等代理读取
            os.chmod(self.socket_path, 0o666)
        except Exception as e:
            print(f"致命错误：绑定 UNIX Domain Socket 套接字失败 (路径: {self.socket_path})。\n原因: {e}")
            raise e
            
        self.server_address = self.socket_path
        
        # 启动 BaseWSGIServer（传入 dummy 的 host 和 port）
        super().__init__('127.0.0.1', 0, *args, **kwargs)

    def server_bind(self):
        try:
            self.socket.close()
        except Exception:
            pass
        self.socket = self.socket_fd


if __name__ == '__main__':
    logger.info(f"Music Library Path: {app_config.MUSIC_LIBRARY_PATH}")
    if app_config.BASE_URL:
        logger.info(f"Base URL prefix enabled: {app_config.BASE_URL}")
    
    # 6. 后台异步挂起数据库初始化、增量扫描与文件变化自动同步
    threading.Thread(target=lambda: (init_db(), clean_temp_part_files(), scan_library_incremental()), daemon=True).start()
    threading.Thread(target=init_watchdog, daemon=True).start()
    
    # 智能分发三种启动模式
    if app_config.UNIX_SOCKET and app_config.PORT:
        # 3. 端口+sock 并发模式
        logger.info(f"启动模式：[端口+sock 并发模式] (端口: {app_config.PORT}, UNIX Socket: {app_config.UNIX_SOCKET})")
        try:
            unix_server = UnixWSGIServer(app_config.UNIX_SOCKET, app)
            # 在单独的守护线程里后台开启 UNIX socket 监听
            threading.Thread(target=unix_server.serve_forever, daemon=True).start()
            
            # 主线程阻塞运行 TCP 端口服务
            app.run(host='0.0.0.0', port=app_config.PORT, threaded=True, use_reloader=False)
        except Exception as e:
            logger.exception(f"服务并发启动失败: {e}")
        finally:
            if os.path.exists(app_config.UNIX_SOCKET):
                try:
                    os.remove(app_config.UNIX_SOCKET)
                    logger.info(f"已清理已退出的 UNIX Socket 临时文件: {app_config.UNIX_SOCKET}")
                except Exception:
                    pass
    elif app_config.UNIX_SOCKET:
        # 2. 纯 sock 模式
        logger.info(f"启动模式：[纯 sock 模式] (UNIX Socket: {app_config.UNIX_SOCKET})")
        try:
            unix_server = UnixWSGIServer(app_config.UNIX_SOCKET, app)
            unix_server.serve_forever()
        except Exception as e:
            logger.exception(f"UNIX Socket 服务启动失败: {e}")
        finally:
            if os.path.exists(app_config.UNIX_SOCKET):
                try:
                    os.remove(app_config.UNIX_SOCKET)
                    logger.info(f"已清理已退出的 UNIX Socket 临时文件: {app_config.UNIX_SOCKET}")
                except Exception:
                    pass
    else:
        # 1. 纯端口模式 (向下兼容)
        logger.info(f"启动模式：[纯端口模式] (端口: {app_config.PORT})")
        try:
            app.run(host='0.0.0.0', port=app_config.PORT, threaded=True, use_reloader=False)
        except Exception as e:
            logger.exception(f"端口服务启动失败: {e}")