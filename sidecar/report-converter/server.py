import json, os, re, subprocess, tempfile, uuid
from fastapi import FastAPI, HTTPException
from pydantic import BaseModel
import markdown as md_lib
import weasyprint
from docx import Document
from docx.shared import Inches

app = FastAPI()

class ConvertRequest(BaseModel):
    markdown: str
    title: str = ""

def render_mermaid(markdown_text: str, output_dir: str) -> str:
    pattern = r"```mermaid\n([\s\S]*?)```"
    def replacer(m):
        code = m.group(1).strip()
        mmd_file = os.path.join(output_dir, f"chart_{uuid.uuid4().hex}.mmd")
        png_file = mmd_file.replace(".mmd", ".png")
        with open(mmd_file, "w", encoding="utf-8") as f:
            f.write(code)
        subprocess.run(["mmdc", "-i", mmd_file, "-o", png_file, "-b", "transparent"], capture_output=True, timeout=30)
        if os.path.exists(png_file):
            return f"![chart]({png_file})"
        return ""
    return re.sub(pattern, replacer, markdown_text)

@app.post("/convert/pdf")
async def convert_pdf(req: ConvertRequest):
    try:
        workdir = tempfile.mkdtemp()
        md_with_images = render_mermaid(req.markdown, workdir)
        html = md_lib.markdown(md_with_images)
        if req.title:
            html = f"<h1>{req.title}</h1>\n" + html
        output = os.path.join(workdir, f"report_{uuid.uuid4().hex}.pdf")
        weasyprint.HTML(string=html).write_pdf(output)
        return {"file_path": output}
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))

@app.post("/convert/docx")
async def convert_docx(req: ConvertRequest):
    try:
        workdir = tempfile.mkdtemp()
        md_with_images = render_mermaid(req.markdown, workdir)
        doc = Document()
        if req.title:
            doc.add_heading(req.title, 0)
        for line in req.markdown.split("\n"):
            if line.startswith("# "):
                doc.add_heading(line[2:], 1)
            elif line.startswith("## "):
                doc.add_heading(line[3:], 2)
            elif line.startswith("### "):
                doc.add_heading(line[4:], 3)
            elif line.startswith("|"):
                doc.add_paragraph(line)
            elif "```" in line:
                continue
            elif line.strip():
                doc.add_paragraph(line)
        for img_tag in re.finditer(r'!\[.*?\]\((.+?)\)', md_with_images):
            img_path = img_tag.group(1)
            if os.path.exists(img_path):
                doc.add_picture(img_path, width=Inches(5.5))
        output = os.path.join(workdir, f"report_{uuid.uuid4().hex}.docx")
        doc.save(output)
        return {"file_path": output}
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))

if __name__ == "__main__":
    import uvicorn
    uvicorn.run(app, host="127.0.0.1", port=9800)
