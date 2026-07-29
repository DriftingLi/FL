"""
LibreOffice Sidecar HTTP 服务
提供 PPT → PDF → PNG 图片转换的 HTTP API
共享 /data/uploads volume 与 backend 容器

API:
  POST /convert
    body: {"input_path": "/data/uploads/chapters/xxx.pptx", "output_dir": "/data/uploads/slides/123"}
    response: {"success": true, "images": ["slide_001.png", "slide_002.png"]}
    或    {"success": false, "error": "错误信息"}

  GET /health
    response: {"status": "ok"}
"""

import os
import re
import subprocess
import threading
from flask import Flask, request, jsonify

app = Flask(__name__)

# 并发锁:LibreOffice 同时多个实例会冲突
LO_LOCK = threading.Lock()


def convert_ppt_to_pdf(input_path: str, output_dir: str) -> str | None:
    """用 LibreOffice headless 把 PPT 转 PDF"""
    os.makedirs(output_dir, exist_ok=True)
    try:
        with LO_LOCK:
            cmd = ["soffice", "--headless", "--convert-to", "pdf",
                   "--outdir", output_dir, input_path]
            result = subprocess.run(cmd, capture_output=True, timeout=120)
            if result.returncode != 0:
                print(f"LibreOffice 转换失败: {result.stderr.decode(errors='replace')}")
                return None
    except subprocess.TimeoutExpired:
        print("LibreOffice 转换超时(120s)")
        return None
    except Exception as e:
        print(f"LibreOffice 转换异常: {e}")
        return None

    # 查找输出 PDF
    base_name = os.path.splitext(os.path.basename(input_path))[0]
    pdf_path = os.path.join(output_dir, base_name + ".pdf")
    if not os.path.exists(pdf_path):
        # 找任意 PDF
        for f in os.listdir(output_dir):
            if f.lower().endswith(".pdf"):
                pdf_path = os.path.join(output_dir, f)
                break
    if not os.path.exists(pdf_path):
        return None
    return pdf_path


def convert_pdf_to_images(pdf_path: str, output_dir: str) -> list:
    """用 pdftoppm 把 PDF 转成 PNG 图片"""
    prefix = os.path.join(output_dir, "slide")
    try:
        cmd = ["pdftoppm", "-png", "-r", "150", pdf_path, prefix]
        result = subprocess.run(cmd, capture_output=True, timeout=60)
        if result.returncode != 0:
            print(f"pdftoppm 转换失败: {result.stderr.decode(errors='replace')}")
            return []
    except subprocess.TimeoutExpired:
        print("pdftoppm 转换超时(60s)")
        return []
    except Exception as e:
        print(f"pdftoppm 转换异常: {e}")
        return []

    # 重命名 slide-1.png → slide_001.png
    images = []
    for f in sorted(os.listdir(output_dir)):
        if not f.lower().endswith(".png"):
            continue
        # slide-1.png 格式
        m = re.match(r"^(.+)-(\d+)\.png$", f)
        if m:
            num = int(m.group(2))
            new_name = f"slide_{num:03d}.png"
            old_path = os.path.join(output_dir, f)
            new_path = os.path.join(output_dir, new_name)
            os.rename(old_path, new_path)
            images.append(new_name)
        elif f.startswith("slide_"):
            images.append(f)
    return images


@app.route("/health", methods=["GET"])
def health():
    return jsonify({"status": "ok"})


@app.route("/convert", methods=["POST"])
def convert():
    data = request.get_json(silent=True) or {}
    input_path = data.get("input_path", "")
    output_dir = data.get("output_dir", "")

    if not input_path or not output_dir:
        return jsonify({"success": False, "error": "input_path 和 output_dir 必填"}), 400

    if not os.path.exists(input_path):
        return jsonify({"success": False, "error": f"输入文件不存在: {input_path}"}), 404

    try:
        os.makedirs(output_dir, exist_ok=True)

        # 1. PPT → PDF
        pdf_path = convert_ppt_to_pdf(input_path, output_dir)
        if not pdf_path:
            return jsonify({"success": False, "error": "LibreOffice 转换 PDF 失败"}), 500

        # 2. PDF → PNG
        images = convert_pdf_to_images(pdf_path, output_dir)

        # 3. 删除中间 PDF
        try:
            os.remove(pdf_path)
        except Exception:
            pass

        if not images:
            return jsonify({"success": False, "error": "PDF 转图片失败"}), 500

        return jsonify({"success": True, "images": images})

    except Exception as e:
        return jsonify({"success": False, "error": str(e)}), 500


if __name__ == "__main__":
    # 监听 8000 端口,仅 Docker 内网访问
    app.run(host="0.0.0.0", port=8000, threaded=False)
