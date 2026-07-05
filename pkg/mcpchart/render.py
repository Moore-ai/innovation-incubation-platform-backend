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

plt.rcParams["font.sans-serif"] = ["SimHei", "Microsoft YaHei", "DejaVu Sans"]
plt.rcParams["axes.unicode_minus"] = False

OUTPUT_DIR = os.environ.get("MCP_CHART_OUTPUT", os.path.join(os.getcwd(), "internal", "storage", "charts"))
os.makedirs(OUTPUT_DIR, exist_ok=True)

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
        chart_type = params.get("type", "bar")
        title = params.get("title", "")
        data = params.get("data", [[]])
        labels = params.get("labels", [])
        xlabel = params.get("xlabel", "")
        ylabel = params.get("ylabel", "")

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

        ax.set_title(title)
        if xlabel:
            ax.set_xlabel(xlabel)
        if ylabel:
            ax.set_ylabel(ylabel)

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
    else:
        print(json.dumps({"id": req_id, "error": f"unknown method: {method}"}), flush=True)
