"""
LibreOffice Sidecar HTTP 服务
提供两类 HTTP API（均通过 multipart 收发，不依赖共享 volume）：
  1. PPT → PDF → WebP 转换（/convert）
  2. 任意图片 → WebP 压缩（/convert-image）

API:
  POST /convert
    multipart/form-data:
      file: PPT 文件二进制内容 (.ppt/.pptx)
      chapter_id: 字符串,用于临时文件命名
    response: {"success": true, "images": [{"name": "slide_001.webp", "data": "<base64>"}, ...]}
    或    {"success": false, "error": "错误信息"}
    说明：PPT → PDF → PNG → WebP（Pillow 质量 85），返回 WebP base64 数据

  POST /convert-image
    multipart/form-data:
      file: 图片二进制内容（jpg/png/bmp/webp/gif 等）
    response: {"success": true, "data": "<base64 WebP>"}
    或    {"success": false, "error": "错误信息"}
    说明：将任意常见图片转为 WebP（质量 85）；若转换后体积反而变大则返回原始数据

  GET /health
    response: {"status": "ok"}
"""

import base64
import io
import os
import re
import shutil
import subprocess
import threading
import time
from flask import Flask, request, jsonify
from PIL import Image

app = Flask(__name__)

# 并发锁:LibreOffice 同时多个实例会冲突
LO_LOCK = threading.Lock()

# WebP 编码质量（1-100，越高画质越好体积越大）
WEBP_QUALITY = 85

# 需要转 WebP 的源格式（svg 矢量图、gif 动图不转）
COMPRESSIBLE_FORMATS = {"JPEG", "PNG", "BMP", "WEBP", "TIFF"}


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


def png_to_webp_bytes(png_path: str, quality: int = WEBP_QUALITY) -> bytes | None:
    """读取 PNG 文件并转为 WebP 字节（质量 85）。失败返回 None。"""
    try:
        with Image.open(png_path) as img:
            # 统一转 RGB（WebP 有损模式不支持 RGBA/P 模式直接编码会自动处理，
            # 但显式转换可避免部分边缘 case）
            buf = io.BytesIO()
            img.save(buf, format="WEBP", quality=quality, method=6)
            return buf.getvalue()
    except Exception as e:
        print(f"PNG → WebP 转换失败 {png_path}: {e}")
        return None


def image_bytes_to_webp(content: bytes, quality: int = WEBP_QUALITY) -> tuple[bytes, str] | None:
    """
    将任意常见图片字节转为 WebP。
    返回 (webp_bytes, format_name)，失败返回 None。
    若 WebP 体积 >= 原始体积，则返回 (original_bytes, "ORIGINAL")。
    svg/gif 不转换，返回 (original_bytes, "SKIP")。
    """
    if not content:
        return None
    try:
        img = Image.open(io.BytesIO(content))
        fmt = (img.format or "").upper()
        if fmt not in COMPRESSIBLE_FORMATS:
            # svg/gif 等不转换
            return (content, "SKIP")
        # 统一转 RGB 编码 WebP
        if img.mode not in ("RGB", "RGBA"):
            img = img.convert("RGB")
        buf = io.BytesIO()
        img.save(buf, format="WEBP", quality=quality, method=6)
        webp_data = buf.getvalue()
        # 若压缩后反而变大，回退原数据
        if len(webp_data) >= len(content):
            return (content, "ORIGINAL")
        return (webp_data, "WEBP")
    except Exception as e:
        print(f"image → WebP 转换失败: {e}")
        return None


@app.route("/health", methods=["GET"])
def health():
    return jsonify({"status": "ok"})


@app.route("/convert", methods=["POST"])
def convert():
    """PPT → PDF → PNG → WebP 转换"""
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

        # 4. PNG → WebP，base64 编码后返回
        result_images = []
        for name in images:
            img_path = os.path.join(output_dir, name)
            webp_data = png_to_webp_bytes(img_path)
            if webp_data is None:
                # WebP 转换失败，回退原始 PNG
                with open(img_path, "rb") as f:
                    webp_data = f.read()
                out_name = name
            else:
                # slide_001.png → slide_001.webp
                out_name = re.sub(r"\.png$", ".webp", name, flags=re.IGNORECASE)
            result_images.append({
                "name": out_name,
                "data": base64.b64encode(webp_data).decode("ascii"),
            })

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


@app.route("/convert-image", methods=["POST"])
def convert_image():
    """任意图片 → WebP 转换

    用于用户上传的图片（封面、题干图、报告附图等）。
    svg/gif 不转换；其他格式转 WebP（质量 85），若体积反而变大则返回原始数据。
    """
    if "file" not in request.files:
        return jsonify({"success": False, "error": "缺少 file 字段"}), 400
    file = request.files["file"]
    if not file or not file.filename:
        return jsonify({"success": False, "error": "file 不能为空"}), 400

    content = file.read()
    if not content:
        return jsonify({"success": False, "error": "文件内容为空"}), 400

    result = image_bytes_to_webp(content)
    if result is None:
        return jsonify({"success": False, "error": "图片格式不支持或解码失败"}), 400

    webp_data, status = result
    # status: "WEBP"=已转 WebP / "ORIGINAL"=压缩后变大回退原数据 / "SKIP"=svg/gif 不转换
    return jsonify({
        "success": True,
        "status": status,
        "data": base64.b64encode(webp_data).decode("ascii"),
    })


if __name__ == "__main__":
    # 监听 8000 端口,仅 Docker 内网访问
    app.run(host="0.0.0.0", port=8000, threaded=False)
