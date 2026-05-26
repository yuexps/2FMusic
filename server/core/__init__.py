import os
from datetime import timedelta
from flask import Flask, render_template, send_file, url_for, jsonify
from werkzeug.middleware.proxy_fix import ProxyFix

from core.config import app_config
from core.routes.auth import auth_bp
from core.routes.music import music_bp
from core.routes.netease import netease_bp
from core.routes.ws import register_ws


def create_app() -> Flask:
    """Flask 应用程序工厂函数"""
    app = Flask(
        __name__, 
        static_folder=app_config.WWW_DIR, 
        template_folder=app_config.WWW_DIR,
        static_url_path=''
    )
    
    # 负载均衡反向代理支持
    app.wsgi_app = ProxyFix(app.wsgi_app, x_for=1, x_proto=1, x_host=1, x_prefix=1)
    
    # 配置基础属性与会话寿命 (30天永久有效)
    app.config['SEND_FILE_MAX_AGE_DEFAULT'] = 31536000
    app.config['TEMPLATES_AUTO_RELOAD'] = True
    app.secret_key = app_config.SECRET_KEY
    app.permanent_session_lifetime = timedelta(days=30)

    # 1. 注册核心功能 Blueprints
    app.register_blueprint(auth_bp)
    app.register_blueprint(music_bp)
    app.register_blueprint(netease_bp)

    # 2. 初始化并注册 WebSocket 服务
    register_ws(app)



    # 4. 后置钩子：缓存控制与 CORS 头自动补全
    @app.after_request
    def add_cache_control_and_cors(response):
        # 对 HTML 页面禁用强缓存，确保每次都能拉取到最新的静态资源 Hash 指针
        if response.content_type.startswith('text/html'):
            response.headers['Cache-Control'] = 'no-cache, no-store, must-revalidate'
            if not response.headers.get('ETag'):
                etag = hashlib_md5_data(response.data)
                response.headers['ETag'] = f'"{etag}"'
                
        # 允许全域 CORS 请求
        response.headers['Access-Control-Allow-Origin'] = '*'
        response.headers['Access-Control-Allow-Headers'] = 'Content-Type,Authorization'
        response.headers['Access-Control-Allow-Methods'] = 'GET,PUT,POST,DELETE,OPTIONS'
            
        return response

    def hashlib_md5_data(data: bytes) -> str:
        h = hashlib.md5()
        h.update(data)
        return h.hexdigest()
    import hashlib

    # 5. 主页路由
    @app.route('/')
    def index():
        return send_file(os.path.join(app_config.WWW_DIR, 'index.html'))

    return app
