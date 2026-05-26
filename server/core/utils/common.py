
COMMON_HEADERS = {
    'User-Agent': 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36',
    'Authorization': '2FMusic'
}

def parse_cookie_string(cookie_str: str) -> dict:
    """将 Set-Cookie 字符串解析为 requests 兼容的字典"""
    if not cookie_str: 
        return {}
    cookies = {}
    # 只取 key=value 形式，忽略 Path/Expires 等属性
    for part in cookie_str.split(';'):
        if '=' in part:
            k, v = part.strip().split('=', 1)
            if k.lower() in ('path', 'expires', 'max-age', 'domain', 'samesite', 'secure'): 
                continue
            cookies[k] = v
    return cookies

def normalize_cookie_string(raw: str) -> str:
    """规范化 cookie 字符串，移除换行并过滤非关键属性"""
    if not raw: 
        return ''
    parts = []
    # 常见的 Set-Cookie 属性，不应出现在请求头 Cookie 中
    skip_keys = ('path', 'expires', 'max-age', 'domain', 'samesite', 'secure', 'httponly')
    
    for part in raw.replace('\n', ';').split(';'):
        part = part.strip()
        if not part: 
            continue
        
        # 忽略没有等号的属性
        if '=' not in part: 
            continue
            
        k, v = part.split('=', 1)
        if k.strip().lower() in skip_keys:
            continue
            
        parts.append(part)
        
    return '; '.join(parts)
