#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
caption_runner.py — 后台图注填充器（A 方案阶段4）
遍历 /root/mineru-out 全部图片(>=8KB)，调 ollama qwen3-vl:2b 生成中文图注，
按 sha256 写入 /root/.caption_cache.json（与 mineru2kb.py 共用缓存格式）。
幂等可续跑：已缓存直接跳过；自然完成后写 /root/.caption_done 标记。
"""
import base64
import hashlib
import json
import os
import re
import sys
import time
import urllib.request

MINERU_OUT = "/root/mineru-out"
CACHE_FILE = "/root/.caption_cache.json"
TARGET_FILE = "/root/.caption_target.txt"
DONE_MARKER = "/root/.caption_done"
MODEL = os.getenv("CAPTION_MODEL", "qwen3-vl:2b")
OLLAMA = os.getenv("CAPTION_OLLAMA", "http://127.0.0.1:11434")
PROMPT = (
    "你是叉车维修培训专家。用中文描述这张手册图片的核心内容"
    "（部件名称、位置关系、操作要点），70字以内，直接输出描述，不要任何前缀。"
)
MIN_BYTES = 8 * 1024  # 与 mineru2kb 一致：<8KB 图标丢弃，不值得图注
SAVE_EVERY = 100


def load_cache():
    try:
        with open(CACHE_FILE, encoding="utf-8") as fp:
            return json.load(fp)
    except Exception:
        return {}


def save_cache(cache):
    tmp = CACHE_FILE + ".tmp"
    with open(tmp, "w", encoding="utf-8") as fp:
        json.dump(cache, fp, ensure_ascii=False)
    os.replace(tmp, CACHE_FILE)


def check_model():
    """预检：ollama 可达且 qwen3-vl:2b 已拉取。"""
    try:
        tags = json.load(urllib.request.urlopen(OLLAMA + "/api/tags", timeout=15))
    except Exception as e:
        print(f"ERROR: ollama 不可达 {OLLAMA}: {e}", flush=True)
        sys.exit(2)
    names = [m.get("name", "") for m in tags.get("models", [])]
    if not any(n.startswith(MODEL.split(":")[0]) for n in names):
        print(f"ERROR: 模型 {MODEL} 未拉取，现有: {names}", flush=True)
        sys.exit(2)


def collect_images():
    out = []
    for root, _, files in os.walk(MINERU_OUT):
        for f in files:
            if f.lower().endswith((".jpg", ".jpeg", ".png")):
                p = os.path.join(root, f)
                try:
                    if os.path.getsize(p) >= MIN_BYTES:
                        out.append(p)
                except OSError:
                    pass
    return out


def caption(img_bytes):
    b64 = base64.b64encode(img_bytes).decode()
    payload = json.dumps(
        {"model": MODEL, "prompt": PROMPT, "images": [b64], "stream": False}
    ).encode()
    req = urllib.request.Request(
        OLLAMA + "/api/generate", data=payload,
        headers={"Content-Type": "application/json"})
    resp = json.load(urllib.request.urlopen(req, timeout=180))
    text = resp.get("response", "")
    text = re.sub(r"<think>.*?</think>", "", text, flags=re.S).strip()
    text = re.sub(r"\s+", " ", text)[:120]
    return text


def main():
    check_model()
    if os.path.exists(DONE_MARKER):
        os.remove(DONE_MARKER)
    cache = load_cache()
    imgs = collect_images()
    with open(TARGET_FILE, "w") as fp:
        fp.write(str(len(imgs)))
    print(f"目标图片: {len(imgs)}  已缓存: {len(cache)}", flush=True)
    done = fail = 0
    t0 = time.time()
    since_save = 0
    for i, p in enumerate(imgs, 1):
        try:
            with open(p, "rb") as fp:
                data = fp.read()
        except OSError:
            continue
        h = hashlib.sha256(data).hexdigest()
        if h in cache:
            done += 1
            continue
        try:
            cap = caption(data)
        except Exception as e:
            fail += 1
            if fail % 50 == 1:
                print(f"  [失败{fail}] {os.path.basename(p)}: {str(e)[:60]}", flush=True)
            time.sleep(1)
            continue
        if cap:
            cache[h] = cap
            done += 1
            since_save += 1
            if since_save >= SAVE_EVERY:
                save_cache(cache)
                since_save = 0
        if i % 200 == 0:
            el = time.time() - t0
            rate = i / el if el > 0 else 0
            eta_h = (len(imgs) - i) / rate / 3600 if rate > 0 else -1
            print(f"进度 {i}/{len(imgs)} 缓存{len(cache)} "
                  f"速率{rate:.2f}图/s 预计剩余{eta_h:.1f}h", flush=True)
    save_cache(cache)
    with open(DONE_MARKER, "w") as fp:
        fp.write(f"complete {len(cache)}/{len(imgs)} fail={fail}")
    print(f"完成: 缓存 {len(cache)}/{len(imgs)} 失败 {fail} "
          f"耗时{(time.time() - t0) / 3600:.1f}h", flush=True)


if __name__ == "__main__":
    main()
