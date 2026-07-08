from flask import Blueprint, request, jsonify
import requests
import json
from core.models.preferences import get_preference, set_preference
from core.utils.logger import logger

folia_bp = Blueprint('folia', __name__)


@folia_bp.route('/api/generate-theme_openai', methods=['POST'])
@folia_bp.route('/api/generate-theme', methods=['POST'])
def handle_folia_ai_theme_proxy():
    """代理大模型请求为 Folia 生成主题配色 (即用即取，杜绝 API Key 进入浏览器)"""
    enable_ai = get_preference('folia_enable_ai', 'false') == 'true'
    if not enable_ai:
        return jsonify({'error': 'Custom AI theme generation is disabled in host settings'}), 400

    openai_key = get_preference('folia_openai_key', '')
    if not openai_key:
        return jsonify({'error': 'OpenAI API Key is not configured in host settings'}), 400

    openai_url = get_preference('folia_openai_url', 'https://api.openai.com/v1')
    openai_model = get_preference('folia_openai_model', 'gpt-4o')
    openai_proxy = get_preference('folia_openai_proxy', '')

    proxies = None
    if openai_proxy and openai_proxy.strip():
        proxy_val = openai_proxy.strip()
        proxies = {
            "http": proxy_val,
            "https": proxy_val
        }

    try:
        req_json = request.get_json() or {}
        lyrics_text = req_json.get('lyricsText', '')
        if not lyrics_text:
            return jsonify({'error': 'Lyrics text is required'}), 400

        # 构建 Prompt，格式符合 DualTheme 要求
        prompt = """Analyze the mood of the provided song lyrics and generate TWO visual theme configurations for a music player - one for LIGHT mode and one for DARK mode.
Return ONLY a valid JSON object matching this schema:
{
  "light": {
    "name": "Creative name (<=10 chars) in Chinese",
    "description": "Creative description (15-30 chars) in Chinese",
    "backgroundColor": "Hex code for light background",
    "primaryColor": "Hex code for primary text",
    "accentColor": "Hex code for highlights",
    "secondaryColor": "Hex code for secondary text",
    "wordColors": [{"word": "standalone emotional word from lyrics", "color": "Hex color"}],
    "lyricsIcons": ["lucide icon name", "lucide icon name"]
  },
  "dark": {
    "name": "Creative name (<=10 chars) in Chinese",
    "description": "Creative description (15-30 chars) in Chinese",
    "backgroundColor": "Hex code for dark background",
    "primaryColor": "Hex code for primary text",
    "accentColor": "Hex code for highlights",
    "secondaryColor": "Hex code for secondary text",
    "wordColors": [{"word": "standalone emotional word from lyrics", "color": "Hex color"}],
    "lyricsIcons": ["lucide icon name", "lucide icon name"]
  }
}
Lyrics:
""" + lyrics_text

        payload = {
            "model": openai_model,
            "messages": [
                {"role": "user", "content": prompt}
            ],
            "temperature": 0.7
        }

        headers = {
            "Authorization": f"Bearer {openai_key}",
            "Content-Type": "application/json"
        }

        api_url = openai_url.strip().rstrip('/') + '/chat/completions'
        resp = requests.post(api_url, json=payload, headers=headers, proxies=proxies, timeout=20.0)
        resp.raise_for_status()

        res_data = resp.json()
        content = res_data['choices'][0]['message']['content'].strip()

        # 处理 markdown JSON 栅栏
        if content.startswith('```json'):
            content = content[7:]
        if content.endswith('```'):
            content = content[:-3]
        content = content.strip()

        # 加载并返回规范 the JSON 对象给前端
        return jsonify(json.loads(content))

    except Exception as e:
        logger.error(f"Folia AI theme proxy request failed: {e}")
        return jsonify({'error': 'AI Theme proxy request failed', 'details': str(e)}), 500


def handle_get_folia_ai_config() -> tuple:
    """获取 Folia AI 主题大模型配置"""
    try:
        enable_ai = get_preference('folia_enable_ai', 'false')
        openai_url = get_preference('folia_openai_url', 'https://api.openai.com/v1')
        openai_model = get_preference('folia_openai_model', 'gpt-4o')
        openai_key = get_preference('folia_openai_key', '')
        openai_proxy = get_preference('folia_openai_proxy', '')
        return True, {
            'folia_enable_ai': enable_ai == 'true',
            'openai_url': openai_url,
            'openai_model': openai_model,
            'openai_key': openai_key,
            'openai_proxy': openai_proxy
        }, None
    except Exception as e:
        logger.exception(f"获取 Folia AI 配置失败: {e}")
        return False, None, str(e)


def handle_save_folia_ai_config(enable_ai: bool, openai_url: str, openai_model: str, openai_key: str, openai_proxy: str = None) -> tuple:
    """保存 Folia AI 主题大模型配置"""
    try:
        set_preference('folia_enable_ai', 'true' if enable_ai else 'false')
        set_preference('folia_openai_url', (openai_url or '').strip())
        set_preference('folia_openai_model', (openai_model or '').strip())
        set_preference('folia_openai_key', (openai_key or '').strip())
        if openai_proxy is not None:
            set_preference('folia_openai_proxy', (openai_proxy or '').strip())
        return True, None, None
    except Exception as e:
        logger.exception(f"保存 Folia AI 配置失败: {e}")
        return False, None, str(e)
