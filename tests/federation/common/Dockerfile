# Test runner image for federation tests. Each target (misskey, mastodon, ...)
# shares this image; the per-target test directory is mounted at /tests/<target>.
FROM python:3.12-slim

WORKDIR /tests

COPY common/requirements.txt /requirements.txt
RUN pip install --no-cache-dir -r /requirements.txt

# デフォルトでは misskey ターゲットを実行する。個別に変更したい場合は
# docker-compose の command 側で上書きする。
CMD ["pytest", "-xvs", "--tb=short"]
