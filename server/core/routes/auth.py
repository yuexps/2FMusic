import hashlib
import time
from collections import defaultdict
from flask import Blueprint, request, session, redirect, url_for, jsonify, render_template, make_response, current_app
from core.config import app_config
from core.utils.logger import logger

auth_bp = Blueprint('auth', __name__)

def get_safe_redirect_url(target):
    """确保重定向地址包含 BASE_URL 前缀（仅当请求确实来自反向代理子路径时）"""
    if not target:
        target = '/'
    base_url_active = request.environ.get('2FMUSIC_BASE_URL_ACTIVE') == '1'
    if app_config.BASE_URL and app_config.BASE_URL != '/' and target.startswith('/') and base_url_active:
        # 幂等处理：避免重复拼接
        if not target.startswith(app_config.BASE_URL + '/') and target != app_config.BASE_URL:
            target = app_config.BASE_URL + target
    return target

def _auth_failed():
    """认证失败响应，API 请求返回 401，页面请求重定向至登录页"""
    if request.path.startswith('/api/'):
        return jsonify({'success': False, 'error': 'unauthorized'}), 401
    next_url = get_safe_redirect_url(request.path)
    return redirect(url_for('auth.login', next=next_url))

@auth_bp.before_app_request
def require_auth():
    """全局路由请求前置认证拦截钩子"""
    if not app_config.PASSWORD:
        return
        
    path = request.path or ''
    # 豁免静态资源、登录页与图标请求
    if (path.startswith('/assets') or path.startswith('/images') or
            path.startswith('/login') or
            path in ('/ICON.PNG', '/manifest.json') or
            path.endswith('/manifest.json') or
            path.endswith('/manifest.webmanifest') or
            path.endswith('/ICON.PNG') or
            path.endswith('/icon.svg')):
        return

    # 放行 CORS OPTIONS 预检请求
    if request.method == 'OPTIONS':
        return

    # 检查 X-Password 头部认证
    password_header = request.headers.get('X-Password')
    # 检查 URL 参数 auth (常用于不支持 headers 的 audio 播放流等)
    if not password_header:
        password_header = request.args.get('auth')
        
    if password_header:
        stored_hash = hashlib.sha256(app_config.PASSWORD.encode()).hexdigest()
        if password_header == app_config.PASSWORD or password_header.lower() == stored_hash.lower():
            session['authed'] = True
            return
            
    if session.get('authed'):
        return
        
    return _auth_failed()

# 记录 IP 登录失败的时间戳列表
ip_failures = defaultdict(list)

def get_client_ip():
    """获取客户端的真实 IP 地址"""
    if request.headers.getlist("X-Forwarded-For"):
        return request.headers.getlist("X-Forwarded-For")[0].split(',')[0].strip()
    return request.remote_addr

def is_ip_blocked(ip):
    """检查 IP 是否因为最近一小时内失败次数达到或超过三次而被封禁"""
    now = time.time()
    # 仅保留最近 1 小时 (3600 秒) 的失败时间戳
    ip_failures[ip] = [t for t in ip_failures[ip] if now - t < 3600]
    return len(ip_failures[ip]) >= 3

@auth_bp.route('/login', methods=['GET', 'POST'])
def login():
    """登录界面渲染与校验逻辑"""
    if not app_config.PASSWORD:
        # 如果未设置密码，则免登直接重定向回主页
        return redirect(get_safe_redirect_url('/'))
        
    next_path = request.args.get('next') or '/'
    
    if request.method == 'POST':
        ip = get_client_ip()
        logger.info(f"IP {ip} 发起登录尝试")
        if is_ip_blocked(ip):
            session['login_error'] = '尝试次数过多，请一小时后再试'
            logger.warning(f"IP {ip} 登录失败：尝试次数过多")
            return redirect(url_for('auth.login', next=next_path))
            
        pwd = request.form.get('password') or ''
        stored_hash = hashlib.sha256(app_config.PASSWORD.encode()).hexdigest()
        
        # 兼容明文及 SHA256 哈希值比对
        if pwd == app_config.PASSWORD or pwd.lower() == stored_hash.lower():
            session['authed'] = True
            logger.info(f"IP {ip} 登录成功")
            # 登录成功，清除当前 IP 的失败记录
            ip_failures.pop(ip, None)
            if request.form.get('remember'):
                session.permanent = True
            else:
                session.permanent = False
            return redirect(get_safe_redirect_url(next_path))
        else:
            # 登录失败，记录失败时间戳
            ip_failures[ip].append(time.time())
            logger.warning(f"IP {ip} 登录失败：密码错误")
            # 校验失败，将错误存入 session，并重定向 (Redirect) 回登录页
            session['login_error'] = '密码错误'
            return redirect(url_for('auth.login', next=next_path))
            
    # GET 请求：从 session 中提取一次性错误消息（pop 会在提取后自动清除）
    error = session.pop('login_error', None)
    return render_template('login.html', error=error, next_path=next_path)

@auth_bp.route('/logout')
def logout():
    """注销会话并清理 Cookie"""
    session.pop('authed', None)
    session.clear()
    resp = make_response(redirect(url_for('auth.login')))
    
    # 强制清理 Session Cookie
    session_cookie_name = current_app.config.get('SESSION_COOKIE_NAME', 'session')
    resp.delete_cookie(session_cookie_name)
    return resp

