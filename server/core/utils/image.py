import io
from PIL import Image
from core.utils.logger import logger

def compress_and_convert_to_webp(image_bytes: bytes, max_size=(500, 500)) -> bytes:
    """
    将输入的图片 bytes 进行尺寸缩放（保持比例，最大不超过 max_size），并转换为 WebP 格式返回。
    如果转换失败，则直接返回原始的 bytes 确保系统稳定性。
    """
    if not image_bytes:
        return image_bytes

    try:
        # 使用 Pillow 读取图片
        img = Image.open(io.BytesIO(image_bytes))
        
        # 限制最大宽高比并缩放
        img.thumbnail(max_size, Image.Resampling.LANCZOS)
        
        # 排除包含透明通道（RGBA / LA 等）或调色板（P）模式，转为 RGB 以兼容 WebP 格式
        if img.mode in ('RGBA', 'LA', 'P'):
            img = img.convert('RGB')
        elif img.mode != 'RGB':
            img = img.convert('RGB')

        # 写入内存 buffer
        out_buf = io.BytesIO()
        img.save(out_buf, format='WEBP', quality=80)
        compressed_bytes = out_buf.getvalue()
        
        # 仅当压缩后的体积确实变小时才使用（防止本来就很小的图压缩后反而变大）
        if len(compressed_bytes) < len(image_bytes):
            return compressed_bytes
        return image_bytes
    except Exception as e:
        logger.warning(f"Image compression and WebP conversion failed: {e}")
        return image_bytes
