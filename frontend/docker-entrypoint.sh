#!/bin/sh

# Only emit whitelisted locale tags to avoid config.js injection from env values.
RUNTIME_DEFAULT_LOCALE=""
case "${DEFAULT_LOCALE:-}" in
  zh-CN|zh-TW|en-US|ru-RU|ko-KR|ja-JP) RUNTIME_DEFAULT_LOCALE="${DEFAULT_LOCALE}" ;;
esac

# Only an origin is allowed here. This value is emitted into JavaScript below:
# reject credentials, paths, quotes and control characters before interpolation.
RUNTIME_LANGFLOW_EDITOR_URL=${LANGFLOW_EDITOR_URL:-}
if [ -n "$RUNTIME_LANGFLOW_EDITOR_URL" ]; then
  if [ "$(printf '%s' "$RUNTIME_LANGFLOW_EDITOR_URL" | wc -l)" -ne 0 ] || ! printf '%s' "$RUNTIME_LANGFLOW_EDITOR_URL" | grep -Eq '^(https://[A-Za-z0-9.-]+|http://(localhost|127\.0\.0\.1|\[::1\]))(:[0-9]{1,5})?/?$'; then
    echo 'Invalid LANGFLOW_EDITOR_URL: expected HTTPS origin or local HTTP origin' >&2
    exit 1
  fi
fi

# 生成运行时配置文件，注入环境变量到前端
FILE_MB=${MAX_FILE_SIZE_MB:-50}
SKILL_MB=${MAX_SKILL_BUNDLE_SIZE_MB:-256}
if [ "$SKILL_MB" -lt "$FILE_MB" ] 2>/dev/null; then
  SKILL_MB=$FILE_MB
fi
if [ "$SKILL_MB" -gt 512 ] 2>/dev/null; then
  SKILL_MB=512
fi

cat > /usr/share/nginx/html/config.js << EOF
window.__RUNTIME_CONFIG__ = {
  MAX_FILE_SIZE_MB: ${FILE_MB},
  MAX_SKILL_BUNDLE_SIZE_MB: ${SKILL_MB},
  DEFAULT_LOCALE: "${RUNTIME_DEFAULT_LOCALE}",
  LANGFLOW_EDITOR_URL: "${RUNTIME_LANGFLOW_EDITOR_URL}"
};
EOF

# 处理 nginx 配置。
# 两个上限分开注入：全站保持知识库的 MAX_FILE_SIZE，只有技能 zip 上传的两条
# 集合路由放宽到 MAX_SKILL_BUNDLE_SIZE（不含 /install、PATCH 等子路径）。
# 合成一个全站上限会让每个上传端点都能收到技能包那么大的 body。
export MAX_FILE_SIZE=${FILE_MB}M
export MAX_SKILL_BUNDLE_SIZE=${SKILL_MB}M
export APP_HOST=${APP_HOST:-app}
export APP_PORT=${APP_PORT:-8080}
export APP_SCHEME=${APP_SCHEME:-http}
envsubst '${MAX_FILE_SIZE} ${MAX_SKILL_BUNDLE_SIZE} ${APP_HOST} ${APP_PORT} ${APP_SCHEME}' \
  < /etc/nginx/templates/default.conf.template > /etc/nginx/conf.d/default.conf

# 启动 nginx
exec nginx -g 'daemon off;'
