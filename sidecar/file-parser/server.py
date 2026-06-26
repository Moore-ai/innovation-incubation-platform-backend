import sys
import os
from io import BytesIO
from fastapi import FastAPI, UploadFile, File, Form
from markitdown import MarkItDown
import uvicorn

app = FastAPI()
converter = MarkItDown()


@app.post("/convert")
async def convert(file: UploadFile = File(...), ext: str = Form("")):
    content = await file.read()
    try:
        result = converter.convert(BytesIO(content), extension=ext)
        return {"markdown": result.text_content}
    except Exception as e:
        return {"error": str(e)}


if __name__ == "__main__":
    sock_path = sys.argv[1]
    os.makedirs(os.path.dirname(sock_path), exist_ok=True)
    if os.path.exists(sock_path):
        os.remove(sock_path)

    config = uvicorn.Config(app, uds=sock_path, log_level="info")
    server = uvicorn.Server(config)
    server.run()
