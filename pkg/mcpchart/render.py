#!/usr/bin/env python3
"""MCP chart renderer — reads JSON-RPC lines from stdin, writes results to stdout."""
import sys
import json
import io
import os
from datetime import datetime, timezone

import matplotlib
matplotlib.use("Agg")
import matplotlib.pyplot as plt
from matplotlib.font_manager import FontProperties

# 直接查找系统中的中文字体
_font_path = None
for _fp in ["C:/Windows/Fonts/simhei.ttf", "C:/Windows/Fonts/msyh.ttf",
            "/usr/share/fonts/truetype/wqy/wqy-zenhei.ttc",
            "/usr/share/fonts/opentype/noto/NotoSansCJK-Regular.ttc"]:
    if os.path.exists(_fp):
        _font_path = _fp
        break

if _font_path:
    _cn_font = FontProperties(fname=_font_path)
else:
    _cn_font = None

plt.rcParams["axes.unicode_minus"] = False

OUTPUT_DIR = os.environ.get("MCP_CHART_OUTPUT", os.path.join(os.path.dirname(__file__), "output"))
os.makedirs(OUTPUT_DIR, exist_ok=True)


def sanitize_text(s):
    """移除非法 Unicode，修复常见编码错误，避免 FT2Font 崩溃。"""
    if not isinstance(s, str):
        return str(s)

    # 尝试修复 Latin-1 双重编码（UTF-8 字节被误解释为 Latin-1 再编码为 UTF-8）
    try:
        repaired = s.encode("latin-1").decode("utf-8")
        # 如果修复后全是可打印字符且包含 CJK，说明修复成功
        if any("一" <= c <= "鿿" for c in repaired):
            s = repaired
    except (UnicodeEncodeError, UnicodeDecodeError):
        pass

    result = []
    for ch in s:
        cp = ord(ch)
        if 0xD800 <= cp <= 0xDFFF:   # 代理对
            continue
        if cp < 0x20 and ch not in "\n\t":  # 控制字符
            continue
        result.append(ch)
    return "".join(result)


def sanitize_labels(labels):
    return [sanitize_text(l) for l in labels]


def sanitize_data(data):
    """替换 None 值为 0，过滤非数值。"""
    clean = []
    for row in data:
        if not isinstance(row, list):
            continue
        clean_row = []
        for v in row:
            if v is None:
                clean_row.append(0)
            elif isinstance(v, (int, float)):
                clean_row.append(v)
            else:
                try:
                    clean_row.append(float(v))
                except (ValueError, TypeError):
                    clean_row.append(0)
        clean.append(clean_row)
    return clean


for line in sys.stdin:
    line = line.strip()
    if not line:
        continue
    try:
        req = json.loads(line)
    except json.JSONDecodeError:
        continue

    method = req.get("method", "")
    params = req.get("params", {})
    req_id = req.get("id", "")

    if method == "render_chart":
        chart_type = params.get("type", "bar") or "bar"
        title = sanitize_text(params.get("title", "") or "")
        data = sanitize_data(params.get("data") or [[]])
        labels = sanitize_labels(params.get("labels") or [])
        xlabel = sanitize_text(params.get("xlabel", "") or "")
        ylabel = sanitize_text(params.get("ylabel", "") or "")

        try:
            fig, ax = plt.subplots(figsize=(10, 5))
            if chart_type == "bar":
                ax.bar(labels, data[0] if data else [])
            elif chart_type == "line":
                ax.plot(labels, data[0] if data else [], marker="o")
            elif chart_type == "pie":
                ax.pie(data[0] if data else [], labels=labels, autopct="%1.1f%%")
            elif chart_type == "table":
                ax.axis("off")
                tbl = ax.table(cellText=data, colLabels=labels, loc="center")
                tbl.auto_set_font_size(False)
                tbl.set_fontsize(10)

            ax.set_title(title, fontproperties=_cn_font)
            if xlabel:
                ax.set_xlabel(xlabel, fontproperties=_cn_font)
            if ylabel:
                ax.set_ylabel(ylabel, fontproperties=_cn_font)
            if chart_type in ("bar", "line"):
                for lbl in ax.get_xticklabels():
                    lbl.set_fontproperties(_cn_font)
                for lbl in ax.get_yticklabels():
                    lbl.set_fontproperties(_cn_font)
            elif chart_type == "pie":
                for txt in ax.texts:
                    txt.set_fontproperties(_cn_font)
            elif chart_type == "table":
                for (r, c), cell in tbl.get_celld().items():
                    cell.get_text().set_fontproperties(_cn_font)

            buf = io.BytesIO()
            fig.savefig(buf, format="png", dpi=100, bbox_inches="tight")
            plt.close(fig)
            buf.seek(0)

            ts = datetime.now(timezone.utc).strftime("%Y%m%d_%H%M%S_%f")
            fname = f"chart_{ts}.png"
            fpath = os.path.join(OUTPUT_DIR, fname)
            with open(fpath, "wb") as f:
                f.write(buf.read())

            result = {
                "file_name": fname,
                "file_path": fpath,
                "size": os.path.getsize(fpath),
            }
            print(json.dumps({"id": req_id, "result": result}), flush=True)
        except Exception as e:
            print(json.dumps({"id": req_id, "error": f"render failed: {e}"}), flush=True)
    else:
        print(json.dumps({"id": req_id, "error": f"unknown method: {method}"}), flush=True)
