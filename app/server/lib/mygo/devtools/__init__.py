import logging
import functools

logger = logging.getLogger(__name__)

def no_error(throw=None, exceptions=(Exception,)):
    """
    Decorator to suppress exceptions.
    :param throw: Function to call with the exception if one occurs (e.g. logger.error).
    :param exceptions: Tuple of exception classes to catch.
    """
    def decorator(func):
        @functools.wraps(func)
        def wrapper(*args, **kwargs):
            try:
                return func(*args, **kwargs)
            except exceptions as e:
                if throw:
                    throw(f"Error in {func.__name__}: {e}")
                return None
        return wrapper
    return decorator

class Benchmark:
    def __init__(self, threads=1, rounds=1):
        self.threads = threads
        self.rounds = rounds
    
    def run(self, func, *args, **kwargs):
        # Mock implementation
        return 0, 0
