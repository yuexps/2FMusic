#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <sys/socket.h>
#include <arpa/inet.h>
#include <unistd.h>
#include <sys/stat.h>
#include <fcntl.h>
#include <libgen.h>
#include <strings.h> // Required for strcasecmp

#define BACKEND_IP "127.0.0.1"
#define BACKEND_PORT 23237
#define BUFFER_SIZE 4096
#define CGI_NAME "index.cgi"

// --- 工具函数Utils ---

void error_response(int status, const char *msg) {
    if (status == 404) printf("Status: 404 Not Found\n\n");
    else if (status == 403) printf("Status: 403 Forbidden\n\n");
    else if (status == 500) printf("Status: 500 Internal Server Error\n\n");
    else if (status == 502) printf("Status: 502 Bad Gateway\n\n");
    else printf("Status: %d Error\n\n", status);
    
    printf("%s", msg);
    exit(0);
}

// --- 路径解析Path Parsing ---

// 如果 PATH_INFO 缺失或不可靠，通过 REQUEST_URI 解析
void get_relative_path(char *buffer, size_t size) {
    char *path_info = getenv("PATH_INFO");

    // 如果 PATH_INFO 包含 index.cgi，转而使用更可靠的 REQUEST_URI 解析
    if (path_info && strlen(path_info) > 0 && strstr(path_info, CGI_NAME) == NULL) {
        strncpy(buffer, path_info, size - 1);
        return;
    }

    char *request_uri = getenv("REQUEST_URI");
    if (!request_uri) {
        strcpy(buffer, "/");
        return;
    }

    // 去除查询字符串
    char uri_no_query[2048];
    strncpy(uri_no_query, request_uri, sizeof(uri_no_query) - 1);
    char *q = strchr(uri_no_query, '?');
    if (q) *q = '\0';

    // 查找 "index.cgi"
    char *p = strstr(uri_no_query, CGI_NAME);
    if (p) {
        strncpy(buffer, p + strlen(CGI_NAME), size - 1);
    } else {
        // 后备或根路径
        strcpy(buffer, "/");
    }
}

// --- 静态文件服务Static File Server ---
const char* get_mime_type(const char *path) {
    const char *ext = strrchr(path, '.');
    if (!ext) return "application/octet-stream";
    if (strcasecmp(ext, ".html") == 0) return "text/html; charset=utf-8";
    if (strcasecmp(ext, ".js") == 0) return "application/javascript; charset=utf-8";
    if (strcasecmp(ext, ".css") == 0) return "text/css; charset=utf-8";
    if (strcasecmp(ext, ".png") == 0) return "image/png";
    if (strcasecmp(ext, ".jpg") == 0 || strcasecmp(ext, ".jpeg") == 0) return "image/jpeg";
    if (strcasecmp(ext, ".svg") == 0) return "image/svg+xml";
    if (strcasecmp(ext, ".json") == 0) return "application/json; charset=utf-8";
    return "application/octet-stream";
}

#define ASSET_ROOT "/var/apps/2fmusic-preview/target/ui"

void serve_static_file(const char *rel_path) {
    // 基础保护：防止目录遍历
    if (strstr(rel_path, "..")) {
        error_response(403, "Access Denied");
    }

    // 规范化路径：跳过开头的斜杠
    const char *p = (rel_path[0] == '/') ? rel_path + 1 : rel_path;
    
    // 确定基础路径 (Base Path)
    char base_path[1024];
    struct stat s;
    if (stat(ASSET_ROOT, &s) == 0 && S_ISDIR(s.st_mode)) {
        // 如果系统绝对路径存在 (生产环境)
        strcpy(base_path, ASSET_ROOT);
    } else {
        // 否则使用相对路径 (开发环境/Fallback)
        strcpy(base_path, ".");
    }

    // 构建完整文件路径
    char filepath[2048];
    // 如果请求路径为空或根，服务 preview.html
    if (strlen(p) == 0) {
        snprintf(filepath, sizeof(filepath), "%s/preview.html", base_path);
    } else {
        snprintf(filepath, sizeof(filepath), "%s/%s", base_path, p);
    }

    FILE *f = fopen(filepath, "rb");
    if (!f) {
        // 尝试 fallback 到 preview.html
        // 简单处理：严格 404
        char err_msg[2048];
        snprintf(err_msg, sizeof(err_msg), "File Not Found: %s", filepath);
        error_response(404, err_msg);
    }

    const char *mime = get_mime_type(filepath);
    printf("X-Debug-Path: %s\n", filepath);
    printf("Content-Type: %s\n\n", mime);
    fflush(stdout);

    char buffer[BUFFER_SIZE];
    size_t n;
    while ((n = fread(buffer, 1, sizeof(buffer), f)) > 0) {
        fwrite(buffer, 1, n, stdout);
    }
    fclose(f);
}

// --- 反向代理Reverse Proxy ---
void proxy_request(const char *rel_path) {
    int sock;
    struct sockaddr_in server_addr;
    char buffer[BUFFER_SIZE];

    // 创建 Socket
    if ((sock = socket(AF_INET, SOCK_STREAM, 0)) < 0) {
        error_response(500, "Socket creation failed");
    }

    server_addr.sin_family = AF_INET;
    server_addr.sin_port = htons(BACKEND_PORT);
    if (inet_pton(AF_INET, BACKEND_IP, &server_addr.sin_addr) <= 0) {
        error_response(500, "Invalid address");
    }

    // 设置超时
    struct timeval tv;
    tv.tv_sec = 5;
    tv.tv_usec = 0;
    setsockopt(sock, SOL_SOCKET, SO_RCVTIMEO, (const char*)&tv, sizeof tv);
    setsockopt(sock, SOL_SOCKET, SO_SNDTIMEO, (const char*)&tv, sizeof tv);

    if (connect(sock, (struct sockaddr *)&server_addr, sizeof(server_addr)) < 0) {
        error_response(502, "Bad Gateway: Cannot connect to 2FMusic Backend");
    }

    // 准备请求头
    char *method = getenv("REQUEST_METHOD");
    if (!method) method = "GET";
    
    char *query_string = getenv("QUERY_STRING");
    char full_path[4096];
    
    // 代理以 /api/ 开头的路径
    if (query_string && strlen(query_string) > 0) {
        snprintf(full_path, sizeof(full_path), "%s?%s", rel_path, query_string);
    } else {
        snprintf(full_path, sizeof(full_path), "%s", rel_path);
    }

    // 构建请求
    dprintf(sock, "%s %s HTTP/1.1\r\n", method, full_path);
    dprintf(sock, "Host: %s:%d\r\n", BACKEND_IP, BACKEND_PORT);
    dprintf(sock, "Connection: close\r\n");
    
    // 转发特定 Header
    char *content_type = getenv("CONTENT_TYPE");
    if (content_type) dprintf(sock, "Content-Type: %s\r\n", content_type);
    
    char *content_length = getenv("CONTENT_LENGTH");
    if (content_length) dprintf(sock, "Content-Length: %s\r\n", content_length);
    
    char *cookie = getenv("HTTP_COOKIE");
    if (cookie) dprintf(sock, "Cookie: %s\r\n", cookie);
    
    char *user_agent = getenv("HTTP_USER_AGENT");
    if (user_agent) dprintf(sock, "User-Agent: %s\r\n", user_agent);

    char *authorization = getenv("HTTP_AUTHORIZATION");
    if (authorization) dprintf(sock, "Authorization: %s\r\n", authorization);

    char *referer = getenv("HTTP_REFERER");
    if (referer) dprintf(sock, "Referer: %s\r\n", referer);
    
    char *accept = getenv("HTTP_ACCEPT");
    if (accept) dprintf(sock, "Accept: %s\r\n", accept);
    
    char *accept_language = getenv("HTTP_ACCEPT_LANGUAGE");
    if (accept_language) dprintf(sock, "Accept-Language: %s\r\n", accept_language);

    char *x_requested_with = getenv("HTTP_X_REQUESTED_WITH");
    if (x_requested_with) dprintf(sock, "X-Requested-With: %s\r\n", x_requested_with);

    dprintf(sock, "\r\n"); // 头部结束

    // 转发 POST/PUT 的 API Body
    if (content_length) {
        int cl = atoi(content_length);
        int total_read = 0;
        int to_read;
        while (total_read < cl) {
            to_read = (cl - total_read) > sizeof(buffer) ? sizeof(buffer) : (cl - total_read);
            int n = fread(buffer, 1, to_read, stdin);
            if (n <= 0) break;
            write(sock, buffer, n);
            total_read += n;
        }
    }

    // 读取响应
    ssize_t n = read(sock, buffer, sizeof(buffer) - 1);
    if (n > 0) {
        // 去除 HTTP 状态行并替换为 CGI 状态
        char *header_end = strstr(buffer, "\r\n\r\n");
        if (header_end) {
            *header_end = '\0'; 
            
            // 处理状态行
            char *line = strtok(buffer, "\r\n");
            // 示例: HTTP/1.1 200 OK
            if (line && strncmp(line, "HTTP", 4) == 0) {
                char *space = strchr(line, ' ');
                if (space) {
                    printf("Status:%s\n", space); 
                }
            }
            
            // 输出其他 Header
            while ((line = strtok(NULL, "\r\n")) != NULL) {
                 if (strncasecmp(line, "Transfer-Encoding", 17) == 0) continue;
                 if (strncasecmp(line, "Connection", 10) == 0) continue;
                 printf("%s\n", line);
            }
            printf("\n"); // 头部结束
            
            // 写入 Body
            fwrite(header_end + 4, 1, n - (header_end - buffer) - 4, stdout);
        } else {
             // 后备方案
             fwrite(buffer, 1, n, stdout);
        }
        
        while ((n = read(sock, buffer, sizeof(buffer))) > 0) {
            fwrite(buffer, 1, n, stdout);
        }
    } else {
        // 空响应或错误
    }

    close(sock);
}


int main() {
    // 1. 确定相对路径
    char rel_path[2048] = {0};
    get_relative_path(rel_path, sizeof(rel_path));
    
    // 2. 路由 (Route)
    if (strncmp(rel_path, "/api/", 5) == 0 || strncmp(rel_path, "/login", 6) == 0) {
        proxy_request(rel_path);
    } else {
        serve_static_file(rel_path);
    }

    return 0;
}
