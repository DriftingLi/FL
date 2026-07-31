"""
LibreOffice Sidecar HTTP 服务
提供 PPT → PDF → PNG 图片转换的 HTTP API
通过 HTTP multipart 收发文件内容,不再依赖共享 volume

API:
  POST /convert
    multipart/form-data:
      file: PPT 文件二进制内容 (.ppt/.pptx)
      chapter_id: 字符串,用于临时文件命名
    response: {"success": true, "images": [{"name": "slide_001.png", "data": "<base64>"}, ...]}
    或    {"success": false, "error": "错误信息"}

  GET /health
    response: {"status": "ok"}
"""

import base64
import os
import re
import shutil
import subprocess
import threading
import time
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
    # 接收 multipart/form-data
    if "file" not in request.files:
        return jsonify({"success": False, "error": "缺少 file 字段"}), 400
    file = request.files["file"]
    chapter_id = (request.form.get("chapter_id") or "").strip()

    if not file or not file.filename:
        return jsonify({"success": False, "error": "file 不能为空"}), 400
    if not chapter_id:
        return jsonify({"success": False, "error": "chapter_id 必填"}), 400

    timestamp = int(time.time())
    # 临时 PPT 文件路径
    ppt_path = f"/tmp/ppt_{chapter_id}_{timestamp}.pptx"
    # 临时输出目录
    output_dir = f"/tmp/slides_{chapter_id}_{timestamp}"

    try:
        # 1. 把上传的 PPT 写到临时文件
        file.save(ppt_path)
        if not os.path.exists(ppt_path):
            return jsonify({"success": False, "error": "保存上传文件失败"}), 500

        os.makedirs(output_dir, exist_ok=True)

        # 2. PPT → PDF(LibreOffice headless)
        pdf_path = convert_ppt_to_pdf(ppt_path, output_dir)
        if not pdf_path:
            return jsonify({"success": False, "error": "LibreOffice 转换 PDF 失败"}), 500

        # 3. PDF → PNG(pdftoppm)
        images = convert_pdf_to_images(pdf_path, output_dir)

        # 删除中间 PDF
        try:
            os.remove(pdf_path)
        except Exception:
            pass

        if not images:
            return jsonify({"success": False, "error": "PDF 转图片失败"}), 500

        # 4. 读取所有 PNG 文件内容,base64 编码
        result_images = []
        for name in images:
            img_path = os.path.join(output_dir, name)
            with open(img_path, "rb") as f:
                data = base64.b64encode(f.read()).decode("ascii")
            result_images.append({"name": name, "data": data})

        return jsonify({"success": True, "images": result_images})

    except Exception as e:
        return jsonify({"success": False, "error": str(e)}), 500

    finally:
        # 5. 清理所有临时文件(PPT、PDF、PNG)
        try:
            if os.path.exists(ppt_path):
                os.remove(ppt_path)
        except Exception:
            pass
        try:
            if os.path.exists(output_dir):
                shutil.rmtree(output_dir, ignore_errors=True)
        except Exception:
            pass


if __name__ == "__main__":
    # 监听 8000 端口,仅 Docker 内网访问
    app.run(host="0.0.0.0", port=8000, threaded=False)
