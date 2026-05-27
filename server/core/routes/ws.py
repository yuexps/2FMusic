import json
import threading
from flask_sock import Sock
from core.utils.logger import logger

sock = Sock()
# 使用线程锁保护连接集合，防止多线程广播时产生并发修改异常
connected_clients = set()
clients_lock = threading.Lock()

def broadcast_ws_message(msg_type: str, data: dict):
    """向所有连接的客户端广播消息"""
    msg = json.dumps({'type': msg_type, 'data': data})
    with clients_lock:
        clients = list(connected_clients)
    
    for client in clients:
        try:
            client.send(msg)
        except Exception as e:
            logger.debug(f"向 WebSocket 客户端发送消息失败，正在清理该连接: {e}")
            with clients_lock:
                try:
                    connected_clients.remove(client)
                except Exception:
                    pass

def dispatch_ws_action(action: str, data: dict) -> tuple:
    """
    根据 action 路由并分发至各业务模块处理函数。
    返回格式: (success: bool, response_data: any, error_message: str)
    """
    try:
        # 1. 音乐相关 action
        if action.startswith('music/'):
            from core.routes.music import (
                handle_get_music_list,
                handle_delete_file,
                handle_clear_metadata,
                handle_get_lyrics,
                handle_get_album_art
            )
            if action == 'music/get_list':
                return handle_get_music_list()
            elif action == 'music/delete':
                return handle_delete_file(data.get('song_id'))
            elif action == 'music/clear_metadata':
                return handle_clear_metadata(data.get('song_id'), data.get('path'))
            elif action == 'music/lyrics':
                return handle_get_lyrics(data.get('title'), data.get('artist'), data.get('filename'), data.get('song_id'))
            elif action == 'music/album-art':
                return handle_get_album_art(data.get('title'), data.get('artist'), data.get('filename'), data.get('song_id'))

        # 2. 挂载目录相关 action
        elif action.startswith('mount/'):
            from core.routes.mounts import (
                handle_list_mount_points,
                handle_add_mount_point,
                handle_delete_mount_point,
                handle_update_mount_point,
                handle_retry_scrape_mount
            )
            if action == 'mount/list':
                return handle_list_mount_points()
            elif action == 'mount/add':
                return handle_add_mount_point(data.get('path'))
            elif action == 'mount/delete':
                return handle_delete_mount_point(data.get('path'))
            elif action == 'mount/scan':
                return handle_update_mount_point(data.get('path'))
            elif action == 'mount/retry_scrape':
                return handle_retry_scrape_mount(data.get('path'))

        # 3. 收藏夹相关 action
        elif action.startswith('favorite/'):
            from core.routes.favorites import (
                handle_get_favorite_playlists,
                handle_create_favorite_playlist,
                handle_delete_favorite_playlist,
                handle_get_playlist_songs,
                handle_add_favorite,
                handle_remove_favorite,
                handle_batch_add_favorites,
                handle_batch_remove_favorites,
                handle_batch_move_favorites
            )
            if action == 'favorite/list_playlists':
                return handle_get_favorite_playlists()
            elif action == 'favorite/create_playlist':
                return handle_create_favorite_playlist(data.get('name'))
            elif action == 'favorite/delete_playlist':
                return handle_delete_favorite_playlist(data.get('playlist_id'))
            elif action == 'favorite/playlist_songs':
                return handle_get_playlist_songs(data.get('playlist_id'))
            elif action == 'favorite/add':
                return handle_add_favorite(data.get('song_id'), data.get('playlist_id'), data.get('title'), data.get('artist'))
            elif action == 'favorite/delete':
                return handle_remove_favorite(data.get('song_id'), data.get('playlist_id'))
            elif action == 'favorite/batch_add':
                return handle_batch_add_favorites(data.get('song_ids'), data.get('playlist_ids'), data.get('songs'))
            elif action == 'favorite/batch_delete':
                return handle_batch_remove_favorites(data.get('song_ids'), data.get('playlist_ids'))
            elif action == 'favorite/batch_move':
                return handle_batch_move_favorites(data.get('song_ids'), data.get('from_playlist_id'), data.get('to_playlist_id'))

        # 4. 系统状态相关 action
        elif action.startswith('system/'):
            from core.routes.system import (
                handle_get_system_status,
                handle_get_lyrics_preference,
                handle_save_lyrics_preference
            )
            if action == 'system/get_status':
                return handle_get_system_status()
            elif action == 'system/get_lyrics_preference':
                return handle_get_lyrics_preference()
            elif action == 'system/save_lyrics_preference':
                return handle_save_lyrics_preference(data.get('value'))

        # 5. 网易云音乐相关 action
        elif action.startswith('netease/'):
            from core.routes.netease import (
                handle_search_netease_music,
                handle_netease_daily_recommend,
                handle_netease_login_status,
                handle_netease_logout,
                handle_netease_login_qrcode,
                handle_netease_login_check,
                handle_netease_config,
                handle_netease_resolve,
                handle_netease_playlist_detail,
                handle_netease_song_detail,
                handle_download_netease_music,
                handle_get_netease_task_detail,
                handle_get_install_status,
                handle_install_netease_service,
                handle_check_docker_container
            )
            if action == 'netease/search':
                return handle_search_netease_music(data.get('keywords'), data.get('limit'))
            elif action == 'netease/recommend':
                return handle_netease_daily_recommend()
            elif action == 'netease/login_status':
                return handle_netease_login_status()
            elif action == 'netease/logout':
                return handle_netease_logout()
            elif action == 'netease/login_qrcode':
                return handle_netease_login_qrcode()
            elif action == 'netease/login_check':
                return handle_netease_login_check(data.get('key'))
            elif action == 'netease/get_config':
                return handle_netease_config('GET')
            elif action == 'netease/save_config':
                return handle_netease_config('POST', data.get('download_dir'), data.get('api_base'))
            elif action == 'netease/resolve':
                return handle_netease_resolve(data.get('input'))
            elif action == 'netease/playlist':
                return handle_netease_playlist_detail(data.get('id'))
            elif action == 'netease/song':
                return handle_netease_song_detail(data.get('id'))
            elif action == 'netease/download':
                return handle_download_netease_music(data)
            elif action == 'netease/task_status':
                return handle_get_netease_task_detail(data.get('task_id'))
            elif action == 'netease/install_status':
                return handle_get_install_status()
            elif action == 'netease/install_service':
                return handle_install_netease_service()
            elif action == 'netease/check_container':
                return handle_check_docker_container()

        # 6. 播放历史相关 action
        elif action.startswith('history/'):
            from core.routes.history import (
                handle_get_play_history,
                handle_add_play_history,
                handle_clear_play_history,
                handle_remove_play_history
            )
            if action == 'history/get':
                return handle_get_play_history()
            elif action == 'history/add':
                return handle_add_play_history(data.get('song_id'))
            elif action == 'history/clear':
                return handle_clear_play_history()
            elif action == 'history/remove':
                return handle_remove_play_history(data.get('song_id'), data.get('play_time'))

        return False, None, f"未知的 WebSocket Action: {action}"
    except Exception as e:
        logger.exception(f"执行 action {action} 发生异常: {e}")
        return False, None, f"服务器内部处理异常: {str(e)}"

def register_ws(app):
    """初始化并挂载 WebSocket 连接处理器"""
    sock.init_app(app)
    
    # 注册服务层广播回调
    from core.services.scanner import register_ws_broadcast_callback
    from core.services.downloader import register_downloader_ws_broadcast_callback
    
    register_ws_broadcast_callback(broadcast_ws_message)
    register_downloader_ws_broadcast_callback(broadcast_ws_message)
    
    @sock.route('/api/ws')
    def ws_handler(ws):
        logger.info("新 WebSocket 客户端已连接")
        with clients_lock:
            connected_clients.add(ws)
        try:
            while True:
                raw_data = ws.receive()
                if raw_data is None:
                    break
                
                try:
                    payload = json.loads(raw_data)
                except Exception as e:
                    logger.warning(f"接收到非法的 JSON 帧: {e}")
                    continue

                action = payload.get('action')
                if not action:
                    continue

                # 心跳响应
                if action == 'ping':
                    try:
                        ws.send(json.dumps({'type': 'pong'}))
                    except Exception:
                        break
                    continue

                # 业务请求路由分发
                seq = payload.get('seq')
                data = payload.get('data') or {}
                
                success, res_data, error_msg = dispatch_ws_action(action, data)
                
                # 回写响应帧
                response = {
                    'seq': seq,
                    'type': 'response',
                    'action': action,
                    'success': success,
                    'data': res_data,
                    'error': error_msg
                }
                try:
                    ws.send(json.dumps(response))
                except Exception:
                    break

        except Exception as e:
            logger.debug(f"WebSocket 异常断开: {e}")
        finally:
            with clients_lock:
                try:
                    connected_clients.remove(ws)
                except Exception:
                    pass
            logger.info("WebSocket 客户端断开连接")
