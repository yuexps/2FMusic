# -*- coding: utf-8 -*-
import os
import logging

class LogManager:
    """统一日志管理器"""
    _logger = None

    @classmethod
    def setup(cls, log_file: str, level=logging.INFO) -> logging.Logger:
        """
        初始化全局日志器配置。
        支持控制台流输出与写入物理日志文件。
        """
        cls._logger = logging.getLogger("2fmusic")
        cls._logger.setLevel(level)
        cls._logger.handlers.clear()
        cls._logger.propagate = False

        # 确保日志存储目录存在
        if log_file:
            log_dir = os.path.dirname(os.path.abspath(log_file))
            if log_dir:
                try:
                    os.makedirs(log_dir, exist_ok=True)
                except Exception:
                    pass

            try:
                file_handler = logging.FileHandler(log_file, mode='w', encoding='utf-8')
                file_handler.setFormatter(logging.Formatter('%(asctime)s - %(levelname)s - %(message)s'))
                cls._logger.addHandler(file_handler)
            except Exception as e:
                print(f"警告：创建日志文件处理器失败: {e}")

        # 始终添加控制台输出处理器
        console_handler = logging.StreamHandler()
        console_handler.setFormatter(logging.Formatter('%(levelname)s: %(message)s'))
        cls._logger.addHandler(console_handler)

        return cls._logger

    @classmethod
    def get_logger(cls) -> logging.Logger:
        """获取唯一的全局 Logger 实例"""
        if cls._logger is None:
            cls._logger = logging.getLogger("2fmusic")
        return cls._logger

# 导出供外部使用的默认 logger 实例
logger = LogManager.get_logger()
