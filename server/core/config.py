import os
import sys

class AppConfig:
    """应用程序配置管理类"""
    def __init__(self):
        # 基础目录计算
        if getattr(sys, 'frozen', False):
            # 打包模式下的可执行文件目录
            self.BASE_DIR = os.path.dirname(sys.executable)
        else:
            # 源码开发模式下的目录
            self.BASE_DIR = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
            
        self.WWW_DIR = os.path.abspath(os.path.join(self.BASE_DIR, '../www'))
        
        # 由启动参数或环境变量指定的项
        self.MUSIC_LIBRARY_PATH = ""
        self.LYRICS_DIR = ""
        self.COVERS_DIR = ""
        self.DB_PATH = ""
        self.LOG_FILE = ""
        self.PASSWORD = None
        self.PORT = 23237
        self.UNIX_SOCKET = None
        self.BASE_URL = ""
        
        # 秘钥与会话设置
        self.SECRET_KEY = os.environ.get('APP_SECRET_KEY', '2fmusic_secret')
        
        # 网易云相关设置 (默认值，加载配置后可能会被数据库设置覆盖)
        self.NETEASE_DOWNLOAD_DIR = ""
        self.NETEASE_API_BASE_DEFAULT = os.environ.get('NETEASE_API_BASE', 'http://localhost:23236')
        self.NETEASE_API_BASE = None
        self.NETEASE_COOKIE = None
        self.NETEASE_MAX_CONCURRENT = 5
        self.NETEASE_QUALITY_DEFAULT = 'exhigh'

    def init_from_args(self, args):
        """从命令行参数或环境变量初始化配置"""
        # 若命令行参数与环境变量皆未指定启动端口和 socket，默认端口 23237
        if not args.port and not args.unix_socket:
            self.PORT = 23237
            self.UNIX_SOCKET = None
        else:
            self.PORT = args.port
            self.UNIX_SOCKET = args.unix_socket

        # 规范化 BASE_URL 为以 / 开头且末尾无 / 的格式
        if args.base_url:
            base_url = args.base_url.strip()
            if base_url and not base_url.startswith('/'):
                base_url = '/' + base_url
            self.BASE_URL = base_url.rstrip('/')
        else:
            self.BASE_URL = ""

        self.PASSWORD = args.password
        
        self.MUSIC_LIBRARY_PATH = os.path.abspath(args.music_library_path or os.getcwd())
        os.makedirs(self.MUSIC_LIBRARY_PATH, exist_ok=True)
        
        self.LYRICS_DIR = os.path.join(self.MUSIC_LIBRARY_PATH, 'lyrics')
        self.COVERS_DIR = os.path.join(self.MUSIC_LIBRARY_PATH, 'covers')
        os.makedirs(self.LYRICS_DIR, exist_ok=True)
        os.makedirs(self.COVERS_DIR, exist_ok=True)
        
        self.DB_PATH = os.path.join(self.MUSIC_LIBRARY_PATH, 'data.db')
        self.LOG_FILE = args.log_path or os.path.join(os.getcwd(), 'app.log')
        os.makedirs(os.path.dirname(self.LOG_FILE), exist_ok=True)
        
        # 默认下载目录
        self.NETEASE_DOWNLOAD_DIR = os.path.join(self.MUSIC_LIBRARY_PATH, 'NetEase')

# 全局共享配置实例
app_config = AppConfig()
